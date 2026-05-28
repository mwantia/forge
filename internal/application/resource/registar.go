package resource

import (
	"context"
	"strings"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	domresource "github.com/mwantia/forge/internal/domain/resource"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
)

type ResourceRegistar = domresource.ResourceRegistar

func (s *ResourceService) Store(ctx context.Context, path, name, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error) {
	res, err := s.defaultStore.Store(ctx, path, name, content, tags, metadata)
	if err != nil {
		return nil, err
	}
	go s.indexContent(context.WithoutCancel(ctx), namespace(path), res.ID, res)
	return res, nil
}

func (s *ResourceService) Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error) {
	effectivePath := q.Path
	if strings.ContainsAny(q.Path, "*?[") {
		effectivePath = strings.TrimRight(globBase(q.Path), "/")
		if effectivePath == "" {
			effectivePath = "/"
		}
	}

	// Semantic recall only applies to exact namespace paths — glob queries span
	// multiple namespaces and fall back to the substring scorer in defaultStore.
	if !strings.ContainsAny(q.Path, "*?[") {
		if results, ok := s.recallSemantic(ctx, namespace(effectivePath), q.Query, q); ok {
			return results, nil
		}
	}

	return s.defaultStore.Recall(ctx, q)
}

func (s *ResourceService) Forget(ctx context.Context, path, id string) error {
	if err := s.defaultStore.Forget(ctx, path, id); err != nil {
		return err
	}
	go s.removeFromIndex(context.WithoutCancel(ctx), namespace(path), id)
	return nil
}

func (s *ResourceService) List(ctx context.Context, path string) ([]*sdkplugins.Resource, error) {
	return s.defaultStore.List(ctx, path)
}

func (s *ResourceService) Get(ctx context.Context, path, id string) (*sdkplugins.Resource, error) {
	return s.defaultStore.Get(ctx, path, id)
}

func (s *ResourceService) UpdateMeta(ctx context.Context, path, name string, tags []string, metadata map[string]any) error {
	return s.defaultStore.UpdateMeta(ctx, path, name, tags, metadata)
}

func (s *ResourceService) History(ctx context.Context, path, name string) ([]*ResourceRevision, error) {
	return s.defaultStore.History(ctx, path, name)
}

func (s *ResourceService) GetAt(ctx context.Context, hash string) (*dag.ResourceObj, error) {
	return s.defaultStore.GetAt(ctx, hash)
}

func (s *ResourceService) Revert(ctx context.Context, path, name, toHash string) error {
	return s.defaultStore.Revert(ctx, path, name, toHash)
}

var _ ResourceRegistar = (*ResourceService)(nil)
