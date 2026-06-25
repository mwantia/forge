package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	plugins "github.com/mwantia/forge-sdk/pkg/plugin"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
)

func (s *SessionService) ExecuteTool(ctx context.Context, request plugins.ExecuteToolRequest) (*plugins.ExecuteToolResponse, error) {
	args := request.Args.AsMap()
	switch request.Tool {
	case "update_session":
		return s.execUpdateSession(ctx, args)

	case "query_sessions":
		parent, ok := ArgString(args, "parent")
		if !ok {
			parent = CallerSessionID(ctx)
		}
		offset := ArgInt(args, "offset", 0)
		limit := ArgInt(args, "limit", 20)

		// archived param: absent = active only; true = archived; false = active
		archived := false
		if v, ok := args["archived"]; ok {
			if b, ok := v.(bool); ok {
				archived = b
			}
		}

		s.mu.RLock()
		sessions, err := s.store.ListParentSessions(ctx, parent, archived, offset, limit)
		s.mu.RUnlock()
		if err != nil {
			return nil, fmt.Errorf("failed to list sessions: %w", err)
		}
		return plugins.ExecuteSuccess(sessions), nil

	case "create_session":
		return s.execCreateSession(ctx, args)

	case "query_messages":
		sessionID, err := ResolveSessionArg(ctx, args, "session_id")
		if err != nil {
			return nil, err
		}
		offset := ArgInt(args, "offset", 0)
		limit := ArgInt(args, "limit", 50)

		s.mu.RLock()
		msgs, err := s.store.ListMessages(ctx, sessionID, offset, limit)
		s.mu.RUnlock()
		if err != nil {
			return nil, fmt.Errorf("failed to list messages: %w", err)
		}

		// Post-fetch filtering.
		role, hasRoleFilter := ArgString(args, "role")
		hasToolCallsFilter, filterToolCalls := args["has_tool_calls"]
		wantToolCalls, _ := hasToolCallsFilter.(bool)

		if hasRoleFilter || filterToolCalls {
			filtered := msgs[:0:0]
			for _, m := range msgs {
				if hasRoleFilter && !strings.EqualFold(m.Role, role) {
					continue
				}

				if filterToolCalls && wantToolCalls != (len(m.ToolCalls) > 0) {
					continue
				}

				filtered = append(filtered, m)
			}

			msgs = filtered
		}

		return plugins.ExecuteSuccess(msgs), nil

	case "archive_session":
		sessionID, err := ResolveSessionArg(ctx, args, "session_id")
		if err != nil {
			return nil, err
		}
		ref, _ := ArgString(args, "ref")
		res, err := s.ArchiveSession(ctx, sessionID, ref, "")
		if err != nil {
			return nil, err
		}
		return plugins.ExecuteSuccess(res), nil

	case "clone_session":
		sourceID, ok := ArgString(args, "source_id")
		if !ok {
			sourceID = CallerSessionID(ctx)
		}

		if sourceID == "" {
			return nil, fmt.Errorf("missing argument %q and no caller session available", "source_id")
		}

		name, _ := ArgString(args, "name")
		clone, err := s.CloneSession(ctx, sourceID, name)
		if err != nil {
			return nil, err
		}

		return plugins.ExecuteSuccess(clone), nil
	}

	return nil, fmt.Errorf("unknown tool execution: %s (%s)", request.Tool, request.CallID)
}

func (s *SessionService) execUpdateSession(ctx context.Context, args map[string]any) (*plugins.ExecuteToolResponse, error) {
	sessionID, err := ResolveSessionArg(ctx, args, "session_id")
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.store.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	changes := map[string]map[string]string{}

	if title, ok := ArgString(args, "title"); ok {
		changes["title"] = map[string]string{
			"previous": meta.Title,
			"new":      title,
		}
		meta.Title = title
	}

	if desc, ok := ArgString(args, "description"); ok {
		changes["description"] = map[string]string{
			"previous": meta.Description,
			"new":      desc,
		}
		meta.Description = desc
	}

	if mode, ok := ArgString(args, "mode"); ok {
		changes["mode"] = map[string]string{
			"previous": ModeOrDefault(meta.Mode),
			"new":      mode,
		}
		meta.Mode = mode
	}

	if len(changes) == 0 {
		return plugins.ExecuteSuccess("no fields provided"), nil
	}

	meta.UpdatedAt = time.Now()
	if err := s.store.SaveSession(ctx, meta); err != nil {
		return nil, err
	}

	return plugins.ExecuteSuccess(changes), nil
}

func (s *SessionService) execCreateSession(ctx context.Context, args map[string]any) (*plugins.ExecuteToolResponse, error) {
	callerID := CallerSessionID(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Inherit model from caller session; agent may override via "model" arg.
	var model string
	if callerID != "" {
		if caller, err := s.store.LoadSession(ctx, callerID); err == nil {
			model = caller.Model
		}
	}
	if str, ok := ArgString(args, "model"); ok && strings.TrimSpace(str) != "" {
		model = strings.TrimSpace(str)
	}
	if model == "" {
		return nil, fmt.Errorf("missing argument %q and no caller session model available", "model")
	}

	title, _ := ArgString(args, "title")
	description, _ := ArgString(args, "description")
	pluginNames, _ := ArgStringSlice(args, "plugins")

	now := time.Now()
	meta := &SessionMetadata{
		ID:          infratemplate.GenerateNewID(),
		Name:        infratemplate.GenerateUniqueName(),
		Title:       title,
		Description: description,
		Parent:      callerID,
		Model:       model,
		Plugins:     PluginConfigsFromNames(pluginNames),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.store.SaveSession(ctx, meta); err != nil {
		return nil, fmt.Errorf("failed to create sub-session: %w", err)
	}

	return plugins.ExecuteSuccess(meta), nil
}
