package resource

import "time"

// ResourceRevision is one entry in a resource's parent-chain history.
type ResourceRevision struct {
	Hash      string         `json:"hash"`
	CreatedAt time.Time      `json:"created_at"`
	Tags      []string       `json:"tags,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	IndexedAt *time.Time     `json:"indexed_at,omitempty"`
	IndexedBy string         `json:"indexed_by,omitempty"`
}
