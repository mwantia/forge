package resource

import "time"

type FilterOp string

const (
	FilterOpEq       FilterOp = "eq"
	FilterOpPrefix   FilterOp = "prefix"
	FilterOpContains FilterOp = "contains"
	FilterOpGte      FilterOp = "gte"
	FilterOpLte      FilterOp = "lte"
)

type FilterPredicate struct {
	Key   string   `json:"key"`
	Op    FilterOp `json:"op"`
	Value any      `json:"value"`
}

type RecallQuery struct {
	Query         string
	Tags          []string
	Filter        []FilterPredicate
	CreatedAfter  time.Time
	CreatedBefore time.Time
	Limit         int
}

// ResourceMeta is the contracted set of metadata fields for a resource.
// The AI always sees exactly these fields — no arbitrary map keys.
type ResourceMeta struct {
	Name        string         `json:"name,omitempty"`
	Type        string         `json:"type,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Description string         `json:"description,omitempty"`
	Session     string         `json:"session,omitempty"`
	CreatedAt   time.Time      `json:"created_at,omitempty"`
	UpdatedAt   time.Time      `json:"updated_at,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

type Resource struct {
	ID      string       `json:"id"`
	Content string       `json:"content"`
	Score   float64      `json:"score,omitempty"`
	Meta    ResourceMeta `json:"meta"`
}

type ResourceRevision struct {
	Hash          string     `json:"hash"`
	CommitMessage string     `json:"commit_message,omitempty"`
	CommittedAt   time.Time  `json:"committed_at,omitempty"`
	IndexedAt     *time.Time `json:"indexed_at,omitempty"`
	IndexedBy     string     `json:"indexed_by,omitempty"`
}
