package session

import (
	"context"
	"fmt"
)

// SessionManager is the narrow surface other services use to interact with
// session state. It hides locking, storage layout, and tool registration
// from callers (e.g. PipelineService).
type SessionManager interface {
	// ResolveSession loads a session by ID, falling back to a name lookup.
	// Returns a non-nil error when neither resolves.
	ResolveSession(ctx context.Context, idOrName string) (*SessionMetadata, error)

	// LoadSession loads a session by exact ID.
	LoadSession(ctx context.Context, id string) (*SessionMetadata, error)

	// ListMessages returns messages for a session in chronological order.
	// Pass offset=0, limit=0 to get the full history.
	ListMessages(ctx context.Context, sessionID string, offset, limit int) ([]*Message, error)

	// AppendMessage persists a message to the session's history.
	AppendMessage(ctx context.Context, sessionID string, msg *Message) error
}

// ResolveSession implements SessionManager.
func (s *SessionService) ResolveSession(ctx context.Context, idOrName string) (*SessionMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resolveSessionLocked(ctx, idOrName)
}

func (s *SessionService) resolveSessionLocked(ctx context.Context, idOrName string) (*SessionMetadata, error) {
	meta, err := s.store.LoadSession(ctx, idOrName)
	if err == nil {
		return meta, nil
	}
	meta, nerr := s.store.FindSessionByName(ctx, idOrName)
	if nerr != nil || meta == nil {
		return nil, fmt.Errorf("session not found: %s", idOrName)
	}
	return meta, nil
}

// LoadSession implements SessionManager.
func (s *SessionService) LoadSession(ctx context.Context, id string) (*SessionMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.LoadSession(ctx, id)
}

// ListMessages implements SessionManager.
func (s *SessionService) ListMessages(ctx context.Context, sessionID string, offset, limit int) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.ListMessages(ctx, sessionID, offset, limit)
}

// AppendMessage implements SessionManager.
func (s *SessionService) AppendMessage(ctx context.Context, sessionID string, msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.SaveMessage(ctx, sessionID, msg)
}

var _ SessionManager = (*SessionService)(nil)
