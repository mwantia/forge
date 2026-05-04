// Package dag implements the immutable, content-addressed Merkle DAG that
// backs Forge sessions. See docs/03-proposal-merkle-DAG-concept.md.
//
// Three object kinds keyed by SHA-256 of canonical JSON:
//   - MessageObj    one conversation turn
//   - PromptContext the materialized prompt sent to a provider
//   - ToolCatalog   snapshot of available tools at dispatch time
//
// Mutable refs ("HEAD" and named branches) live in the per-session ref store
// and point at message hashes.
package dag

import (
	"time"

	"github.com/mwantia/forge-sdk/pkg/contenthash"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
)

// MessageObj is one immutable conversation turn. Identity = sha256 of its
// canonical JSON. Author/timestamp/provenance live in MessageMeta, never
// here, so byte-identical turns dedup across sessions.
type MessageObj struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	ToolCalls  []MessageToolCall `json:"tool_calls,omitempty"`
	ParentHash string            `json:"parent_hash,omitempty"`
}

// MessageToolCall mirrors a provider tool-call request + result on the
// chain. Arguments map keys are sorted by the canonical encoder.
type MessageToolCall struct {
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    string         `json:"result,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

func (m *MessageObj) Hash() (string, error) { return contenthash.Hash(m) }

// MessageMeta is the sidecar provenance record for a MessageObj. Keyed by
// the message hash, never folded into the hashed object.
type MessageMeta struct {
	Hash        string                 `json:"hash"`
	SessionID   string                 `json:"session_id,omitempty"`
	ContextHash string                 `json:"context_hash,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	// Usage is the provider-reported token consumption for the turn that
	// produced this message. Populated for assistant messages; nil for
	// user/tool messages where no provider call was made.
	Usage *sdkplugins.TokenUsage `json:"usage,omitempty"`
}

// PromptContext records the exact prompt handed to a provider on one turn.
// Bodies live in the object store; this object only carries hashes + params.
// MessageHashes includes the system message hash (position 0) when present.
type PromptContext struct {
	Provider        string         `json:"provider"`
	Model           string         `json:"model"`
	MessageHashes   []string       `json:"message_hashes"`
	ToolCatalogHash string         `json:"tool_catalog_hash,omitempty"`
	Options         map[string]any `json:"options,omitempty"`
}

func (p *PromptContext) Hash() (string, error) { return contenthash.Hash(p) }

// ToolCatalog is the available-tools snapshot referenced by a PromptContext.
type ToolCatalog struct {
	Tools []ToolDefinition `json:"tools"`
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
}

func (t *ToolCatalog) Hash() (string, error) { return contenthash.Hash(t) }
