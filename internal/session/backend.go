package session

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/random"
)

// ListOptions controls filtering and pagination for List.
type ListOptions struct {
	Limit    int
	Offset   int
	ParentID *string
}

// --- key helpers ---

func sessionKey(id string) string       { return "sessions/" + id + "/session.json" }
func messagesPrefix(id string) string   { return "sessions/" + id + "/messages/" }
func messageKey(id, name string) string { return messagesPrefix(id) + name }

// --- session persistence ---

func (m *SessionManager) saveSession(sess *Session) error {
	return m.backend.PutJson(context.Background(), sessionKey(sess.ID), sess)
}

func (m *SessionManager) loadSession(id string) (*Session, error) {
	session := &Session{}
	if err := m.backend.GetJson(context.Background(), sessionKey(id), session); err != nil {
		return nil, err
	}
	if session.ID == "" {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return session, nil
}

func (m *SessionManager) findByName(name string) (*Session, error) {
	entries, err := m.backend.List(context.Background(), "sessions/")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e, "/") {
			continue
		}
		sess, err := m.loadSession(strings.TrimSuffix(e, "/"))
		if err != nil {
			continue
		}
		if sess.Name == name {
			return sess, nil
		}
	}
	return nil, nil
}

func (m *SessionManager) deleteSession(id string) error {
	return m.backend.DeletePrefix(context.Background(), "sessions/"+id+"/")
}

func (m *SessionManager) listSessions(opts ListOptions) ([]*Session, error) {
	entries, err := m.backend.List(context.Background(), "sessions/")
	if err != nil {
		return nil, err
	}
	var out []*Session
	for _, e := range entries {
		if !strings.HasSuffix(e, "/") {
			continue
		}
		sess, err := m.loadSession(strings.TrimSuffix(e, "/"))
		if err != nil {
			continue
		}
		if opts.ParentID != nil && sess.Parent != *opts.ParentID {
			continue
		}
		out = append(out, sess)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if opts.Offset > 0 {
		if opts.Offset >= len(out) {
			return nil, nil
		}
		out = out[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(out) {
		out = out[:opts.Limit]
	}
	return out, nil
}

func (m *SessionManager) generateUniqueName() string {
	return random.GenerateNewID()[:8]
}

// --- message persistence ---

func (m *SessionManager) saveMessage(sessionID string, msg *Message) error {
	name := fmt.Sprintf("%020d_%s.json", msg.CreatedAt.UnixNano(), msg.ID)
	return m.backend.PutJson(context.Background(), messageKey(sessionID, name), msg)
}

func (m *SessionManager) listMessages(sessionID string, limit, offset int) ([]*Message, error) {
	entries, err := m.backend.List(context.Background(), messagesPrefix(sessionID))
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if offset >= len(entries) {
			return nil, nil
		}
		entries = entries[offset:]
	}
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}
	var out []*Message
	for _, e := range entries {
		if !strings.HasSuffix(e, ".json") {
			continue
		}
		msg := &Message{}
		if err := m.backend.GetJson(context.Background(), messageKey(sessionID, e), msg); err != nil || msg.ID == "" {
			continue
		}
		out = append(out, msg)
	}
	return out, nil
}

func (m *SessionManager) getMessage(sessionID, messageID string) (*Message, error) {
	entries, err := m.backend.List(context.Background(), messagesPrefix(sessionID))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !strings.Contains(e, messageID) {
			continue
		}
		msg := &Message{}
		if err := m.backend.GetJson(context.Background(), messageKey(sessionID, e), msg); err != nil || msg.ID == "" {
			continue
		}
		if msg.ID == messageID {
			return msg, nil
		}
	}
	return nil, fmt.Errorf("message not found: %s", messageID)
}

func (m *SessionManager) countMessages(sessionID string) int {
	entries, _ := m.backend.List(context.Background(), messagesPrefix(sessionID))
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e, ".json") {
			n++
		}
	}
	return n
}

func (m *SessionManager) compactMessages(sessionID string, stripTools bool) (int, error) {
	entries, err := m.backend.List(context.Background(), messagesPrefix(sessionID))
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, e := range entries {
		if !strings.HasSuffix(e, ".json") {
			continue
		}
		key := messageKey(sessionID, e)
		msg := &Message{}
		if err := m.backend.GetJson(context.Background(), key, msg); err != nil || msg.ID == "" {
			continue
		}
		if stripTools && (msg.Role == "tool" ||
			(msg.Role == "assistant" && len(msg.ToolCalls) > 0 && msg.Content == "")) {
			if err := m.backend.Delete(context.Background(), key); err == nil {
				deleted++
			}
		}
	}
	return deleted, nil
}
