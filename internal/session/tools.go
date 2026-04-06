package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/plugins"
)

// sessionToolsPlugin is a built-in ToolsPlugin that exposes session management
// tools to the LLM under the "agent__" namespace. It is created per-dispatch
// and bound to a specific session.
type SessionToolsPlugin struct {
	plugins.UnimplementedToolsPlugin
	manager   *SessionManager
	sessionID string
}

func (p *SessionToolsPlugin) GetLifecycle() plugins.Lifecycle {
	return nil
}

func (p *SessionToolsPlugin) ListTools(_ context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	tools := make([]plugins.ToolDefinition, 0)
	for _, def := range toolDefinitions {
		if plugins.MatchesToolsFilter(def, filter) {
			tools = append(tools, def)
		}
	}

	return &plugins.ListToolsResponse{
		Tools: tools,
	}, nil
}

func (p *SessionToolsPlugin) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	def, ok := toolDefinitions[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}

	return &def, nil
}

func (p *SessionToolsPlugin) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	def, ok := toolDefinitions[req.Tool]
	if !ok {
		return &plugins.ValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("unknown tool %q", req.Tool)},
		}, nil
	}
	return plugins.ValidateAgainstDefinition(def, req), nil
}

func (p *SessionToolsPlugin) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	switch req.Tool {
	case "session_set_title":
		return p.execSetTitle(req)
	case "session_set_description":
		return p.execSetDescription(req)
	case "session_list":
		return p.execList(req)
	case "session_get":
		return p.execGet(req)
	case "session_create":
		return p.execCreate(req)
	case "session_dispatch":
		return p.execDispatch(ctx, req)
	case "session_get_history":
		return p.execGetHistory(req)
	case "session_get_message":
		return p.execGetMessage(req)
	}
	return nil, fmt.Errorf("unknown session tool: %s", req.Tool)
}

func (p *SessionToolsPlugin) execSetTitle(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	title, _ := req.Arguments["title"].(string)
	if _, err := p.manager.UpdateMeta(p.sessionID, UpdateMetaOptions{Title: &title}); err != nil {
		return errResp(err), nil
	}
	return &plugins.ExecuteResponse{Result: fmt.Sprintf("title set to %q", title)}, nil
}

func (p *SessionToolsPlugin) execSetDescription(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	desc, _ := req.Arguments["description"].(string)
	if _, err := p.manager.UpdateMeta(p.sessionID, UpdateMetaOptions{Description: &desc}); err != nil {
		return errResp(err), nil
	}
	return &plugins.ExecuteResponse{Result: fmt.Sprintf("description set to %q", desc)}, nil
}

func (p *SessionToolsPlugin) execList(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	parent := p.sessionID
	if v, ok := req.Arguments["parent"].(string); ok && v != "" {
		parent = v
	}
	limit := 20
	if v, ok := req.Arguments["limit"].(float64); ok {
		limit = int(v)
	}
	offset := 0
	if v, ok := req.Arguments["offset"].(float64); ok {
		offset = int(v)
	}

	sessions, err := p.manager.List(ListOptions{Limit: limit, Offset: offset, ParentID: &parent})
	if err != nil {
		return errResp(err), nil
	}
	return jsonResp(sessions)
}

func (p *SessionToolsPlugin) execGet(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	id, _ := req.Arguments["session_id"].(string)
	sess, err := p.manager.Get(id)
	if err != nil {
		return errResp(err), nil
	}
	return jsonResp(sess)
}

func (p *SessionToolsPlugin) execCreate(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	parent, err := p.manager.Get(p.sessionID)
	if err != nil {
		return errResp(err), nil
	}

	opts := CreateOptions{
		Parent:            p.sessionID,
		Model:             parent.Model,
		Tools:             parent.Tools,
		MaxToolIterations: parent.MaxToolIterations,
	}
	if v, ok := req.Arguments["model"].(string); ok && v != "" {
		opts.Model = v
	}
	if v, ok := req.Arguments["title"].(string); ok {
		opts.Title = v
	}
	if v, ok := req.Arguments["description"].(string); ok {
		opts.Description = v
	}
	if v, ok := req.Arguments["system_prompt"].(string); ok {
		opts.SystemPrompt = v
	}
	if v, ok := req.Arguments["max_tool_iterations"].(float64); ok && v > 0 {
		opts.MaxToolIterations = int(v)
	}
	if raw, ok := req.Arguments["tools"].([]any); ok {
		tools := make([]string, 0, len(raw))
		for _, t := range raw {
			if s, ok := t.(string); ok {
				tools = append(tools, s)
			}
		}
		opts.Tools = tools
	}

	sess, err := p.manager.Create(opts)
	if err != nil {
		return errResp(err), nil
	}
	return jsonResp(sess)
}

func (p *SessionToolsPlugin) execDispatch(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	targetID, _ := req.Arguments["session_id"].(string)
	content, _ := req.Arguments["content"].(string)

	if targetID == p.sessionID {
		return &plugins.ExecuteResponse{
			Result:  "cannot dispatch to the current session",
			IsError: true,
		}, nil
	}

	// Verify the target exists and is owned by the current session.
	target, err := p.manager.Get(targetID)
	if err != nil {
		return errResp(err), nil
	}
	if target.Parent != p.sessionID {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("session %q is not a sub-session of the current session", targetID),
			IsError: true,
		}, nil
	}

	stream, err := p.manager.Dispatch(ctx, targetID, content)
	if err != nil {
		return errResp(err), nil
	}

	result, err := plugins.CollectStream(stream)
	if err != nil {
		return errResp(err), nil
	}

	return &plugins.ExecuteResponse{Result: result.Content}, nil
}

func (p *SessionToolsPlugin) execGetHistory(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	id, _ := req.Arguments["session_id"].(string)
	limit := 50
	if v, ok := req.Arguments["limit"].(float64); ok {
		limit = int(v)
	}
	offset := 0
	if v, ok := req.Arguments["offset"].(float64); ok {
		offset = int(v)
	}

	messages, err := p.manager.GetMessages(id, limit, offset)
	if err != nil {
		return errResp(err), nil
	}
	return jsonResp(messages)
}

func (p *SessionToolsPlugin) execGetMessage(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	sessionID, _ := req.Arguments["session_id"].(string)
	messageID, _ := req.Arguments["message_id"].(string)

	msg, err := p.manager.GetMessage(sessionID, messageID)
	if err != nil {
		return errResp(err), nil
	}
	return jsonResp(msg)
}

// errResp wraps an error as a tool error response.
func errResp(err error) *plugins.ExecuteResponse {
	return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}
}

// jsonResp serialises v to JSON and returns it as the tool result string.
func jsonResp(v any) (*plugins.ExecuteResponse, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to serialise result: %w", err)
	}
	return &plugins.ExecuteResponse{Result: string(b)}, nil
}
