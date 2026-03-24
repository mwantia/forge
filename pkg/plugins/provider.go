package plugins

import (
	"context"

	"github.com/mwantia/forge/pkg/errors"
)

// ProviderPlugin acts as LLM provider to "provide" access to endpoints like Ollama, Anthropic, etc.
type ProviderPlugin interface {
	BasePlugin

	Chat(ctx context.Context, messages []ChatMessage, tools []ToolCall, model *Model) (*ChatResult, error)

	Embed(ctx context.Context, content string, model *Model) ([][]float32, error)

	ListModels(ctx context.Context) ([]*Model, error)
	CreateModel(ctx context.Context, modelName string, template *ModelTemplate) (*Model, error)
	GetModel(ctx context.Context, name string) (*Model, error)
	DeleteModel(ctx context.Context, name string) (bool, error)
}

type Model struct {
	ModelName   string         `json:"model_name,omitempty"`
	Dimension   int            `json:"dimension,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	Template    *ModelTemplate `json:"template,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type ModelTemplate struct {
	BaseModel      string         `json:"base_model"`
	PromptTemplate string         `json:"prompt_template,omitempty"`
	System         string         `json:"system,omitempty"`
	Parameters     map[string]any `json:"parameters,omitempty"`
}

type ChatResult struct {
	ID        string         `json:"id"`
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []ChatToolCall `json:"tool_calls,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type ChatToolCall struct {
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ChatMessage struct {
	Role      string                `json:"role"`
	Content   string                `json:"content"`
	ToolCalls *ChatMessageToolCalls `json:"tool_calls,omitempty"`
}

type ChatMessageToolCalls struct {
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	ToolCalls []ChatToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// UnimplementedProviderPlugin can be embedded to satisfy ProviderPlugin with default
// implementations that return errors.ErrPluginCapabilityNotSupported.
type UnimplementedProviderPlugin struct{}

func (UnimplementedProviderPlugin) GetLifecycle() Lifecycle { return nil }

func (UnimplementedProviderPlugin) Chat(_ context.Context, _ []ChatMessage, _ []ToolCall, _ *Model) (*ChatResult, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedProviderPlugin) Embed(_ context.Context, _ string, _ *Model) ([][]float32, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedProviderPlugin) ListModels(_ context.Context) ([]*Model, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedProviderPlugin) CreateModel(_ context.Context, _ string, _ *ModelTemplate) (*Model, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedProviderPlugin) GetModel(_ context.Context, _ string) (*Model, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedProviderPlugin) DeleteModel(_ context.Context, _ string) (bool, error) {
	return false, errors.ErrPluginCapabilityNotSupported
}
