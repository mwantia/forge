package resource

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

// resourceStore is the narrow storage contract ResourceService depends on.
// Two implementations: fileResourceStore (built-in, file-backed) and
// pluginResourceStore (forwards to a bound ResourcePlugin).
type resourceStore interface {
	Store(ctx context.Context, namespace, content string, metadata map[string]any) (*sdkplugins.Resource, error)
	Recall(ctx context.Context, namespace, query string, limit int, filter map[string]any) ([]*sdkplugins.Resource, error)
	Forget(ctx context.Context, namespace, id string) error
	List(ctx context.Context, namespace string) ([]*sdkplugins.Resource, error)
	Get(ctx context.Context, namespace, id string) (*sdkplugins.Resource, error)
}

// fileResourceStore persists resources under resources/<namespace>/<id>.json
// using the injected StorageBackend. Retrieve uses a naive case-insensitive
// substring match — good enough for the built-in fallback. Plug in a real
// resource plugin (OpenViking) for semantic search.
type fileResourceStore struct {
	storage storage.StorageBackend
}

type fileResource struct {
	ID        string         `json:"id"`
	Namespace string         `json:"namespace"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func resourceKey(namespace, id string) string {
	return "resources/" + namespace + "/" + id + ".json"
}

func resourceNamespacePrefix(namespace string) string {
	return "resources/" + namespace + "/"
}

func (s *fileResourceStore) Store(ctx context.Context, namespace, content string, metadata map[string]any) (*sdkplugins.Resource, error) {
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
	if err := s.storage.WriteJson(ctx, resourceKey(namespace, res.ID), res); err != nil {
		return nil, fmt.Errorf("failed to write resource: %w", err)
	}
	return &sdkplugins.Resource{
		ID:       res.ID,
		Content:  res.Content,
		Metadata: res.Metadata,
	}, nil
}

func (s *fileResourceStore) Recall(ctx context.Context, namespace, query string, limit int, filter map[string]any) ([]*sdkplugins.Resource, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	prefix := resourceNamespacePrefix(namespace)
	entries, err := s.storage.ListEntry(ctx, prefix)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}

	q := strings.ToLower(strings.TrimSpace(query))
	scored := make([]*sdkplugins.Resource, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry, "/") || !strings.HasSuffix(entry, ".json") {
			continue
		}
		var r fileResource
		if err := s.storage.ReadJson(ctx, prefix+entry, &r); err != nil {
			continue
		}
		if !metadataMatches(r.Metadata, filter) {
			continue
		}
		score := substringScore(r.Content, q)
		if q != "" && score == 0 {
			continue
		}
		scored = append(scored, &sdkplugins.Resource{
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

func (s *fileResourceStore) List(ctx context.Context, namespace string) ([]*sdkplugins.Resource, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	prefix := resourceNamespacePrefix(namespace)
	entries, err := s.storage.ListEntry(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]*sdkplugins.Resource, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry, "/") || !strings.HasSuffix(entry, ".json") {
			continue
		}
		var r fileResource
		if err := s.storage.ReadJson(ctx, prefix+entry, &r); err != nil {
			continue
		}
		out = append(out, &sdkplugins.Resource{
			ID:       r.ID,
			Content:  r.Content,
			Metadata: r.Metadata,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *fileResourceStore) Forget(ctx context.Context, namespace, id string) error {
	if namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return s.storage.DeleteEntry(ctx, resourceKey(namespace, id))
}

func (s *fileResourceStore) Get(ctx context.Context, namespace, id string) (*sdkplugins.Resource, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	var r fileResource
	if err := s.storage.ReadJson(ctx, resourceKey(namespace, id), &r); err != nil {
		return nil, err
	}
	return &sdkplugins.Resource{
		ID:       r.ID,
		Content:  r.Content,
		Metadata: r.Metadata,
	}, nil
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

// pluginResourceStore delegates to a bound ResourcePlugin.
type pluginResourceStore struct {
	plugin sdkplugins.ResourcePlugin
}

func (s *pluginResourceStore) Store(ctx context.Context, namespace, content string, metadata map[string]any) (*sdkplugins.Resource, error) {
	return s.plugin.Store(ctx, namespace, content, metadata)
}

func (s *pluginResourceStore) Recall(ctx context.Context, namespace, query string, limit int, filter map[string]any) ([]*sdkplugins.Resource, error) {
	return s.plugin.Recall(ctx, namespace, query, limit, filter)
}

func (s *pluginResourceStore) Forget(ctx context.Context, namespace, id string) error {
	return s.plugin.Forget(ctx, namespace, id)
}

func (s *pluginResourceStore) List(ctx context.Context, namespace string) ([]*sdkplugins.Resource, error) {
	return nil, fmt.Errorf("list is not supported by the active resource plugin")
}

func (s *pluginResourceStore) Get(ctx context.Context, namespace, id string) (*sdkplugins.Resource, error) {
	return nil, fmt.Errorf("get is not supported by the active resource plugin")
}
