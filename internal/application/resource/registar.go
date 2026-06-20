package resource

import (
	"context"

	domresource "github.com/mwantia/forge/internal/domain/resource"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
)

type ResourceRegistar = domresource.ResourceRegistar

func (s *ResourceService) Store(ctx context.Context, content, commitMessage string, meta domresource.ResourceMeta) (*domresource.Resource, error) {
	res, err := s.defaultStore.Store(ctx, content, commitMessage, meta)
	if err != nil {
		return nil, err
	}
	go s.indexContent(context.WithoutCancel(ctx), res.ID, res)
	return res, nil
}

func (s *ResourceService) Commit(ctx context.Context, id, content, commitMessage string) (*domresource.Resource, error) {
	res, err := s.defaultStore.Commit(ctx, id, content, commitMessage)
	if err != nil {
		return nil, err
	}
	go s.indexContent(context.WithoutCancel(ctx), res.ID, res)
	return res, nil
}

func (s *ResourceService) Recall(ctx context.Context, q domresource.RecallQuery) ([]*domresource.Resource, error) {
	if results, ok := s.recallSemantic(ctx, q.Query, q); ok {
		return results, nil
	}
	return s.defaultStore.Recall(ctx, q)
}

func (s *ResourceService) Forget(ctx context.Context, id string) error {
	if err := s.defaultStore.Forget(ctx, id); err != nil {
		return err
	}
	go s.removeFromIndex(context.WithoutCancel(ctx), id)
	return nil
}

func (s *ResourceService) List(ctx context.Context, filter []domresource.FilterPredicate) ([]*domresource.Resource, error) {
	return s.defaultStore.List(ctx, filter)
}

func (s *ResourceService) Get(ctx context.Context, id string) (*domresource.Resource, error) {
	return s.defaultStore.Get(ctx, id)
}

func (s *ResourceService) UpdateMeta(ctx context.Context, id string, meta domresource.ResourceMeta) error {
	return s.defaultStore.UpdateMeta(ctx, id, meta)
}

func (s *ResourceService) History(ctx context.Context, id string) ([]*domresource.ResourceRevision, error) {
	return s.defaultStore.History(ctx, id)
}

func (s *ResourceService) GetAt(ctx context.Context, hash string) (*dag.ResourceObj, error) {
	return s.defaultStore.GetAt(ctx, hash)
}

func (s *ResourceService) Revert(ctx context.Context, id, toHash string) error {
	return s.defaultStore.Revert(ctx, id, toHash)
}

var _ ResourceRegistar = (*ResourceService)(nil)
