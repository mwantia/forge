package session

import "time"

type Session struct {
	ID                string    `json:"id"`
	Model             string    `json:"model"`
	Memory            string    `json:"memory,omitempty"`
	Tools             []string  `json:"tools,omitempty"`
	MaxToolIterations int       `json:"max_tool_iterations"`
	SystemPrompt      string    `json:"system_prompt,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	MessageCount      int       `json:"message_count"`
}

type Message struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	ToolCalls []ToolCallEntry `json:"tool_calls,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// ToolCallEntry records a tool invocation within a message.
type ToolCallEntry struct {
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    string         `json:"result,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}
