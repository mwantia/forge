package resource

import (
	"context"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	domdag "github.com/mwantia/forge/internal/domain/dag"
)

// ResourceRegistar is the narrow interface the rest of the agent depends on.
type ResourceRegistar interface {
	Store(ctx context.Context, path, name, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error)
	Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error)
	Forget(ctx context.Context, path, name string) error
	List(ctx context.Context, path string) ([]*sdkplugins.Resource, error)
	Get(ctx context.Context, path, name string) (*sdkplugins.Resource, error)
	UpdateMeta(ctx context.Context, path, name string, tags []string, metadata map[string]any) error
	History(ctx context.Context, path, name string) ([]*ResourceRevision, error)
	GetAt(ctx context.Context, hash string) (*domdag.ResourceObj, error)
	Revert(ctx context.Context, path, name, toHash string) error
}
