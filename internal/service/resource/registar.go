package resource

import (
	"context"
	"strings"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
)

// ResourceRegistar is the narrow interface the rest of the agent depends on.
type ResourceRegistar interface {
	Store(ctx context.Context, path, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error)
	Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error)
	Forget(ctx context.Context, path, id string) error
	List(ctx context.Context, path string) ([]*sdkplugins.Resource, error)
	Get(ctx context.Context, path, id string) (*sdkplugins.Resource, error)
}

// prefixMatches reports whether path falls under mountPrefix.
// "/" matches everything. "/global" matches "/global" and "/global/foo" but not "/globally".
func prefixMatches(path, mountPrefix string) bool {
	if mountPrefix == "/" {
		return true
	}
	return path == mountPrefix || strings.HasPrefix(path, mountPrefix+"/")
}

// resolveStore returns the store responsible for the given normalized path.
// Caller must hold s.mu.RLock (or s.mu.Lock during Serve).
func (s *ResourceService) resolveStore(path string) resourceStore {
	// Normalize: single leading slash, no trailing slash.
	path = "/" + strings.Trim(path, "/")
	for _, m := range s.mounts {
		if prefixMatches(path, m.prefix) {
			return m.store
		}
	}
	return s.defaultStore
}

func (s *ResourceService) Store(ctx context.Context, path, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.resolveStore(path)
	s.mu.RUnlock()
	return store.Store(ctx, path, content, tags, metadata)
}

func (s *ResourceService) Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error) {
	effectivePath := q.Path
	if strings.ContainsAny(q.Path, "*?[") {
		effectivePath = strings.TrimRight(globBase(q.Path), "/")
		if effectivePath == "" {
			effectivePath = "/"
		}
	}
	s.mu.RLock()
	store := s.resolveStore(effectivePath)
	s.mu.RUnlock()
	return store.Recall(ctx, q)
}

func (s *ResourceService) Forget(ctx context.Context, path, id string) error {
	s.mu.RLock()
	store := s.resolveStore(path)
	s.mu.RUnlock()
	return store.Forget(ctx, path, id)
}

func (s *ResourceService) List(ctx context.Context, path string) ([]*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.resolveStore(path)
	s.mu.RUnlock()
	return store.List(ctx, path)
}

func (s *ResourceService) Get(ctx context.Context, path, id string) (*sdkplugins.Resource, error) {
	s.mu.RLock()
	store := s.resolveStore(path)
	s.mu.RUnlock()
	return store.Get(ctx, path, id)
}

var _ ResourceRegistar = (*ResourceService)(nil)
