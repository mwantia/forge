package event

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service"
	"github.com/mwantia/forge/internal/service/pipeline"
	"github.com/mwantia/forge/internal/service/server"
	"github.com/mwantia/forge/internal/service/session"
	svctemplate "github.com/mwantia/forge/internal/service/template"
)

type EventService struct {
	service.UnimplementedService

	logger  hclog.Logger      `fabric:"logger:event"`
	router  server.HttpRouter `fabric:"inject"`
	configs []*EventConfig    `fabric:"config:event"`

	pipeline  pipeline.BackgroundDispatcher `fabric:"inject"`
	sessions  session.SessionManager        `fabric:"inject"`
	templates svctemplate.TemplateRenderer  `fabric:"inject"`

	mu     sync.RWMutex
	states map[string]*EventWindowState
}

type EventWindowState struct {
	mu         sync.Mutex
	queue      []EventInfo
	timer      *time.Timer
	expires    time.Time
	branchBase string
}

type EventInfo struct {
	payload []byte
	firedAt time.Time
}

func init() {
	if err := container.Register[*EventService](container.AsSingleton()); err != nil {
		panic(err)
	}
}

func (s *EventService) Init(_ context.Context) error {
	s.mu.Lock()
	s.states = make(map[string]*EventWindowState, len(s.configs))
	for _, cfg := range s.configs {
		s.states[cfg.ID] = &EventWindowState{}
	}
	s.mu.Unlock()

	g := s.router.AuthGroup("/events")
	g.GET("", s.handleList())
	g.GET("/:id", s.handleGet())
	g.GET("/:id/fire", s.handleFire())
	g.POST("/:id/fire", s.handleFire())

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

// fire implements the fire semantics from §1.3 of the proposal.
func (s *EventService) fire(cfg *EventConfig, payload []byte, baseRef string) (FireResponse, int) {
	now := time.Now()

	if cfg.Options == nil || cfg.Options.Timespan == "" {
		branch, err := s.dispatchNow(cfg, []EventInfo{{payload, now}}, baseRef)
		if err != nil {
			s.logger.Error("immediate dispatch failed", "event", cfg.ID, "error", err)
			return FireResponse{EventID: cfg.ID, Status: "error", FiredAt: now}, http.StatusInternalServerError
		}
		return FireResponse{EventID: cfg.ID, Status: "dispatched", FiredAt: now, Branch: branch}, http.StatusOK
	}

	dur, err := time.ParseDuration(cfg.Options.Timespan)
	if err != nil {
		s.logger.Error("invalid timespan", "event", cfg.ID, "timespan", cfg.Options.Timespan, "error", err)
		return FireResponse{EventID: cfg.ID, Status: "error", FiredAt: now}, http.StatusInternalServerError
	}

	ws := s.state(cfg.ID)
	ws.mu.Lock()
	defer ws.mu.Unlock()

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
			s.logger.Error("first-fire dispatch failed", "event", cfg.ID, "error", dispErr)
			return FireResponse{EventID: cfg.ID, Status: "error", FiredAt: now}, http.StatusInternalServerError
		}
		exp := ws.expires
		return FireResponse{
			EventID:         cfg.ID,
			Status:          "dispatched",
			FiredAt:         now,
			Branch:          branch,
			WindowExpiresAt: &exp,
		}, http.StatusOK
	}

	// Window is open.
	exp := ws.expires
	if cfg.Options.MaxQueue == 0 {
		return FireResponse{
			EventID:         cfg.ID,
			Status:          "window_open",
			FiredAt:         now,
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

	return FireResponse{
		EventID:         cfg.ID,
		Status:          "queued",
		FiredAt:         now,
		QueueSize:       len(ws.queue),
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

	status := EventStatus{
		ID:          cfg.ID,
		Description: cfg.Description,
		Session:     cfg.Session,
		Options:     opts,
		Queue:       &queueState{Size: size, WindowExpiresAt: expiresPtr},
	}

	if meta, err := s.sessions.ResolveSession(ctx, cfg.Session); err == nil {
		if refs, err := s.sessions.ListRefs(ctx, meta.ID); err == nil {
			prefix := "event/" + cfg.ID + "-"
			var branches []string
			for name := range refs {
				if strings.HasPrefix(name, prefix) {
					branches = append(branches, name)
				}
			}
			if len(branches) > 0 {
				sort.Strings(branches)
				status.LastBranch = branches[len(branches)-1]
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

func (s *EventService) handleFire() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := s.findConfig(c.Param("id"))
		if cfg == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown event"})
			return
		}

		baseRef := c.Query("ref")
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

		resp, status := s.fire(cfg, payload, baseRef)
		c.JSON(status, resp)
	}
}

var _ service.Service = (*EventService)(nil)
