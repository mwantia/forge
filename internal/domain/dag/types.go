// Package dag holds the pure value types for the content-addressed Merkle DAG
// that backs Forge sessions and resources. These types carry no I/O or
// infrastructure dependencies and are shared across domain interfaces.
//
// Operations (ObjectStore, RefStore, Walk) live in
// internal/service/session/dag/ (infrastructure layer) and import this package.
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

// ToolDefinition is a serialisable snapshot of a single tool entry stored
// inside a ToolCatalog. It is distinct from sdkplugins.ToolDefinition which
// carries the full live schema; this variant is minimal for content-hash stability.
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
// CommitMessage and CommittedAt are part of the canonical object so two commits
// with identical content but different messages or timestamps get distinct hashes.
type ResourceObj struct {
	ContentType   string    `json:"content_type"`
	Content       string    `json:"content"`
	CommitMessage string    `json:"commit_message,omitempty"`
	CommittedAt   time.Time `json:"committed_at"`
	ParentHash    string    `json:"parent_hash,omitempty"`
}

func (r *ResourceObj) Hash() (string, error) { return contenthash.Hash(r) }

// ResourceMeta is the mutable sidecar for a resource, stored at
// resources/meta/<id>.json. Mirrors domain/resource.ResourceMeta plus
// DAG bookkeeping fields (Hash, ID, IndexedAt, IndexedBy).
type ResourceMeta struct {
	ID          string         `json:"id"`
	Hash        string         `json:"hash"`
	Name        string         `json:"name,omitempty"`
	Type        string         `json:"type,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Description string         `json:"description,omitempty"`
	Session     string         `json:"session,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Extra       map[string]any `json:"extra,omitempty"`
	IndexedAt   *time.Time     `json:"indexed_at,omitempty"`
	IndexedBy   string         `json:"indexed_by,omitempty"`
}
