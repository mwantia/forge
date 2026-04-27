package session

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mwantia/forge/internal/service/storage"
)

// sessionBackend is the storage surface that SessionService consumes. The
// service picks one implementation at Init: fileSessionStore or
// pluginSessionStore.
type sessionBackend interface {
	LoadSession(ctx context.Context, id string) (*SessionMetadata, error)
	SaveSession(ctx context.Context, s *SessionMetadata) error
	DeleteSession(ctx context.Context, id string) error
	ListParentSessions(ctx context.Context, parentID string, offset, limit int) ([]*SessionMetadata, error)
	ListSessions(ctx context.Context, offset, limit int) ([]*SessionMetadata, error)
	FindSessionByName(ctx context.Context, name string) (*SessionMetadata, error)

	LoadMessage(ctx context.Context, sessionID, msgID string) (*Message, error)
	SaveMessage(ctx context.Context, sessionID string, msg *Message) error
	ListMessages(ctx context.Context, sessionID string, offset, limit int) ([]*Message, error)
	CountMessages(ctx context.Context, sessionID string) (int, error)
	CompactToolsMessages(ctx context.Context, sessionID string) (int, error)
}

// fileSessionStore is the default file-backed session/message store. It is
// stateless w.r.t. concurrency; callers (SessionService) are responsible for
// locking.
type fileSessionStore struct {
	storage storage.StorageBackend
}

func (m *fileSessionStore) LoadSession(ctx context.Context, id string) (*SessionMetadata, error) {
	meta := &SessionMetadata{}
	key := constructSessionKey(id)

	if err := m.storage.ReadJson(ctx, key, meta); err != nil {
		return nil, fmt.Errorf("failed to load session %q: %w", id, err)
	}

	if meta.ID == "" {
		return nil, fmt.Errorf("invalid session id received: %s", id)
	}

	return meta, nil
}

func (m *fileSessionStore) SaveSession(ctx context.Context, s *SessionMetadata) error {
	key := constructSessionKey(s.ID)
	if err := m.storage.WriteJson(ctx, key, s); err != nil {
		return fmt.Errorf("failed to save session %q: %w", s.ID, err)
	}

	return nil
}

func (m *fileSessionStore) DeleteSession(ctx context.Context, id string) error {
	prefix := constructSessionPrefix(id)

	if err := m.storage.DeletePrefix(ctx, prefix); err != nil {
		return fmt.Errorf("failed to delete session %q: %w", id, err)
	}

	return nil
}

func (m *fileSessionStore) ListParentSessions(ctx context.Context, parentID string, offset, limit int) ([]*SessionMetadata, error) {
	entries, err := m.storage.ListEntry(ctx, "sessions/")
	if err != nil {
		return nil, err
	}

	errs := make([]error, 0)
	sessions := make([]*SessionMetadata, 0)

	for _, entry := range entries {
		if !strings.HasSuffix(entry, "/") {
			continue
		}

		id := strings.TrimSuffix(entry, "/")
		meta, err := m.LoadSession(ctx, id)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if parentID != "" && meta.Parent != parentID {
			continue
		}
		sessions = append(sessions, meta)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	if offset > 0 {
		if offset >= len(sessions) {
			return make([]*SessionMetadata, 0), nil
		}
		sessions = sessions[offset:]
	}
	if limit > 0 && limit < len(sessions) {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

func (m *fileSessionStore) ListSessions(ctx context.Context, offset, limit int) ([]*SessionMetadata, error) {
	return m.ListParentSessions(ctx, "", offset, limit)
}

func (m *fileSessionStore) FindSessionByName(ctx context.Context, name string) (*SessionMetadata, error) {
	entries, err := m.storage.ListEntry(ctx, "sessions/")
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if !strings.HasSuffix(e, "/") {
			continue
		}

		id := strings.TrimSuffix(e, "/")
		meta, err := m.LoadSession(ctx, id)
		if err != nil {
			continue
		}

		if meta.Name == name {
			return meta, nil
		}
	}

	return nil, nil
}

func (m *fileSessionStore) LoadMessage(ctx context.Context, sessionID, msgID string) (*Message, error) {
	prefix := constructMessagePrefix(sessionID)
	entries, err := m.storage.ListEntry(ctx, prefix)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry, ".json") {
			continue
		}
		msg := &Message{}
		key := prefix + entry

		if err := m.storage.ReadJson(ctx, key, msg); err != nil || msg.ID != msgID {
			continue
		}

		return msg, nil
	}

	return nil, fmt.Errorf("message with id %q and session %q not found", msgID, sessionID)
}

func (m *fileSessionStore) SaveMessage(ctx context.Context, sessionID string, msg *Message) error {
	key := constructMessageKey(sessionID, msg)

	if err := m.storage.WriteJson(ctx, key, msg); err != nil {
		return fmt.Errorf("failed to save message for session %q: %w", sessionID, err)
	}

	return nil
}

func (m *fileSessionStore) ListMessages(ctx context.Context, sessionID string, offset, limit int) ([]*Message, error) {
	prefix := constructMessagePrefix(sessionID)
	entries, err := m.storage.ListEntry(ctx, prefix)
	if err != nil {
		return nil, err
	}

	messages := make([]*Message, 0)
	for _, entry := range entries {
		if !strings.HasSuffix(entry, ".json") {
			continue
		}
		msg := &Message{}
		key := prefix + entry
		if err := m.storage.ReadJson(ctx, key, msg); err != nil || msg.ID == "" {
			continue
		}
		messages = append(messages, msg)
	}

	if offset > 0 {
		if offset >= len(messages) {
			return make([]*Message, 0), nil
		}
		messages = messages[offset:]
	}
	if limit > 0 && limit < len(messages) {
		messages = messages[:limit]
	}

	return messages, nil
}

func (m *fileSessionStore) CountMessages(ctx context.Context, sessionID string) (int, error) {
	prefix := constructMessagePrefix(sessionID)
	entries, err := m.storage.ListEntry(ctx, prefix)
	if err != nil {
		return 0, err
	}

	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e, ".json") {
			n++
		}
	}

	return n, nil
}

func (m *fileSessionStore) CompactToolsMessages(ctx context.Context, sessionID string) (int, error) {
	prefix := constructMessagePrefix(sessionID)
	entries, err := m.storage.ListEntry(ctx, prefix)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry, ".json") {
			continue
		}

		key := prefix + entry
		msg := &Message{}

		if err := m.storage.ReadJson(ctx, key, msg); err != nil || msg.ID == "" {
			continue
		}

		if msg.Role == "tool" || (msg.Role == "assistant" && len(msg.ToolCalls) > 0 && msg.Content == "") {
			if err := m.storage.DeleteEntry(ctx, key); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

var _ sessionBackend = (*fileSessionStore)(nil)
