package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
)

type FileStorageBackend struct {
	root   string
	logger hclog.Logger
}

func NewFileStorageBackend(logger hclog.Logger, path string) *FileStorageBackend {
	return &FileStorageBackend{
		root:   path,
		logger: logger,
	}
}

func (b *FileStorageBackend) constructKeyPath(key string) string {
	return filepath.Join(b.root, filepath.FromSlash(key))
}

// Read retrieves the value stored at key.
// Returns nil, if the key does not exist.
func (b *FileStorageBackend) ReadRaw(ctx context.Context, key string) ([]byte, error) {
	path := b.constructKeyPath(key)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make([]byte, 0), nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read raw data from %q: %w", key, err)
	}

	return data, nil
}

// ReadJSON fetches the value at key and JSON-unmarshals it.
// Returns nil, when the key does not exist.
func (b *FileStorageBackend) ReadJson(ctx context.Context, key string, v any) error {
	raw, err := b.ReadRaw(ctx, key)
	if err != nil {
		return err
	}

	if raw == nil {
		return nil
	}

	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("failed to unmarshal json data for key %q: %w", key, err)
	}

	return nil
}

// Write writes value at key, creating any intermediate path segments as needed.
func (b *FileStorageBackend) WriteRaw(ctx context.Context, key string, value []byte) error {
	path := b.constructKeyPath(key)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directories for path %q: %w", key, err)
	}

	if err := os.WriteFile(path, value, 0644); err != nil {
		return fmt.Errorf("failed to write raw data to path %q: %w", key, err)
	}

	return nil
}

// WriteJSON JSON-marshals val and writes it at key.
func (b *FileStorageBackend) WriteJson(ctx context.Context, key string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data for key %q: %w", key, err)
	}

	return b.WriteRaw(ctx, key, data)
}

// CreateEntry creates a new key/prefix, creating any intermediate path segments as needed.
func (b *FileStorageBackend) CreateEntry(ctx context.Context, key string) error {
	path := b.constructKeyPath(key)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directories for path %q: %w", path, err)
	}

	if _, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0666); err != nil {
		return fmt.Errorf("failed to create empty file for path %q: %w", path, err)
	}

	return nil
}

func (b *FileStorageBackend) ListEntry(ctx context.Context, prefix string) ([]string, error) {
	dir := filepath.Join(b.root, filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list files for directory %q: %w", dir, err)
	}

	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name()+"/")
		} else {
			out = append(out, e.Name())
		}
	}

	return out, nil
}

func (b *FileStorageBackend) DeleteEntry(ctx context.Context, key string) error {
	path := b.constructKeyPath(key)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to delete file from path %q: %w", key, err)
	}

	return nil
}

func (b *FileStorageBackend) DeletePrefix(ctx context.Context, prefix string) error {
	path := filepath.Join(b.root, filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
	err := os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("failed to delete file(s) from path-prefix %q: %w", prefix, err)
	}

	return nil
}
