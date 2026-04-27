package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/mwantia/forge/internal/service/metrics"
)

// StorageBackend - Capabilities: read, write, create, list, delete
type StorageBackend interface {
	// ReadRaw retrieves the value stored at key.
	// Returns nil, if the key does not exist.
	ReadRaw(ctx context.Context, key string) ([]byte, error)

	// ReadJSON fetches the value at key and JSON-unmarshals it.
	// Returns nil, when the key does not exist.
	ReadJson(ctx context.Context, key string, v any) error

	// WriteRaw writes value at key.
	WriteRaw(ctx context.Context, key string, value []byte) error

	// WriteJSON JSON-marshals val and writes it at key.
	WriteJson(ctx context.Context, key string, v any) error

	// CreateEntry creates a new key/prefix, creating any intermediate path segments as needed
	CreateEntry(ctx context.Context, key string) error

	// List returns the immediate children of prefix. Sub-prefix entries end
	// with "/"; leaf entries do not. prefix itself must end with "/" or be
	// empty.
	ListEntry(ctx context.Context, prefix string) ([]string, error)

	// Delete removes the value at key.
	// It is not an error if the key does not exist.
	DeleteEntry(ctx context.Context, key string) error

	// DeletePrefix removes all keys that share the given prefix. This is the
	// equivalent of a recursive / cascading delete.
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
