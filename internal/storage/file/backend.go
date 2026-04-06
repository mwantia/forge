package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileBackend struct {
	root string
}

func NewFileBackend(path string) *FileBackend {
	if path == "" {
		path = "./data"
	}

	return &FileBackend{
		root: path,
	}
}

func (b *FileBackend) keyPath(key string) string {
	return filepath.Join(b.root, filepath.FromSlash(key))
}

func (b *FileBackend) GetRaw(ctx context.Context, key string) ([]byte, error) {
	data, err := os.ReadFile(b.keyPath(key))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage get %q: %w", key, err)
	}
	return data, nil
}

func (b *FileBackend) GetJson(ctx context.Context, key string, v any) error {
	buf, err := b.GetRaw(ctx, key)
	if err != nil {
		return err
	}
	if buf == nil {
		return nil
	}

	return json.Unmarshal(buf, v)
}

func (b *FileBackend) PutRaw(ctx context.Context, key string, value []byte) error {
	path := b.keyPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("storage put %q: %w", key, err)
	}
	if err := os.WriteFile(path, value, 0644); err != nil {
		return fmt.Errorf("storage put %q: %w", key, err)
	}
	return nil
}

func (b *FileBackend) PutJson(ctx context.Context, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal failed %q: %w", key, err)
	}
	return b.PutRaw(ctx, key, data)
}

func (b *FileBackend) List(ctx context.Context, prefix string) ([]string, error) {
	dir := filepath.Join(b.root, filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage list %q: %w", prefix, err)
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

func (b *FileBackend) Delete(ctx context.Context, key string) error {
	err := os.Remove(b.keyPath(key))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("storage delete %q: %w", key, err)
	}
	return nil
}

func (b *FileBackend) DeletePrefix(ctx context.Context, prefix string) error {
	path := filepath.Join(b.root, filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
	err := os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("storage delete-prefix %q: %w", prefix, err)
	}
	return nil
}
