package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/template"
)

func (s *SessionService) ExecuteTool(ctx context.Context, request plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	switch request.Tool {
	case "update_session_title":
		return s.execUpdateSessionField(ctx, request.Arguments, "title")

	case "update_session_description":
		return s.execUpdateSessionField(ctx, request.Arguments, "description")

	case "read_session":
		sessionID, err := ResolveSessionArg(ctx, request.Arguments, "session_id")
		if err != nil {
			return nil, err
		}
		s.mu.RLock()
		meta, err := s.store.LoadSession(ctx, sessionID)
		s.mu.RUnlock()
		if err != nil {
			return nil, fmt.Errorf("failed to load session: %w", err)
		}

		return &plugins.ExecuteResponse{
			Result: meta,
		}, nil

	case "list_sub_sessions":
		parent, ok := ArgString(request.Arguments, "parent")
		if !ok {
			parent = CallerSessionID(ctx)
		}
		offset := ArgInt(request.Arguments, "offset", 0)
		limit := ArgInt(request.Arguments, "limit", 20)
		s.mu.RLock()
		sessions, err := s.store.ListParentSessions(ctx, parent, offset, limit)
		s.mu.RUnlock()
		if err != nil {
			return nil, fmt.Errorf("failed to list sub-sessions: %w", err)
		}

		return &plugins.ExecuteResponse{
			Result: sessions,
		}, nil

	case "create_session":
		return s.execCreateSession(ctx, request.Arguments)

	case "dispatch_session":
		return nil, fmt.Errorf("dispatch_session: not yet implemented")

	case "list_message_history":
		sessionID, err := ResolveSessionArg(ctx, request.Arguments, "session_id")
		if err != nil {
			return nil, err
		}
		offset := ArgInt(request.Arguments, "offset", 0)
		limit := ArgInt(request.Arguments, "limit", 50)
		s.mu.RLock()
		msgs, err := s.store.ListMessages(ctx, sessionID, offset, limit)
		s.mu.RUnlock()
		if err != nil {
			return nil, fmt.Errorf("failed to list messages: %w", err)
		}

		return &plugins.ExecuteResponse{
			Result: msgs,
		}, nil

	case "archive_session":
		sessionID, err := ResolveSessionArg(ctx, request.Arguments, "session_id")
		if err != nil {
			return nil, err
		}
		ref, _ := ArgString(request.Arguments, "ref")
		res, err := s.ArchiveSession(ctx, sessionID, ref)
		if err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: res}, nil

	case "clone_archived_session":
		sourceID, ok := ArgString(request.Arguments, "source_id")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "source_id")
		}
		name, _ := ArgString(request.Arguments, "name")
		clone, err := s.CloneSession(ctx, sourceID, name)
		if err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: clone}, nil

	case "read_message":
		sessionID, err := ResolveSessionArg(ctx, request.Arguments, "session_id")
		if err != nil {
			return nil, err
		}
		msgID, ok := ArgString(request.Arguments, "message_id")
		if !ok {
			return nil, fmt.Errorf("invalid Argument defined for %q", "message_id")
		}
		s.mu.RLock()
		msg, err := s.store.LoadMessage(ctx, sessionID, msgID)
		s.mu.RUnlock()
		if err != nil {
			return nil, fmt.Errorf("failed to load message: %w", err)
		}

		return &plugins.ExecuteResponse{
			Result: msg,
		}, nil
	}

	return nil, fmt.Errorf("unknown tool execution: %s (%s)", request.Tool, request.CallID)
}

func (s *SessionService) execUpdateSessionField(ctx context.Context, Args map[string]any, field string) (*plugins.ExecuteResponse, error) {
	sessionID, err := ResolveSessionArg(ctx, Args, "session_id")
	if err != nil {
		return nil, err
	}
	value, ok := ArgString(Args, field)
	if !ok {
		return nil, fmt.Errorf("invalid Argument defined for %q", field)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.store.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	switch field {
	case "title":
		meta.Title = value
	case "description":
		meta.Description = value
	}
	meta.UpdatedAt = time.Now()

	if err := s.store.SaveSession(ctx, meta); err != nil {
		return nil, err
	}
	return &plugins.ExecuteResponse{
		Result: fmt.Sprintf("session %s set to %q", field, value),
	}, nil
}

func (s *SessionService) execCreateSession(ctx context.Context, Args map[string]any) (*plugins.ExecuteResponse, error) {
	parentSession := CallerSessionID(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	var model, title, description, system, parent string
	var allowedPlugins []string
	if parentSession != "" {
		if p, err := s.store.LoadSession(ctx, parent); err == nil {
			parent = p.Model
		}
	}
	if str, ok := ArgString(Args, "model"); ok {
		model = strings.TrimSpace(str)
	} else {
		model = strings.TrimSpace(parent)
	}
	if model == "" {
		return nil, fmt.Errorf("missing Argument for %q and no caller session model available", "model")
	}

	title, _ = ArgString(Args, "title")
	description, _ = ArgString(Args, "description")
	system, _ = ArgString(Args, "system")
	allowedPlugins, _ = ArgStringSlice(Args, "plugins")

	now := time.Now()
	meta := &SessionMetadata{
		ID:          template.GenerateNewID(),
		Name:        template.GenerateUniqueName(),
		Title:       title,
		Description: description,
		Parent:      parent,
		Model:       model,
		System:      system,
		Plugins:     allowedPlugins,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.SaveSession(ctx, meta); err != nil {
		return nil, fmt.Errorf("failed to create sub-session: %w", err)
	}

	return &plugins.ExecuteResponse{
		Result: meta,
	}, nil
}
