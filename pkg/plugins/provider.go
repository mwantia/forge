package plugins

import "context"

// ProviderPlugin acts as LLM provider to "provide" access to endpoints like Ollama, Anthropic, etc.
type ProviderPlugin interface {
	BasePlugin
	// Additional provider methods will be added here
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}

type GenerateRequest struct {
	Model       string         `json:"model"`
	Messages    []Message      `json:"messages"`
	Temperature float64        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Tools       []Tool         `json:"tools,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type GenerateResponse struct {
	ID        string     `json:"id"`
	Content   string     `json:"content"`
	Role      string     `json:"role"`
	Usage     *Usage     `json:"usage,omitempty"`
	Model     string     `json:"model"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}