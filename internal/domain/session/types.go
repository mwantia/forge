package session

import (
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
)

// SessionQuery describes filters and pagination for listing sessions.
// Nil pointer fields are treated as "no filter".
type SessionQuery struct {
	// ParentID restricts to sessions whose Parent == ParentID.
	// Empty string = no parent filter (includes all sessions regardless of parent).
	ParentID string

	// Archived filters by archive status.
	// nil = both; &true = archived only; &false = active only.
	Archived *bool

	// Search is case-insensitive and matched as a substring against Name and Title.
	Search string

	// Plugins restricts to sessions that declare all listed plugins.
	// Sessions with an empty Plugins field are treated as "all plugins" and always match.
	Plugins []string

	Offset int
	Limit  int
}

// SessionMetadata is the mutable descriptor for a session.
type SessionMetadata struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Parent      string    `json:"parent,omitempty"`
	Model       string    `json:"model"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// ArchivedAt is non-nil when the session has been archived. Archived
	// sessions are immutable: ref/commit writes return 409.
	ArchivedAt        *time.Time `json:"archived_at,omitempty"`
	ArchiveResourceID string     `json:"archive_resource_id,omitempty"`
	ArchivePath       string     `json:"archive_path,omitempty"`
	// Usage aggregates provider-reported token consumption across all turns.
	Usage *sdkplugins.TokenUsage `json:"usage,omitempty"`
	// ToolsVerbosity controls how much plugin/tool guidance appears in the
	// assembled system prompt: "full" (default), "basic", or "none".
	ToolsVerbosity string `json:"tools_verbosity,omitempty"`
	// Plugins restricts which plugin namespaces are active for this session.
	Plugins []string `json:"plugins,omitempty"`
	// TotalDurationMs is the sum of wall-clock milliseconds spent in pipeline commits.
	TotalDurationMs int64 `json:"total_duration_ms,omitempty"`
}

// Message is the session-layer projection of a dag.MessageObj plus its
// MessageMeta sidecar. Identity is the content hash.
type Message struct {
	Hash        string                 `json:"hash"`
	ParentHash  string                 `json:"parent_hash,omitempty"`
	Role        string                 `json:"role"`
	Content     string                 `json:"content"`
	ToolCalls   []MessageToolCall      `json:"tool_calls,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	ContextHash string                 `json:"context_hash,omitempty"`
	Usage       *sdkplugins.TokenUsage `json:"usage,omitempty"`
}

// MessageToolCall is the session-layer representation of a tool call or result.
type MessageToolCall struct {
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    string         `json:"result,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}
