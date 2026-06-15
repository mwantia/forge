package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	domsession "github.com/mwantia/forge/internal/domain/session"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
)

type SessionManager = domsession.SessionManager

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
func (s *SessionService) AppendMessage(ctx context.Context, sessionID string, msg *Message) (string, error) {
	s.mu.Lock()
	if err := s.ensureNotArchived(ctx, sessionID); err != nil {
		s.mu.Unlock()
		return "", err
	}

	capturedUsage := msg.Usage
	hash, err := s.store.SaveMessage(ctx, sessionID, dag.HEAD, msg)

	s.mu.Unlock()
	if err != nil {
		return "", err
	}

	if capturedUsage != nil {
		// Fire ans forget call to asynchronously write usage
		go s.accumulateUsage(ctx, sessionID, capturedUsage)
	}

	return hash, nil
}

// AppendMessageToRef implements SessionManager.
func (s *SessionService) AppendMessageToRef(ctx context.Context, sessionID, ref string, msg *Message) (string, error) {
	s.mu.Lock()
	if err := s.ensureNotArchived(ctx, sessionID); err != nil {
		s.mu.Unlock()
		return "", err
	}

	capturedUsage := msg.Usage
	hash, err := s.store.SaveMessage(ctx, sessionID, ref, msg)

	s.mu.Unlock()
	if err != nil {
		return "", err
	}

	if capturedUsage != nil {
		// Fire ans forget call to asynchronously write usage
		go s.accumulateUsage(ctx, sessionID, capturedUsage)
	}

	return hash, nil
}

// accumulateUsage folds a per-message TokenUsage into the session's running
// total. Failures are logged and swallowed: usage accounting must not fail a
// successful message commit.
func (s *SessionService) accumulateUsage(ctx context.Context, sessionID string, usage *sdkplugins.TokenUsage) {
	if usage == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.store.LoadSession(ctx, sessionID)
	if err != nil {
		s.logger.Warn("usage accumulate: load session failed", "session", sessionID, "error", err)
		return
	}

	if meta.Usage == nil {
		meta.Usage = &sdkplugins.TokenUsage{}
	}

	meta.Usage.Add(usage)
	meta.UpdatedAt = time.Now()

	if err := s.store.SaveSession(ctx, meta); err != nil {
		s.logger.Warn("usage accumulate: save session failed", "session", sessionID, "error", err)
	}
}

// AccumulateDuration implements SessionManager.
func (s *SessionService) AccumulateDuration(ctx context.Context, sessionID string, ms int64) {
	if ms <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.store.LoadSession(ctx, sessionID)
	if err != nil {
		s.logger.Warn("duration accumulate: load session failed", "session", sessionID, "error", err)
		return
	}

	meta.TotalDurationMs += ms
	meta.UpdatedAt = time.Now()

	if err := s.store.SaveSession(ctx, meta); err != nil {
		s.logger.Warn("duration accumulate: save session failed", "session", sessionID, "error", err)
	}
}

// HeadHash implements SessionManager.
func (s *SessionService) HeadHash(ctx context.Context, sessionID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.HeadHash(ctx, sessionID)
}

// PutPromptContext implements SessionManager.
func (s *SessionService) PutPromptContext(ctx context.Context, p *dag.PromptContext) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.objects.PutPromptContext(ctx, p)
}

// PutToolCatalog implements SessionManager.
func (s *SessionService) PutToolCatalog(ctx context.Context, t *dag.ToolCatalog) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.objects.PutToolCatalog(ctx, t)
}

// GetPromptContext implements SessionManager.
func (s *SessionService) GetPromptContext(ctx context.Context, hash string) (*dag.PromptContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.objects.GetPromptContext(ctx, hash)
}

// GetMessageObj implements SessionManager.
func (s *SessionService) GetMessageObj(ctx context.Context, hash string) (*dag.MessageObj, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.objects.GetMessage(ctx, hash)
}

// ListMessagesFromRef implements SessionManager.
func (s *SessionService) ListMessagesFromRef(ctx context.Context, sessionID, ref string, offset, limit int) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.ListMessagesFromRef(ctx, sessionID, ref, offset, limit)
}

// ListRefs implements SessionManager.
func (s *SessionService) ListRefs(ctx context.Context, sessionID string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.refs.List(ctx, sessionID)
}

// ReadRef implements SessionManager.
func (s *SessionService) ReadRef(ctx context.Context, sessionID, name string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hash, err := s.store.refs.Read(ctx, sessionID, name)
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return "", nil
		}
		return "", err
	}

	return hash, nil
}

// WriteRef implements SessionManager.
func (s *SessionService) WriteRef(ctx context.Context, sessionID, name, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureNotArchived(ctx, sessionID); err != nil {
		return err
	}

	return s.store.refs.Write(ctx, sessionID, name, hash)
}

// CASRef implements SessionManager.
func (s *SessionService) CASRef(ctx context.Context, sessionID, name, expected, next string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureNotArchived(ctx, sessionID); err != nil {
		return err
	}

	return s.store.refs.CAS(ctx, sessionID, name, expected, next)
}

// DeleteRef implements SessionManager.
func (s *SessionService) DeleteRef(ctx context.Context, sessionID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureNotArchived(ctx, sessionID); err != nil {
		return err
	}

	return s.store.refs.Delete(ctx, sessionID, name)
}

// RenameRef implements SessionManager.
func (s *SessionService) RenameRef(ctx context.Context, sessionID, oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureNotArchived(ctx, sessionID); err != nil {
		return err
	}

	return s.store.refs.Rename(ctx, sessionID, oldName, newName)
}

// PutMessageObj implements SessionManager.
func (s *SessionService) PutMessageObj(ctx context.Context, obj *dag.MessageObj) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.objects.PutMessage(ctx, obj)
}

// ResolveMessageHash implements SessionManager.
func (s *SessionService) ResolveMessageHash(ctx context.Context, sessionID, hashOrPrefix string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.ResolvePrefix(ctx, sessionID, hashOrPrefix)
}

// CheckoutRef implements SessionManager.
func (s *SessionService) CheckoutRef(ctx context.Context, sessionID, targetBranch string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureNotArchived(ctx, sessionID); err != nil {
		return err
	}

	if _, err := s.store.refs.Read(ctx, sessionID, targetBranch); err != nil {
		return fmt.Errorf("ref %q not found: %w", targetBranch, err)
	}

	return s.store.refs.WriteSymRef(ctx, sessionID, dag.HEAD, targetBranch)
}

// QuerySessions returns sessions matching q, sorted by UpdatedAt descending.
func (s *SessionService) QuerySessions(ctx context.Context, q SessionQuery) ([]*SessionMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.QuerySessions(ctx, q)
}

// ListParentSessions returns top-level sessions (parentID="") or children of
// a given parent, optionally including archived sessions.
func (s *SessionService) ListParentSessions(ctx context.Context, parentID string, archived bool, offset, limit int) ([]*SessionMetadata, error) {
	return s.QuerySessions(ctx, SessionQuery{ParentID: parentID, Archived: &archived, Offset: offset, Limit: limit})
}

// CreateSession creates a new session and initialises its HEAD ref.
func (s *SessionService) CreateSession(ctx context.Context, model, name, title, description, parent string, plugins []PluginConfig) (*SessionMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name == "" {
		name = infratemplate.GenerateUniqueName()
	}
	if existing, err := s.store.FindSessionByName(ctx, name); err == nil && existing != nil {
		return nil, fmt.Errorf("session name already exists: %s", name)
	}
	now := time.Now()
	meta := &SessionMetadata{
		ID:          DeriveSessionID(name, now.UnixNano(), parent),
		Name:        name,
		Title:       title,
		Description: description,
		Parent:      parent,
		Model:       model,
		Plugins:     plugins,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.SaveSession(ctx, meta); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}
	if err := s.store.refs.WriteSymRef(ctx, meta.ID, dag.HEAD, dag.MAIN); err != nil {
		s.logger.Warn("create session: HEAD symref init failed", "session", meta.ID, "error", err)
	}
	return meta, nil
}

// SaveSession implements SessionManager.
func (s *SessionService) SaveSession(ctx context.Context, meta *SessionMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.SaveSession(ctx, meta)
}

// DeleteSession removes a session and all its associated data.
func (s *SessionService) DeleteSession(ctx context.Context, idOrName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, err := s.resolveSessionLocked(ctx, idOrName)
	if err != nil {
		return err
	}
	return s.store.DeleteSession(ctx, meta.ID)
}

var _ SessionManager = (*SessionService)(nil)
