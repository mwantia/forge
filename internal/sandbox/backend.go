package sandbox

import (
	"context"
	"fmt"
	"strings"
)

// ListOptions controls filtering and pagination for List.
type ListOptions struct {
	Limit     int
	Offset    int
	SessionID string
}

// --- key helpers ---

func sandboxKey(sessionID, id string) string {
	return "sessions/" + sessionID + "/sandboxes/" + id + "/sandbox.json"
}

func sandboxPrefix(sessionID, id string) string {
	return "sessions/" + sessionID + "/sandboxes/" + id + "/"
}

// --- sandbox persistence ---

func (m *SandboxManager) saveSandbox(sb *Sandbox) error {
	return m.backend.PutJson(context.Background(), sandboxKey(sb.SessionID, sb.ID), sb)
}

func (m *SandboxManager) loadSandbox(sessionID, id string) (*Sandbox, error) {
	var s *Sandbox
	err := m.backend.GetJson(context.Background(), sandboxKey(sessionID, id), s)
	return s, err
}

func (m *SandboxManager) loadSandboxByID(id string) (*Sandbox, error) {
	sessions, err := m.backend.List(context.Background(), "sessions/")
	if err != nil {
		return nil, err
	}
	for _, e := range sessions {
		if !strings.HasSuffix(e, "/") {
			continue
		}
		sb, err := m.loadSandbox(strings.TrimSuffix(e, "/"), id)
		if err == nil {
			return sb, nil
		}
	}
	return nil, fmt.Errorf("sandbox not found: %s", id)
}

func (m *SandboxManager) deleteSandbox(sessionID, id string) error {
	return m.backend.DeletePrefix(context.Background(), sandboxPrefix(sessionID, id))
}

func (m *SandboxManager) listSandboxes(opts ListOptions) ([]*Sandbox, error) {
	sessions, err := m.backend.List(context.Background(), "sessions/")
	if err != nil {
		return nil, err
	}
	var out []*Sandbox
	for _, e := range sessions {
		if !strings.HasSuffix(e, "/") {
			continue
		}
		sessionID := strings.TrimSuffix(e, "/")
		if opts.SessionID != "" && sessionID != opts.SessionID {
			continue
		}
		prefix := "sessions/" + sessionID + "/sandboxes/"
		sbEntries, err := m.backend.List(context.Background(), prefix)
		if err != nil {
			continue
		}
		for _, se := range sbEntries {
			if !strings.HasSuffix(se, "/") {
				continue
			}
			sb, err := m.loadSandbox(sessionID, strings.TrimSuffix(se, "/"))
			if err == nil {
				out = append(out, sb)
			}
		}
	}
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
