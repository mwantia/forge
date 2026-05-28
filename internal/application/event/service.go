package event

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	approot "github.com/mwantia/forge/internal/application"
	appsession "github.com/mwantia/forge/internal/application/session"
	dompipeline "github.com/mwantia/forge/internal/domain/pipeline"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
)

type EventService struct {
	approot.UnimplementedService

	logger  hclog.Logger           `fabric:"logger=event"`
	router  infraserver.HttpRouter `fabric:"inject"`
	configs []*EventConfig         `fabric:"config=event"`

	pipeline  dompipeline.BackgroundDispatcher `fabric:"inject"`
	sessions  appsession.SessionManager        `fabric:"inject"`
	templates infratemplate.TemplateRenderer   `fabric:"inject"`

	mu     sync.RWMutex
	states map[string]*EventWindowState
}

type EventWindowState struct {
	mu         sync.Mutex
	queue      []EventInfo
	timer      *time.Timer
	expires    time.Time
	branchBase string
	paused     bool
	lastErr    error
}

type EventInfo struct {
	payload []byte
	firedAt time.Time
}

func init() {
	container.MustRegister[*EventService](container.AsSingleton())
}

func (*EventService) PreInit(context.Context) error {
	return nil
}

func (s *EventService) PostInit(context.Context) error {
	s.mu.Lock()
	s.states = make(map[string]*EventWindowState, len(s.configs))
	for _, cfg := range s.configs {
		s.states[cfg.ID] = &EventWindowState{}
	}
	s.mu.Unlock()

	g := s.router.AuthGroup("/events")
	g.GET("", s.handleList())
	g.GET("/:id", s.handleGet())
	g.GET("/:id/push", s.handlePush())
	g.POST("/:id/push", s.handlePush())
	g.POST("/:id/pause", s.handlePause())
	g.POST("/:id/resume", s.handleResume())

	return nil
}

func (s *EventService) state(id string) *EventWindowState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.states[id]
}

func (s *EventService) findConfig(id string) *EventConfig {
	for _, cfg := range s.configs {
		if cfg.ID == id {
			return cfg
		}
	}
	return nil
}

// push implements the push semantics for event endpoints.
// When async=false the immediate-dispatch path blocks until the pipeline
// finishes and returns the assistant's response in PushResponse.Content.
// Queued and window-open paths always return immediately (async behaviour).
func (s *EventService) push(cfg *EventConfig, payload []byte, baseRef string, async bool) (PushResponse, int) {
	now := time.Now()

	if ws := s.state(cfg.ID); func() bool {
		ws.mu.Lock()
		defer ws.mu.Unlock()
		return ws.paused
	}() {
		return PushResponse{EventID: cfg.ID, Status: "paused", PushedAt: now}, http.StatusServiceUnavailable
	}

	if cfg.Options == nil || cfg.Options.Timespan == "" {
		if !async {
			branch, content, err := s.dispatchForeground(cfg, []EventInfo{{payload, now}}, baseRef)
			ws := s.state(cfg.ID)
			ws.mu.Lock()
			ws.lastErr = err
			ws.mu.Unlock()
			if err != nil {
				s.logger.Error("sync dispatch failed", "event", cfg.ID, "error", err)
				return PushResponse{EventID: cfg.ID, Status: "error", PushedAt: now}, http.StatusInternalServerError
			}
			return PushResponse{EventID: cfg.ID, Status: "dispatched", PushedAt: now, Branch: branch, Content: content}, http.StatusOK
		}

		branch, err := s.dispatchNow(cfg, []EventInfo{{payload, now}}, baseRef)
		if err != nil {
			s.logger.Error("immediate dispatch failed", "event", cfg.ID, "error", err)
			ws := s.state(cfg.ID)
			ws.mu.Lock()
			ws.lastErr = err
			ws.mu.Unlock()
			return PushResponse{EventID: cfg.ID, Status: "error", PushedAt: now}, http.StatusInternalServerError
		}
		ws := s.state(cfg.ID)
		ws.mu.Lock()
		ws.lastErr = nil
		ws.mu.Unlock()
		return PushResponse{EventID: cfg.ID, Status: "dispatched", PushedAt: now, Branch: branch}, http.StatusOK
	}

	dur, err := time.ParseDuration(cfg.Options.Timespan)
	if err != nil {
		s.logger.Error("invalid timespan", "event", cfg.ID, "timespan", cfg.Options.Timespan, "error", err)
		return PushResponse{EventID: cfg.ID, Status: "error", PushedAt: now}, http.StatusInternalServerError
	}

	ws := s.state(cfg.ID)
	ws.mu.Lock()

	if ws.timer == nil {
		// Window idle → dispatch immediately and open window.
		ws.branchBase = baseRef
		ws.expires = now.Add(dur)
		logger := s.logger.With("event", cfg.ID)
		ws.timer = time.AfterFunc(dur, func() { s.onWindowExpiry(cfg, logger) })
		ws.mu.Unlock()

		branch, dispErr := s.dispatchNow(cfg, []EventInfo{{payload, now}}, baseRef)

		ws.mu.Lock()
		if dispErr != nil {
			ws.timer.Stop()
			ws.timer = nil
			ws.expires = time.Time{}
			ws.lastErr = dispErr
			ws.mu.Unlock()
			s.logger.Error("first-push dispatch failed", "event", cfg.ID, "error", dispErr)
			return PushResponse{EventID: cfg.ID, Status: "error", PushedAt: now}, http.StatusInternalServerError
		}
		ws.lastErr = nil
		exp := ws.expires
		ws.mu.Unlock()
		return PushResponse{
			EventID:         cfg.ID,
			Status:          "dispatched",
			PushedAt:        now,
			Branch:          branch,
			WindowExpiresAt: &exp,
		}, http.StatusOK
	}

	// Window is open.
	exp := ws.expires
	if cfg.Options.MaxQueue == 0 {
		ws.mu.Unlock()
		return PushResponse{
			EventID:         cfg.ID,
			Status:          "window_open",
			PushedAt:        now,
			WindowExpiresAt: &exp,
		}, http.StatusOK
	}

	// Enqueue with ring-buffer eviction.
	evicted := false
	if len(ws.queue) >= cfg.Options.MaxQueue {
		ws.queue = ws.queue[1:]
		evicted = true
	}
	ws.queue = append(ws.queue, EventInfo{payload, now})
	queueSize := len(ws.queue)
	ws.mu.Unlock()

	return PushResponse{
		EventID:         cfg.ID,
		Status:          "queued",
		PushedAt:        now,
		QueueSize:       queueSize,
		QueueCapacity:   cfg.Options.MaxQueue,
		Evicted:         evicted,
		WindowExpiresAt: &exp,
	}, http.StatusAccepted
}

func (s *EventService) buildStatus(ctx context.Context, cfg *EventConfig) EventStatus {
	ws := s.state(cfg.ID)
	ws.mu.Lock()
	size := len(ws.queue)
	expires := ws.expires
	paused := ws.paused
	lastErr := ws.lastErr
	ws.mu.Unlock()

	var opts *queueOpts
	if cfg.Options != nil {
		opts = &queueOpts{
			Timespan: cfg.Options.Timespan,
			MaxQueue: cfg.Options.MaxQueue,
		}
	}

	var expiresPtr *time.Time
	if !expires.IsZero() {
		expiresPtr = &expires
	}

	state := EventStateRunning
	switch {
	case lastErr != nil:
		state = EventStateFailed
	case paused:
		state = EventStatePaused
	}

	status := EventStatus{
		ID:          cfg.ID,
		Description: cfg.Description,
		Session:     cfg.Session,
		State:       state,
		Options:     opts,
		Queue:       &queueState{Size: size, WindowExpiresAt: expiresPtr},
	}

	if meta, err := s.sessions.ResolveSession(ctx, cfg.Session); err == nil {
		if refs, err := s.sessions.ListRefs(ctx, meta.ID); err == nil {
			prefix := "event/" + cfg.ID + "-"
			var latest time.Time
			for name := range refs {
				if !strings.HasPrefix(name, prefix) {
					continue
				}
				ts := strings.TrimPrefix(name, prefix)
				t, err := time.Parse(time.RFC3339, ts)
				if err != nil {
					continue
				}
				if t.After(latest) {
					latest = t
					status.LastBranch = name
				}
			}
		}
	}

	return status
}

func (s *EventService) handleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		out := make([]EventStatus, 0, len(s.configs))
		for _, cfg := range s.configs {
			out = append(out, s.buildStatus(c.Request.Context(), cfg))
		}
		c.JSON(http.StatusOK, out)
	}
}

func (s *EventService) handleGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := s.findConfig(c.Param("id"))
		if cfg == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown event"})
			return
		}
		c.JSON(http.StatusOK, s.buildStatus(c.Request.Context(), cfg))
	}
}

func (s *EventService) handlePush() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := s.findConfig(c.Param("id"))
		if cfg == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown event"})
			return
		}

		baseRef := c.Query("ref")
		async := c.Query("async") == "true"
		var payload []byte

		if c.Request.Method == http.MethodGet {
			payload = []byte(c.Query("payload"))
		} else {
			body, err := c.GetRawData()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
				return
			}
			payload = body
		}

		resp, status := s.push(cfg, payload, baseRef, async)
		c.JSON(status, resp)
	}
}

func (s *EventService) handlePause() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := s.findConfig(c.Param("id"))
		if cfg == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown event"})
			return
		}
		ws := s.state(cfg.ID)
		ws.mu.Lock()
		ws.paused = true
		ws.mu.Unlock()
		c.JSON(http.StatusOK, s.buildStatus(c.Request.Context(), cfg))
	}
}

func (s *EventService) handleResume() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := s.findConfig(c.Param("id"))
		if cfg == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown event"})
			return
		}
		ws := s.state(cfg.ID)
		ws.mu.Lock()
		ws.paused = false
		ws.mu.Unlock()
		c.JSON(http.StatusOK, s.buildStatus(c.Request.Context(), cfg))
	}
}

var _ approot.Service = (*EventService)(nil)
