package resource

import (
	"context"

	domdag "github.com/mwantia/forge/internal/domain/dag"
)

type ResourceRegistar interface {
	Store(ctx context.Context, content, commitMessage string, meta ResourceMeta) (*Resource, error)
	Commit(ctx context.Context, id, content, commitMessage string) (*Resource, error)
	Recall(ctx context.Context, q RecallQuery) ([]*Resource, error)
	Forget(ctx context.Context, id string) error
	List(ctx context.Context, filter []FilterPredicate) ([]*Resource, error)
	Get(ctx context.Context, id string) (*Resource, error)
	UpdateMeta(ctx context.Context, id string, meta ResourceMeta) error
	History(ctx context.Context, id string) ([]*ResourceRevision, error)
	GetAt(ctx context.Context, hash string) (*domdag.ResourceObj, error)
	Revert(ctx context.Context, id, toHash string) error
}
