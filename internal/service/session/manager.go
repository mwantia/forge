package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session/dag"
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

	// AppendMessage persists a message to the session's active branch.
	// Returns the resulting content-hash and updates msg.Hash/ParentHash/CreatedAt
	// in place.
	AppendMessage(ctx context.Context, sessionID string, msg *Message) (string, error)

	// HeadHash returns the current HEAD hash, or "" if the branch is empty.
	HeadHash(ctx context.Context, sessionID string) (string, error)

	// PutPromptContext stores a PromptContext blob in the global object
	// store and returns its hash. Pipeline calls this before dispatching to
	// a provider so the materialized prompt is reproducible.
	PutPromptContext(ctx context.Context, p *dag.PromptContext) (string, error)

	// PutToolCatalog stores a ToolCatalog snapshot in the global object
	// store and returns its content hash. Called by recordPromptContext so
	// replays use exactly the tool set that was active at dispatch time.
	PutToolCatalog(ctx context.Context, t *dag.ToolCatalog) (string, error)

	// GetPromptContext loads a previously stored PromptContext by hash.
	GetPromptContext(ctx context.Context, hash string) (*dag.PromptContext, error)

	// GetMessageObj loads a raw MessageObj from the global object store
	// without requiring a session ID. Use for materializing a PromptContext
	// whose message hashes may span sessions.
	GetMessageObj(ctx context.Context, hash string) (*dag.MessageObj, error)

	// AppendMessageToRef is AppendMessage but advances a named ref instead
	// of HEAD. Used for branch dispatching.
	AppendMessageToRef(ctx context.Context, sessionID, ref string, msg *Message) (string, error)

	// ListMessagesFromRef walks a non-HEAD branch.
	ListMessagesFromRef(ctx context.Context, sessionID, ref string, offset, limit int) ([]*Message, error)

	// ListRefs returns all refs for a session (name -> hash).
	ListRefs(ctx context.Context, sessionID string) (map[string]string, error)

	// ReadRef returns the hash a ref points at, or "" if missing.
	ReadRef(ctx context.Context, sessionID, name string) (string, error)

	// WriteRef unconditionally points name at hash.
	WriteRef(ctx context.Context, sessionID, name, hash string) error

	// CASRef advances ref name from expected to next, or returns an error
	// describing the conflict. Pass expected="" to assert the ref does not
	// currently exist.
	CASRef(ctx context.Context, sessionID, name, expected, next string) error

	// DeleteRef removes a ref. Missing refs are not an error.
	DeleteRef(ctx context.Context, sessionID, name string) error

	// RenameRef atomically renames a ref. Returns an error if newName already exists.
	RenameRef(ctx context.Context, sessionID, oldName, newName string) error

	// PutMessageObj stores a raw MessageObj in the global object pool and
	// returns its content hash. Used to persist system-prompt snapshots.
	PutMessageObj(ctx context.Context, obj *dag.MessageObj) (string, error)

	// ResolveMessageHash expands a hash prefix (>=4 hex chars) to a full
	// hash within a session's log. Returns the input unchanged when it is
	// already a full 64-hex string.
	ResolveMessageHash(ctx context.Context, sessionID, hashOrPrefix string) (string, error)

	// CheckoutRef sets HEAD to point symbolically at targetBranch, so that
	// subsequent dispatches advance targetBranch instead of the previous one.
	// Returns an error if targetBranch does not exist.
	CheckoutRef(ctx context.Context, sessionID, targetBranch string) error

	// AccumulateDuration adds ms to the session's running TotalDurationMs.
	// Failures are logged and swallowed so timing never breaks a commit.
	AccumulateDuration(ctx context.Context, sessionID string, ms int64)
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

var _ SessionManager = (*SessionService)(nil)
