package session

import (
	"context"
	"fmt"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
)

// pluginSessionStore delegates session and message lifecycle to a
// SessionsPlugin (e.g. OpenViking). It maps the plugin's PluginSession /
// PluginMessage shapes onto the local SessionMetadata / Message types.
//
// Local-only fields on SessionMetadata (Title, Description, Parent, Model,
// SystemPrompts, timestamps) are smuggled through PluginSession.Metadata so
// the plugin doesn't have to grow a forge-specific schema. ID, Name, and
// MessageCount come from the plugin directly.
type pluginSessionStore struct {
	plugin sdkplugins.SessionsPlugin
}

func (p *pluginSessionStore) LoadSession(ctx context.Context, id string) (*SessionMetadata, error) {
	ps, err := p.plugin.GetSession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load session %q: %w", id, err)
	}
	if ps == nil || ps.ID == "" {
		return nil, fmt.Errorf("invalid session id received: %s", id)
	}
	return pluginSessionToMeta(ps), nil
}

func (p *pluginSessionStore) SaveSession(ctx context.Context, s *SessionMetadata) error {
	// SessionsPlugin currently has no Update RPC. Newly created sessions
	// flow through CreateSession in handlers; subsequent edits (title,
	// description, system prompts) are written to the plugin's metadata
	// bag. Without a dedicated Update we re-create-or-merge: try Get first;
	// if absent, Create.
	existing, err := p.plugin.GetSession(ctx, s.ID)
	if err == nil && existing != nil && existing.ID != "" {
		// No remote update path — accept best-effort by re-issuing Create.
		// Plugins that disallow re-create should treat this as a no-op or
		// return an explicit error which we bubble up.
		_, cerr := p.plugin.CreateSession(ctx, s.Name, metaToPluginMetadata(s))
		if cerr != nil {
			return fmt.Errorf("failed to update session %q: %w", s.ID, cerr)
		}
		return nil
	}
	created, err := p.plugin.CreateSession(ctx, s.Name, metaToPluginMetadata(s))
	if err != nil {
		return fmt.Errorf("failed to save session %q: %w", s.ID, err)
	}
	if created != nil && created.ID != "" {
		s.ID = created.ID
	}
	return nil
}

func (p *pluginSessionStore) DeleteSession(ctx context.Context, id string) error {
	if _, err := p.plugin.DeleteSession(ctx, id); err != nil {
		return fmt.Errorf("failed to delete session %q: %w", id, err)
	}
	return nil
}

func (p *pluginSessionStore) ListSessions(ctx context.Context, offset, limit int) ([]*SessionMetadata, error) {
	sessions, err := p.plugin.ListSessions(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*SessionMetadata, 0, len(sessions))
	for _, ps := range sessions {
		out = append(out, pluginSessionToMeta(ps))
	}
	return out, nil
}

func (p *pluginSessionStore) ListParentSessions(ctx context.Context, parentID string, offset, limit int) ([]*SessionMetadata, error) {
	all, err := p.ListSessions(ctx, 0, 0)
	if err != nil {
		return nil, err
	}
	filtered := make([]*SessionMetadata, 0, len(all))
	for _, m := range all {
		if parentID != "" && m.Parent != parentID {
			continue
		}
		filtered = append(filtered, m)
	}
	if offset > 0 {
		if offset >= len(filtered) {
			return make([]*SessionMetadata, 0), nil
		}
		filtered = filtered[offset:]
	}
	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (p *pluginSessionStore) FindSessionByName(ctx context.Context, name string) (*SessionMetadata, error) {
	all, err := p.ListSessions(ctx, 0, 0)
	if err != nil {
		return nil, err
	}
	for _, m := range all {
		if m.Name == name {
			return m, nil
		}
	}
	return nil, nil
}

func (p *pluginSessionStore) LoadMessage(ctx context.Context, sessionID, msgID string) (*Message, error) {
	pm, err := p.plugin.GetMessage(ctx, sessionID, msgID)
	if err != nil {
		return nil, err
	}
	if pm == nil {
		return nil, fmt.Errorf("message with id %q and session %q not found", msgID, sessionID)
	}
	return pluginMessageToLocal(pm), nil
}

func (p *pluginSessionStore) SaveMessage(ctx context.Context, sessionID string, msg *Message) error {
	if _, err := p.plugin.AddMessage(ctx, sessionID, localMessageToPlugin(sessionID, msg)); err != nil {
		return fmt.Errorf("failed to save message for session %q: %w", sessionID, err)
	}
	return nil
}

func (p *pluginSessionStore) ListMessages(ctx context.Context, sessionID string, offset, limit int) ([]*Message, error) {
	msgs, err := p.plugin.ListMessages(ctx, sessionID, offset, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*Message, 0, len(msgs))
	for _, pm := range msgs {
		out = append(out, pluginMessageToLocal(pm))
	}
	return out, nil
}

func (p *pluginSessionStore) CountMessages(ctx context.Context, sessionID string) (int, error) {
	return p.plugin.CountMessages(ctx, sessionID)
}

func (p *pluginSessionStore) CompactToolsMessages(ctx context.Context, sessionID string) (int, error) {
	return p.plugin.CompactMessages(ctx, sessionID)
}

func pluginSessionToMeta(ps *sdkplugins.PluginSession) *SessionMetadata {
	if ps == nil {
		return nil
	}
	meta := &SessionMetadata{
		ID:   ps.ID,
		Name: ps.Author,
	}
	mergePluginMetadata(meta, ps.Metadata)
	return meta
}

func metaToPluginMetadata(s *SessionMetadata) map[string]any {
	if s == nil {
		return nil
	}

	return map[string]any{
		"name":        s.Name,
		"title":       s.Title,
		"description": s.Description,
		"parent":      s.Parent,
		"model":       s.Model,
		"system":      s.System,
		"created_at":  s.CreatedAt.Format(time.RFC3339Nano),
		"updated_at":  s.UpdatedAt.Format(time.RFC3339Nano),
	}
}

func mergePluginMetadata(meta *SessionMetadata, m map[string]any) {
	if m == nil {
		return
	}
	if str, ok := m["title"].(string); ok {
		meta.Title = str
	}
	if str, ok := m["description"].(string); ok {
		meta.Description = str
	}
	if str, ok := m["parent"].(string); ok {
		meta.Parent = str
	}
	if str, ok := m["model"].(string); ok {
		meta.Model = str
	}
	if str, ok := m["name"].(string); ok && str != "" {
		meta.Name = str
	}
	if str, ok := m["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
			meta.CreatedAt = t
		}
	}
	if str, ok := m["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
			meta.UpdatedAt = t
		}
	}
	if str, ok := m["system"].(string); ok {
		meta.System = str
	}
}

func pluginMessageToLocal(pm *sdkplugins.PluginMessage) *Message {
	if pm == nil {
		return nil
	}
	msg := &Message{
		ID:        pm.ID,
		Role:      pm.Role,
		Content:   pm.Content,
		CreatedAt: pm.CreatedAt,
	}
	if len(pm.ToolCalls) > 0 {
		msg.ToolCalls = make([]MessageToolCall, 0, len(pm.ToolCalls))
		for _, tc := range pm.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, MessageToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
				Result:    tc.Result,
				IsError:   tc.IsError,
			})
		}
	}
	return msg
}

func localMessageToPlugin(sessionID string, msg *Message) *sdkplugins.PluginMessage {
	if msg == nil {
		return nil
	}
	pm := &sdkplugins.PluginMessage{
		ID:        msg.ID,
		SessionID: sessionID,
		Role:      msg.Role,
		Content:   msg.Content,
		CreatedAt: msg.CreatedAt,
	}
	if len(msg.ToolCalls) > 0 {
		pm.ToolCalls = make([]sdkplugins.PluginToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			pm.ToolCalls = append(pm.ToolCalls, sdkplugins.PluginToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
				Result:    tc.Result,
				IsError:   tc.IsError,
			})
		}
	}
	return pm
}

var _ sessionBackend = (*pluginSessionStore)(nil)
