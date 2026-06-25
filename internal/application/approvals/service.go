package approvals

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugin"
	approot "github.com/mwantia/forge/internal/application"
	domapprovals "github.com/mwantia/forge/internal/domain/approvals"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
)

const defaultTTL = 5 * time.Minute

type pendingEntry struct {
	rec      *domapprovals.ApprovalRecord
	decision chan sdkplugins.ApprovalDecision
}

type ApprovalService struct {
	approot.UnimplementedService

	router infraserver.HttpRouter `fabric:"inject"`
	logger hclog.Logger           `fabric:"logger=approvals"`
	config ApprovalsConfig        `fabric:"config=approvals"`

	mu      sync.RWMutex
	records map[string]*pendingEntry
	subs    []chan *domapprovals.ApprovalRecord
}

func init() {
	container.MustRegister[*ApprovalService](
		container.AsSingleton(),
		container.With[domapprovals.ApprovalRegistar](),
	)
}

func (*ApprovalService) PreInit(context.Context) error {
	return nil
}

func (s *ApprovalService) PostInit(_ context.Context) error {
	s.records = make(map[string]*pendingEntry)

	group := s.router.AuthGroup("/approvals")
	{
		group.GET("", s.handleList())
		group.GET("/stream", s.handleStream())
		group.GET("/:id", s.handleGet())
		group.POST("/:id/respond", s.handleRespond())
		group.DELETE("/:id", s.handleCancel())
	}

	return nil
}

func (s *ApprovalService) Cleanup(context.Context) error {
	return nil
}

func (s *ApprovalService) Create(ctx context.Context, req sdkplugins.ApprovalRequest, meta domapprovals.ApprovalMeta) (*domapprovals.ApprovalRecord, error) {
	id, err := newID()
	if err != nil {
		return nil, fmt.Errorf("generate approval id: %w", err)
	}

	ttl := defaultTTL
	if s.config.TTL != "" {
		if d, parseErr := time.ParseDuration(s.config.TTL); parseErr == nil {
			ttl = d
		}
	}

	now := time.Now()
	expiresAt := now.Add(ttl)

	rec := &domapprovals.ApprovalRecord{
		ID:        id,
		Type:      string(req.Type),
		Title:     req.Title,
		Message:   req.Message,
		Status:    domapprovals.StatusPending,
		CreatedAt: now,
		ExpiresAt: &expiresAt,
		Plugin:    meta.Plugin,
		ToolName:  meta.ToolName,
		ToolArgs:  meta.ToolArgs,
	}

	decisionCh := make(chan sdkplugins.ApprovalDecision, 1)
	entry := &pendingEntry{
		rec:      rec,
		decision: decisionCh,
	}

	s.mu.Lock()
	s.records[id] = entry
	s.mu.Unlock()

	s.broadcast(rec)

	// TTL goroutine.
	go func() {
		<-time.After(ttl)
		s.mu.Lock()
		e, ok := s.records[id]

		if ok && e.rec.Status == domapprovals.StatusPending {
			e.rec.Status = domapprovals.StatusTimeout
			e.rec.Reason = "timeout"
			s.mu.Unlock()

			select {
			case decisionCh <- sdkplugins.ApprovalDecision{Allow: false, Reason: "timeout"}:
			default:
			}

			s.broadcast(rec)
		} else {
			s.mu.Unlock()
		}
	}()

	// Block until decision or ctx cancellation.
	select {
	case d := <-decisionCh:
		s.mu.Lock()

		if rec.Status == domapprovals.StatusPending {
			if d.Allow {
				rec.Status = domapprovals.StatusAllowed
			} else {
				rec.Status = domapprovals.StatusDenied
			}
			rec.Reason = d.Reason
		}

		// keep entry for inspection after resolution
		s.mu.Unlock()
		s.broadcast(rec)

		return rec, nil

	case <-ctx.Done():
		s.mu.Lock()

		if rec.Status == domapprovals.StatusPending {
			rec.Status = domapprovals.StatusCancelled
		}

		s.mu.Unlock()
		s.broadcast(rec)

		return rec, ctx.Err()
	}
}

func (s *ApprovalService) Respond(id string, decision sdkplugins.ApprovalDecision) error {
	s.mu.Lock()
	entry, ok := s.records[id]

	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("approval %q not found", id)
	}

	if entry.rec.Status != domapprovals.StatusPending {
		s.mu.Unlock()
		return fmt.Errorf("approval %q is not pending (status: %s)", id, entry.rec.Status)
	}
	s.mu.Unlock()

	select {
	case entry.decision <- decision:
	default:
		return fmt.Errorf("approval %q already has a decision pending", id)
	}

	return nil
}

func (s *ApprovalService) Get(id string) (*domapprovals.ApprovalRecord, error) {
	s.mu.RLock()
	entry, ok := s.records[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("approval %q not found", id)
	}

	return entry.rec, nil
}

func (s *ApprovalService) List(filter domapprovals.ApprovalFilter) ([]*domapprovals.ApprovalRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statusSet := map[domapprovals.ApprovalStatus]bool{}
	for _, st := range filter.Status {
		statusSet[st] = true
	}

	var out []*domapprovals.ApprovalRecord
	for _, entry := range s.records {
		rec := entry.rec
		if len(statusSet) > 0 && !statusSet[rec.Status] {
			continue
		}

		if filter.Plugin != "" && rec.Plugin != filter.Plugin {
			continue
		}

		out = append(out, rec)
	}

	return out, nil
}

func (s *ApprovalService) Cancel(id string) error {
	s.mu.Lock()
	entry, ok := s.records[id]

	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("approval %q not found", id)
	}

	entry.rec.Status = domapprovals.StatusCancelled
	s.mu.Unlock()

	s.broadcast(entry.rec)
	return nil
}

func (s *ApprovalService) Subscribe(ctx context.Context) <-chan *domapprovals.ApprovalRecord {
	ch := make(chan *domapprovals.ApprovalRecord, 16)

	s.mu.Lock()
	s.subs = append(s.subs, ch)
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.mu.Lock()

		for i, sub := range s.subs {
			if sub == ch {
				s.subs = append(s.subs[:i], s.subs[i+1:]...)
				break
			}
		}

		s.mu.Unlock()
		close(ch)
	}()

	return ch
}

func (s *ApprovalService) broadcast(rec *domapprovals.ApprovalRecord) {
	s.mu.RLock()
	subs := make([]chan *domapprovals.ApprovalRecord, len(s.subs))
	copy(subs, s.subs)
	s.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub <- rec:
		default:
		}
	}
}

func (s *ApprovalService) CheckDeny(tool string) bool {
	for _, pattern := range s.config.Deny {
		if matchGlob(pattern, tool) {
			return true
		}
	}

	return false
}

func (s *ApprovalService) CheckAllow(tool string) bool {
	if len(s.config.Allow) == 0 {
		return false
	}
	for _, pattern := range s.config.Allow {
		if matchGlob(pattern, tool) {
			return true
		}
	}

	return false
}

func newID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

var _ domapprovals.ApprovalRegistar = (*ApprovalService)(nil)
var _ approot.Service = (*ApprovalService)(nil)
