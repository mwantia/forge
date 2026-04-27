package memory

import (
	"context"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/metrics"
)

type MemoryRegistar interface {
	Store(ctx context.Context, namespace, content string, metadata map[string]any) (*sdkplugins.MemoryResource, error)
	Retrieve(ctx context.Context, namespace, query string, limit int, filter map[string]any) ([]*sdkplugins.MemoryResource, error)

	// Backend returns the active backend name ("file" or a plugin name).
	// Memory is always available via the built-in file backend, so there is
	// no separate Enabled flag.
	Backend() string
}

func (s *MemoryService) Backend() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backend
}

func (s *MemoryService) Store(ctx context.Context, namespace, content string, metadata map[string]any) (*sdkplugins.MemoryResource, error) {
	store := s.currentStore()

	start := time.Now()
	res, err := store.Store(ctx, namespace, content, metadata)
	MemoryOperationDuration.WithLabelValues("store").Observe(time.Since(start).Seconds())
	MemoryOperationsTotal.WithLabelValues("store", metrics.ErrToStatusLabel(err)).Inc()
	return res, err
}

func (s *MemoryService) Retrieve(ctx context.Context, namespace, query string, limit int, filter map[string]any) ([]*sdkplugins.MemoryResource, error) {
	store := s.currentStore()

	start := time.Now()
	res, err := store.Retrieve(ctx, namespace, query, limit, filter)
	MemoryOperationDuration.WithLabelValues("retrieve").Observe(time.Since(start).Seconds())
	MemoryOperationsTotal.WithLabelValues("retrieve", metrics.ErrToStatusLabel(err)).Inc()
	return res, err
}

func (s *MemoryService) currentStore() memoryStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store
}

var _ MemoryRegistar = (*MemoryService)(nil)
