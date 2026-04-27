package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/storage"
	"github.com/mwantia/forge/internal/service/template"
)

// memoryStore is the narrow storage contract MemoryService depends on.
// Two implementations: fileMemoryStore (built-in, file-backed) and
// pluginMemoryStore (forwards to a bound MemoryPlugin).
type memoryStore interface {
	Store(ctx context.Context, namespace, content string, metadata map[string]any) (*sdkplugins.MemoryResource, error)
	Retrieve(ctx context.Context, namespace, query string, limit int, filter map[string]any) ([]*sdkplugins.MemoryResource, error)
}

// fileMemoryStore persists resources under memory/<namespace>/<id>.json using
// the injected StorageBackend. Retrieve uses a naive case-insensitive
// substring match — good enough for the built-in fallback. Plug in a real
// memory plugin (OpenViking) for semantic search.
type fileMemoryStore struct {
	storage storage.StorageBackend
}

type fileResource struct {
	ID        string         `json:"id"`
	Namespace string         `json:"namespace"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func memoryResourceKey(namespace, id string) string {
	return "memory/" + namespace + "/" + id + ".json"
}

func memoryNamespacePrefix(namespace string) string {
	return "memory/" + namespace + "/"
}

func (s *fileMemoryStore) Store(ctx context.Context, namespace, content string, metadata map[string]any) (*sdkplugins.MemoryResource, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	res := fileResource{
		ID:        template.GenerateNewID(),
		Namespace: namespace,
		Content:   content,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}
	if err := s.storage.WriteJson(ctx, memoryResourceKey(namespace, res.ID), res); err != nil {
		return nil, fmt.Errorf("failed to write memory resource: %w", err)
	}
	return &sdkplugins.MemoryResource{
		ID:       res.ID,
		Content:  res.Content,
		Metadata: res.Metadata,
	}, nil
}

func (s *fileMemoryStore) Retrieve(ctx context.Context, namespace, query string, limit int, filter map[string]any) ([]*sdkplugins.MemoryResource, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	entries, err := s.storage.ListEntry(ctx, memoryNamespacePrefix(namespace))
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}

	q := strings.ToLower(strings.TrimSpace(query))
	scored := make([]*sdkplugins.MemoryResource, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry, "/") {
			continue
		}
		var r fileResource
		if err := s.storage.ReadJson(ctx, entry, &r); err != nil {
			continue
		}
		if !metadataMatches(r.Metadata, filter) {
			continue
		}
		score := substringScore(r.Content, q)
		if q != "" && score == 0 {
			continue
		}
		scored = append(scored, &sdkplugins.MemoryResource{
			ID:       r.ID,
			Content:  r.Content,
			Score:    score,
			Metadata: r.Metadata,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

func substringScore(content, query string) float64 {
	if query == "" {
		return 1
	}
	c := strings.ToLower(content)
	n := strings.Count(c, query)
	if n == 0 {
		return 0
	}
	return float64(n)
}

func metadataMatches(meta, filter map[string]any) bool {
	if len(filter) == 0 {
		return true
	}
	for k, want := range filter {
		if got, ok := meta[k]; !ok || got != want {
			return false
		}
	}
	return true
}

// pluginMemoryStore delegates to a bound MemoryPlugin.
type pluginMemoryStore struct {
	plugin sdkplugins.MemoryPlugin
}

func (s *pluginMemoryStore) Store(ctx context.Context, namespace, content string, metadata map[string]any) (*sdkplugins.MemoryResource, error) {
	return s.plugin.StoreResource(ctx, namespace, content, metadata)
}

func (s *pluginMemoryStore) Retrieve(ctx context.Context, namespace, query string, limit int, filter map[string]any) ([]*sdkplugins.MemoryResource, error) {
	return s.plugin.RetrieveResource(ctx, namespace, query, limit, filter)
}
