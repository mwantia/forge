package session

import (
	"time"

	"github.com/mwantia/forge-sdk/pkg/plugins"
)

type Session struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Title             string              `json:"title,omitempty"`
	Description       string              `json:"description,omitempty"`
	Parent            string              `json:"parent,omitempty"`
	Model             string              `json:"model"`
	Memory            string              `json:"memory,omitempty"`
	Tools             []string            `json:"tools,omitempty"`
	MaxToolIterations int                 `json:"max_tool_iterations"`
	SystemPrompt      string              `json:"system_prompt,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	MessageCount      int                 `json:"message_count"`
	TotalUsage        *plugins.TokenUsage `json:"total_usage,omitempty"`
}

type Message struct {
	ID        string              `json:"id"`
	Role      string              `json:"role"`
	Content   string              `json:"content"`
	ToolCalls []ToolCallEntry     `json:"tool_calls,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	Usage     *plugins.TokenUsage `json:"usage,omitempty"`
}

// ToolCallEntry records a tool invocation within a message.
type ToolCallEntry struct {
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    string         `json:"result,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}
