package plugins

import "context"

// MemoryPlugin acts as memory management for endpoints like OpenViking.
type MemoryPlugin interface {
	BasePlugin
	// Additional memory methods will be added here
	Store(ctx context.Context, req StoreRequest) (*StoreResponse, error)
	Retrieve(ctx context.Context, req RetrieveRequest) (*RetrieveResponse, error)
}

type StoreRequest struct {
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
}

type StoreResponse struct {
	ID string `json:"id"`
}

type RetrieveRequest struct {
	Query     string         `json:"query"`
	Limit     int            `json:"limit,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
	Filter    map[string]any `json:"filter,omitempty"`
}

type RetrieveResponse struct {
	Results []MemoryResult `json:"results"`
}

type MemoryResult struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata,omitempty"`
}