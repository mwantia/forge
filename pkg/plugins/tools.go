package plugins

import (
	"context"

	"github.com/mwantia/forge/pkg/errors"
)

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
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ExecuteRequest struct {
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
	CallID    string         `json:"call_id,omitempty"`
}

type ExecuteResponse struct {
	Result   any            `json:"result"`
	IsError  bool           `json:"is_error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// UnimplementedToolsPlugin can be embedded to satisfy ToolsPlugin with default
// implementations that return errors.ErrPluginCapabilityNotSupported.
type UnimplementedToolsPlugin struct{}

func (UnimplementedToolsPlugin) GetLifecycle() Lifecycle { return nil }

func (UnimplementedToolsPlugin) List(_ context.Context) (*ListToolsResponse, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedToolsPlugin) Execute(_ context.Context, _ ExecuteRequest) (*ExecuteResponse, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}
