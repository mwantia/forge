// Package dag implements the immutable, content-addressed Merkle DAG that
// backs Forge sessions and resources. See docs/03-proposal-merkle-DAG-concept.md.
//
// Object kinds keyed by SHA-256 of canonical JSON:
//   - MessageObj    one conversation turn
//   - ResourceObj   one immutable revision of a named resource
//   - PromptContext the materialized prompt sent to a provider
//   - ToolCatalog   snapshot of available tools at dispatch time
//
// Mutable refs ("HEAD" and named branches for sessions; named resource refs)
// live in the ref store and point at content hashes.
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

// ResourceObj is one immutable revision of a named resource's content.
// Identity = sha256(canonical_json(ResourceObj)). ContentType lets the recall
// layer choose the right embedding strategy without inspecting the content.
// ParentHash links revisions into a version chain for history, diff, and revert.
type ResourceObj struct {
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
	ParentHash  string `json:"parent_hash,omitempty"`
}

func (r *ResourceObj) Hash() (string, error) { return contenthash.Hash(r) }

// ResourceMeta is the mutable sidecar for a ResourceObj revision.
// Never folded into the hashed object so tags and metadata can change
// without invalidating the content hash or breaking deduplication.
// Layout: resources/<namespace>/log/<020d-unix_nano>_<hash>.json
type ResourceMeta struct {
	Hash      string         `json:"hash"`
	Namespace string         `json:"namespace"`
	Name      string         `json:"name"`
	Tags      []string       `json:"tags,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`

	// Set after RecallPlugin.Index succeeds. Enables incremental rebuilds:
	// only re-index entries where IndexedAt is nil or before CreatedAt.
	IndexedAt *time.Time `json:"indexed_at,omitempty"`
	IndexedBy string     `json:"indexed_by,omitempty"`
}
