package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
)

// sessionToolsPlugin is a built-in ToolsPlugin that exposes session management
// tools to the LLM under the "agent__" namespace. It is created per-dispatch
// and bound to a specific session.
type sessionToolsPlugin struct {
	plugins.UnimplementedToolsPlugin
	manager   *Manager
	sessionID string
}

func (p *sessionToolsPlugin) ListTools(_ context.Context, _ plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	return &plugins.ListToolsResponse{
		Tools: []plugins.ToolDefinition{
			{
				Name:        "session_set_title",
				Description: "Set a short, human-readable title for the current session that summarises the conversation topic.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "The title to assign to this session.",
						},
					},
					"required": []any{"title"},
				},
			},
			{
				Name:        "session_set_description",
				Description: "Set a longer description for the current session that provides additional context about the conversation.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"description": map[string]any{
							"type":        "string",
							"description": "The description to assign to this session.",
						},
					},
					"required": []any{"description"},
				},
			},
			{
				Name:        "session_list",
				Description: "List sub-sessions owned by the current session. Optionally filter by a different parent session ID.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"parent": map[string]any{
							"type":        "string",
							"description": "Filter by parent session ID. Defaults to the current session ID.",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Maximum number of sessions to return. Defaults to 20.",
						},
						"offset": map[string]any{
							"type":        "integer",
							"description": "Pagination offset. Defaults to 0.",
						},
					},
				},
			},
			{
				Name:        "session_get",
				Description: "Get the current state (metadata) of a session by its ID.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"session_id": map[string]any{
							"type":        "string",
							"description": "The ID of the session to retrieve.",
						},
					},
					"required": []any{"session_id"},
				},
			},
			{
				Name:        "session_create",
				Description: "Create a new sub-session owned by the current session. The new session inherits the current session's model and tools unless overridden.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"model": map[string]any{
							"type":        "string",
							"description": "LLM model to use. Defaults to the current session's model.",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "Optional short title for the sub-session.",
						},
						"description": map[string]any{
							"type":        "string",
							"description": "Optional description for the sub-session.",
						},
						"system_prompt": map[string]any{
							"type":        "string",
							"description": "Optional system prompt for the sub-session.",
						},
						"tools": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Tool plugin names to enable. Defaults to the current session's tools.",
						},
						"max_tool_iterations": map[string]any{
							"type":        "integer",
							"description": "Maximum tool call iterations. Defaults to the current session's value.",
						},
					},
				},
			},
			{
				Name:        "session_dispatch",
				Description: "Send a message to a sub-session and wait for the full response. Blocks until the sub-session finishes processing.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"session_id": map[string]any{
							"type":        "string",
							"description": "The ID of the sub-session to dispatch to.",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "The message content to send.",
						},
					},
					"required": []any{"session_id", "content"},
				},
			},
			{
				Name:        "session_get_history",
				Description: "Retrieve the message history of a session.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"session_id": map[string]any{
							"type":        "string",
							"description": "The ID of the session.",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Maximum number of messages to return. Defaults to 50.",
						},
						"offset": map[string]any{
							"type":        "integer",
							"description": "Pagination offset. Defaults to 0.",
						},
					},
					"required": []any{"session_id"},
				},
			},
			{
				Name:        "session_get_message",
				Description: "Retrieve a single message by its ID from a session.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"session_id": map[string]any{
							"type":        "string",
							"description": "The ID of the session containing the message.",
						},
						"message_id": map[string]any{
							"type":        "string",
							"description": "The ID of the message to retrieve.",
						},
					},
					"required": []any{"session_id", "message_id"},
				},
			},
		},
	}, nil
}

func (p *sessionToolsPlugin) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	resp, _ := p.ListTools(context.Background(), plugins.ListToolsFilter{})
	for _, def := range resp.Tools {
		if def.Name == name {
			return &def, nil
		}
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

func (p *sessionToolsPlugin) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
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

func (p *sessionToolsPlugin) execSetTitle(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	title, _ := req.Arguments["title"].(string)
	if _, err := p.manager.UpdateMeta(p.sessionID, UpdateMetaOptions{Title: &title}); err != nil {
		return errResp(err), nil
	}
	return &plugins.ExecuteResponse{Result: fmt.Sprintf("title set to %q", title)}, nil
}

func (p *sessionToolsPlugin) execSetDescription(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	desc, _ := req.Arguments["description"].(string)
	if _, err := p.manager.UpdateMeta(p.sessionID, UpdateMetaOptions{Description: &desc}); err != nil {
		return errResp(err), nil
	}
	return &plugins.ExecuteResponse{Result: fmt.Sprintf("description set to %q", desc)}, nil
}

func (p *sessionToolsPlugin) execList(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
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

func (p *sessionToolsPlugin) execGet(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	id, _ := req.Arguments["session_id"].(string)
	sess, err := p.manager.Get(id)
	if err != nil {
		return errResp(err), nil
	}
	return jsonResp(sess)
}

func (p *sessionToolsPlugin) execCreate(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
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

func (p *sessionToolsPlugin) execDispatch(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
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

func (p *sessionToolsPlugin) execGetHistory(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
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

func (p *sessionToolsPlugin) execGetMessage(req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
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
