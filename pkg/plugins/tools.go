package plugins

import "context"

// ToolsPlugin acts as bridge (or summary of embedded tools) for tool calling.
type ToolsPlugin interface {
	BasePlugin
	// Additional tools methods will be added here
	List(ctx context.Context) (*ListToolsResponse, error)
	Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)
}

type ListToolsResponse struct {
	Tools []ToolDefinition `json:"tools"`
}

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ExecuteRequest struct {
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
	CallID    string         `json:"call_id,omitempty"`
}

type ExecuteResponse struct {
	Result     any            `json:"result"`
	IsError    bool           `json:"is_error,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}