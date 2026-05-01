package resource

import (
	"context"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
)

// ResourceRegistar is the narrow interface the rest of the agent depends on.
type ResourceRegistar interface {
	Store(ctx context.Context, namespace, content string, metadata map[string]any) (*sdkplugins.Resource, error)
	Recall(ctx context.Context, namespace, query string, limit int, filter map[string]any) ([]*sdkplugins.Resource, error)
	Forget(ctx context.Context, namespace, id string) error
	List(ctx context.Context, namespace string) ([]*sdkplugins.Resource, error)
	Get(ctx context.Context, namespace, id string) (*sdkplugins.Resource, error)
	Backend() string
}

func (s *ResourceService) Store(ctx context.Context, namespace, content string, metadata map[string]any) (*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	return store.Store(ctx, namespace, content, metadata)
}

func (s *ResourceService) Recall(ctx context.Context, namespace, query string, limit int, filter map[string]any) ([]*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	return store.Recall(ctx, namespace, query, limit, filter)
}

func (s *ResourceService) Forget(ctx context.Context, namespace, id string) error {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	return store.Forget(ctx, namespace, id)
}

func (s *ResourceService) List(ctx context.Context, namespace string) ([]*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	return store.List(ctx, namespace)
}

func (s *ResourceService) Get(ctx context.Context, namespace, id string) (*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	return store.Get(ctx, namespace, id)
}

func (s *ResourceService) Backend() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backend
}

var _ ResourceRegistar = (*ResourceService)(nil)
