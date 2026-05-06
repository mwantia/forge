package event

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
)

// dispatchNow creates a new event branch on the target session and runs
// the pipeline asynchronously. Returns the branch name on success.
func (s *EventService) dispatchNow(cfg *EventConfig, entries []EventInfo, baseRef string) (string, error) {
	content, err := assembleUserMessage(s.templates, entries, cfg)
	if err != nil {
		return "", fmt.Errorf("event %q: assemble message: %w", cfg.ID, err)
	}

	ctx := context.Background()

	meta, err := s.sessions.ResolveSession(ctx, cfg.Session)
	if err != nil {
		return "", fmt.Errorf("event %q: resolve session %q: %w", cfg.ID, cfg.Session, err)
	}

	ref := baseRef
	if ref == "" {
		ref = cfg.Branch
	}
	if ref == "" {
		ref = "main"
	}

	headHash, err := s.sessions.ReadRef(ctx, meta.ID, ref)
	if err != nil {
		return "", fmt.Errorf("event %q: read ref %q: %w", cfg.ID, ref, err)
	}

	ts := time.Now().UTC().Format(time.RFC3339)
	branch := fmt.Sprintf("event/%s-%s", cfg.ID, ts)

	if err := s.sessions.WriteRef(ctx, meta.ID, branch, headHash); err != nil {
		return "", fmt.Errorf("event %q: create branch %q: %w", cfg.ID, branch, err)
	}

	sessionID := meta.ID
	logger := s.logger.With("event", cfg.ID, "branch", branch, "session", sessionID)

	go func() {
		if err := s.pipeline.DispatchBackground(ctx, sessionID, branch, content); err != nil {
			logger.Error("background dispatch failed", "error", err)
		}
	}()

	return branch, nil
}

// onWindowExpiry is called by time.AfterFunc when a batch window expires.
// It snapshots the queue, clears window state, and dispatches.
func (s *EventService) onWindowExpiry(cfg *EventConfig, logger hclog.Logger) {
	ws := s.state(cfg.ID)
	ws.mu.Lock()
	queue := ws.queue
	base := ws.branchBase
	ws.queue = nil
	ws.timer = nil
	ws.expires = time.Time{}
	ws.mu.Unlock()

	if len(queue) == 0 {
		return
	}

	if _, err := s.dispatchNow(cfg, queue, base); err != nil {
		logger.Error("batch dispatch failed", "error", err)
	}
}
