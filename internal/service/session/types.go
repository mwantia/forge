package session

import (
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
)

type SessionMetadata struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Parent      string     `json:"parent,omitempty"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	// ArchivedAt is non-nil when the session has been archived. Archived
	// sessions are immutable: ref/commit writes return 409.
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	// ArchiveResourceID is the resource-store ID of the archive envelope,
	// when the session has been archived through a ResourcePlugin.
	ArchiveResourceID string `json:"archive_resource_id,omitempty"`
	// ArchivePath is the resource path the envelope was stored under.
	ArchivePath string `json:"archive_path,omitempty"`
	// Usage aggregates provider-reported token consumption across every
	// turn dispatched against this session. Updated atomically with each
	// assistant message that carries a usage report.
	Usage *sdkplugins.TokenUsage `json:"usage,omitempty" swaggertype:"object"`
	// ToolsVerbosity controls how much plugin/tool guidance appears in the
	// assembled system prompt: "full" (default) includes plugin prose and
	// per-tool annotations; "basic" includes only plugin-level prose; "none"
	// omits all plugin and tool blocks entirely.
	ToolsVerbosity string `json:"tools_verbosity,omitempty"`
	// Plugins restricts which plugin namespaces are active for this session.
	// When non-empty, only the listed namespaces appear in the system prompt
	// and are offered as callable tools. Built-in namespaces (sessions,
	// resource) always remain active regardless of this list.
	Plugins []string `json:"plugins,omitempty"`
}

// Message is the session-layer projection of a dag.MessageObj plus its
// MessageMeta sidecar. Identity is the content hash; CreatedAt and
// ContextHash come from the per-session log entry, never from the
// hashed object.
type Message struct {
	Hash        string            `json:"hash"`
	ParentHash  string            `json:"parent_hash,omitempty"`
	Role        string            `json:"role"`
	Content     string            `json:"content"`
	ToolCalls   []MessageToolCall `json:"tool_calls,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	ContextHash string            `json:"context_hash,omitempty"`
	Usage       *sdkplugins.TokenUsage `json:"usage,omitempty" swaggertype:"object"`
}

type MessageToolCall struct {
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    string         `json:"result,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}
