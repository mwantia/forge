package storage

import (
	"context"
	"fmt"

	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/config/eval"
	"github.com/mwantia/forge/internal/storage/file"
)

type StorageBackendInjector struct {
	container.LifecycleService

	config  *config.AgentConfig `fabric:"config"`
	backend Backend
}

func (s *StorageBackendInjector) Init(ctx context.Context) error {
	if s.config.Storage == nil {
		s.backend = file.NewFileBackend("./data")
		return nil
	}

	eval := eval.NewEvalContext(nil)
	params, err := s.config.Storage.DecodeBody(eval)
	if err != nil {
		return fmt.Errorf("storage %q: failed to decode config: %w", s.config.Storage.Type, err)
	}

	switch s.config.Storage.Type {
	case "file":
		path, _ := params["path"].(string)
		s.backend = file.NewFileBackend(path)
		return nil
	}

	return fmt.Errorf("unknown storage backend %q", s.config.Storage.Type)
}

func (s *StorageBackendInjector) Cleanup(context.Context) error {
	return nil
}

// GetRaw implements [Backend].
func (s *StorageBackendInjector) GetRaw(ctx context.Context, key string) ([]byte, error) {
	if s.backend == nil {
		return nil, fmt.Errorf("storage backend not initialized")
	}
	return s.backend.GetRaw(ctx, key)
}

// GetJson implements [Backend].
func (s *StorageBackendInjector) GetJson(ctx context.Context, key string, v any) error {
	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}
	return s.backend.GetJson(ctx, key, v)
}

// PutRaw implements [Backend].
func (s *StorageBackendInjector) PutRaw(ctx context.Context, key string, value []byte) error {
	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}
	return s.backend.PutRaw(ctx, key, value)
}

// PutJson implements [Backend].
func (s *StorageBackendInjector) PutJson(ctx context.Context, key string, v any) error {
	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}
	return s.backend.PutJson(ctx, key, v)
}

// List implements [Backend].
func (s *StorageBackendInjector) List(ctx context.Context, prefix string) ([]string, error) {
	if s.backend == nil {
		return nil, fmt.Errorf("storage backend not initialized")
	}
	return s.backend.List(ctx, prefix)
}

// Delete implements [Backend].
func (s *StorageBackendInjector) Delete(ctx context.Context, key string) error {
	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}
	return s.backend.Delete(ctx, key)
}

// DeletePrefix implements [Backend].
func (s *StorageBackendInjector) DeletePrefix(ctx context.Context, prefix string) error {
	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}
	return s.backend.DeletePrefix(ctx, prefix)
}

var _ Backend = (*StorageBackendInjector)(nil)
