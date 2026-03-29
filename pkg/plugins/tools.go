package plugins

import (
	"context"

	"github.com/mwantia/forge/pkg/errors"
)

type ToolCostHint string

var (
	ToolCostFree      ToolCostHint = "free"
	ToolCostCheap     ToolCostHint = "cheap"
	ToolCostModerate  ToolCostHint = "moderate"
	ToolCostExpensive ToolCostHint = "expensive"
)

// ToolsPlugin acts as bridge (or summary of embedded tools) for tool calling.
type ToolsPlugin interface {
	BasePlugin
	ListTools(ctx context.Context, filter ListToolsFilter) (*ListToolsResponse, error)
	GetTool(ctx context.Context, name string) (*ToolDefinition, error)
	Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)
	ExecuteStream(ctx context.Context, req ExecuteRequest) (<-chan ExecuteChunk, error)
	Cancel(ctx context.Context, callID string) error
	Validate(ctx context.Context, req ExecuteRequest) (*ValidateResponse, error)
}

// --- List ---

type ListToolsFilter struct {
	Tags       []string `json:"tags,omitempty"`
	Deprecated bool     `json:"deprecated,omitempty"`
	Prefix     string   `json:"prefix,omitempty"`
}

type ListToolsResponse struct {
	Tools []ToolDefinition `json:"tools"`
}

// --- Tool definition ---

type ToolAnnotations struct {
	ReadOnly             bool         `json:"read_only,omitempty"`
	Destructive          bool         `json:"destructive,omitempty"`
	Idempotent           bool         `json:"idempotent,omitempty"`
	RequiresConfirmation bool         `json:"requires_confirmation,omitempty"`
	CostHint             ToolCostHint `json:"cost_hint,omitempty"`
}

type ToolDefinition struct {
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Parameters         map[string]any  `json:"parameters"`
	Tags               []string        `json:"tags,omitempty"`
	Annotations        ToolAnnotations `json:"annotations"`
	Version            string          `json:"version,omitempty"`
	Deprecated         bool            `json:"deprecated,omitempty"`
	DeprecationMessage string          `json:"deprecation_message,omitempty"`
}

// --- Execute ---

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

// --- ExecuteStream ---

type ExecuteChunk struct {
	CallID  string `json:"call_id,omitempty"`
	Delta   any    `json:"delta,omitempty"`
	Done    bool   `json:"done,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

// --- Validate ---

type ValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// UnimplementedToolsPlugin can be embedded to satisfy ToolsPlugin with default
// implementations that return errors.ErrPluginCapabilityNotSupported.
type UnimplementedToolsPlugin struct{}

func (UnimplementedToolsPlugin) GetLifecycle() Lifecycle { return nil }

func (UnimplementedToolsPlugin) ListTools(_ context.Context, _ ListToolsFilter) (*ListToolsResponse, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedToolsPlugin) GetTool(_ context.Context, _ string) (*ToolDefinition, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedToolsPlugin) Execute(_ context.Context, _ ExecuteRequest) (*ExecuteResponse, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedToolsPlugin) ExecuteStream(_ context.Context, _ ExecuteRequest) (<-chan ExecuteChunk, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedToolsPlugin) Cancel(_ context.Context, _ string) error {
	return errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedToolsPlugin) Validate(_ context.Context, _ ExecuteRequest) (*ValidateResponse, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}
