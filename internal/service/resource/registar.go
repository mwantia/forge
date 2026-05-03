package resource

import (
	"context"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
)

// ResourceRegistar is the narrow interface the rest of the agent depends on.
type ResourceRegistar interface {
	Store(ctx context.Context, path, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error)
	Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error)
	Forget(ctx context.Context, path, id string) error
	List(ctx context.Context, path string) ([]*sdkplugins.Resource, error)
	Get(ctx context.Context, path, id string) (*sdkplugins.Resource, error)
	Backend() string
}

func (s *ResourceService) Store(ctx context.Context, path, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	return store.Store(ctx, path, content, tags, metadata)
}

func (s *ResourceService) Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	return store.Recall(ctx, q)
}

func (s *ResourceService) Forget(ctx context.Context, path, id string) error {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	return store.Forget(ctx, path, id)
}

func (s *ResourceService) List(ctx context.Context, path string) ([]*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	return store.List(ctx, path)
}

func (s *ResourceService) Get(ctx context.Context, path, id string) (*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	return store.Get(ctx, path, id)
}

func (s *ResourceService) Backend() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backend
}

var _ ResourceRegistar = (*ResourceService)(nil)
