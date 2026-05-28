package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/mwantia/forge/internal/infrastructure/metrics"
)

// StorageBackend - Capabilities: read, write, create, list, delete
type StorageBackend interface {
	ReadRaw(ctx context.Context, key string) ([]byte, error)
	ReadJson(ctx context.Context, key string, v any) error
	WriteRaw(ctx context.Context, key string, value []byte) error
	WriteJson(ctx context.Context, key string, v any) error
	CreateEntry(ctx context.Context, key string) error
	ListEntry(ctx context.Context, prefix string) ([]string, error)
	DeleteEntry(ctx context.Context, key string) error
	DeletePrefix(ctx context.Context, prefix string) error
}

func (s *StorageService) ReadRaw(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.backend == nil {
		return nil, fmt.Errorf("storage backend not initialized")
	}

	start := time.Now()
	data, err := s.backend.ReadRaw(ctx, key)

	OperationsTotal.WithLabelValues("read", metrics.ErrToStatusLabel(err)).Inc()
	OperationDuration.WithLabelValues("read").Observe(time.Since(start).Seconds())
	if err == nil {
		BytesTotal.WithLabelValues("read", "read").Add(float64(len(data)))
	}

	return data, err
}

func (s *StorageService) ReadJson(ctx context.Context, key string, v any) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}

	start := time.Now()
	err := s.backend.ReadJson(ctx, key, v)

	OperationsTotal.WithLabelValues("read", metrics.ErrToStatusLabel(err)).Inc()
	OperationDuration.WithLabelValues("read").Observe(time.Since(start).Seconds())

	return err
}

func (s *StorageService) WriteRaw(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}

	start := time.Now()
	err := s.backend.WriteRaw(ctx, key, value)

	OperationsTotal.WithLabelValues("write", metrics.ErrToStatusLabel(err)).Inc()
	OperationDuration.WithLabelValues("write").Observe(time.Since(start).Seconds())
	if err == nil {
		BytesTotal.WithLabelValues("write", "write").Add(float64(len(value)))
	}

	return err
}

func (s *StorageService) WriteJson(ctx context.Context, key string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}

	start := time.Now()
	err := s.backend.WriteJson(ctx, key, v)

	OperationsTotal.WithLabelValues("write", metrics.ErrToStatusLabel(err)).Inc()
	OperationDuration.WithLabelValues("write").Observe(time.Since(start).Seconds())

	return err
}

func (s *StorageService) CreateEntry(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}

	start := time.Now()
	err := s.backend.CreateEntry(ctx, key)

	OperationsTotal.WithLabelValues("create", metrics.ErrToStatusLabel(err)).Inc()
	OperationDuration.WithLabelValues("create").Observe(time.Since(start).Seconds())

	return err
}

func (s *StorageService) ListEntry(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.backend == nil {
		return nil, fmt.Errorf("storage backend not initialized")
	}

	start := time.Now()
	entries, err := s.backend.ListEntry(ctx, prefix)

	OperationsTotal.WithLabelValues("list", metrics.ErrToStatusLabel(err)).Inc()
	OperationDuration.WithLabelValues("list").Observe(time.Since(start).Seconds())

	return entries, err
}

func (s *StorageService) DeleteEntry(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}

	start := time.Now()
	err := s.backend.DeleteEntry(ctx, key)

	OperationsTotal.WithLabelValues("delete", metrics.ErrToStatusLabel(err)).Inc()
	OperationDuration.WithLabelValues("delete").Observe(time.Since(start).Seconds())

	return err
}

func (s *StorageService) DeletePrefix(ctx context.Context, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.backend == nil {
		return fmt.Errorf("storage backend not initialized")
	}

	start := time.Now()
	err := s.backend.DeletePrefix(ctx, prefix)

	OperationsTotal.WithLabelValues("delete", metrics.ErrToStatusLabel(err)).Inc()
	OperationDuration.WithLabelValues("delete").Observe(time.Since(start).Seconds())

	return err
}
