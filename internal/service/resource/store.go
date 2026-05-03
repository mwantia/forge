package resource

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/storage"
	"github.com/mwantia/forge/internal/service/template"
)

// resourceStore is the narrow storage contract ResourceService depends on.
// Two implementations: fileResourceStore (built-in, file-backed) and
// pluginResourceStore (forwards to a bound ResourcePlugin).
type resourceStore interface {
	Store(ctx context.Context, path, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error)
	Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error)
	Forget(ctx context.Context, path, id string) error
	List(ctx context.Context, path string) ([]*sdkplugins.Resource, error)
	Get(ctx context.Context, path, id string) (*sdkplugins.Resource, error)
}

// fileResourceStore persists resources under resources/<path>/<id>.json
// using the injected StorageBackend. Recall uses case-insensitive substring
// matching with path glob filtering — good enough for the built-in fallback.
type fileResourceStore struct {
	storage storage.StorageBackend
}

type fileResource struct {
	ID        string         `json:"id"`
	Path      string         `json:"path"`
	Content   string         `json:"content"`
	Tags      []string       `json:"tags,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func pathToKey(path, id string) string {
	p := strings.Trim(path, "/")
	return "resources/" + p + "/" + id + ".json"
}

func pathToPrefix(path string) string {
	p := strings.Trim(path, "/")
	if p == "" {
		return "resources/"
	}
	return "resources/" + p + "/"
}

// storageKeyToPathAndID reverses pathToKey: "resources/sessions/abc/id.json"
// → ("/sessions/abc", "id").
func storageKeyToPathAndID(key string) (path, id string) {
	trimmed := strings.TrimPrefix(key, "resources/")
	last := strings.LastIndex(trimmed, "/")
	if last < 0 {
		return "/", strings.TrimSuffix(trimmed, ".json")
	}
	return "/" + trimmed[:last], strings.TrimSuffix(trimmed[last+1:], ".json")
}

// globBase returns the deepest path segment before the first wildcard char,
// used to anchor the recursive listing prefix.
func globBase(pattern string) string {
	idx := strings.IndexAny(pattern, "*?[")
	if idx < 0 {
		return pattern
	}
	slash := strings.LastIndex(pattern[:idx], "/")
	if slash < 0 {
		return "/"
	}
	return pattern[:slash+1]
}

// listAll recursively lists all .json file storage keys under prefix.
func (s *fileResourceStore) listAll(ctx context.Context, prefix string) ([]string, error) {
	entries, err := s.storage.ListEntry(ctx, prefix)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if strings.HasSuffix(e, "/") {
			sub, err := s.listAll(ctx, prefix+e)
			if err != nil {
				return nil, err
			}
			result = append(result, sub...)
		} else if strings.HasSuffix(e, ".json") {
			result = append(result, prefix+e)
		}
	}
	return result, nil
}

func (s *fileResourceStore) Store(ctx context.Context, path, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error) {
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	res := fileResource{
		ID:        template.GenerateNewID(),
		Path:      path,
		Content:   content,
		Tags:      tags,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}
	if err := s.storage.WriteJson(ctx, pathToKey(path, res.ID), res); err != nil {
		return nil, fmt.Errorf("failed to write resource: %w", err)
	}
	return &sdkplugins.Resource{
		ID:        res.ID,
		Path:      res.Path,
		Content:   res.Content,
		Tags:      res.Tags,
		Metadata:  res.Metadata,
		CreatedAt: res.CreatedAt,
	}, nil
}

func (s *fileResourceStore) Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error) {
	if q.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 5
	}

	hasWildcard := strings.ContainsAny(q.Path, "*?[")
	basePath := q.Path
	if hasWildcard {
		basePath = globBase(q.Path)
	}

	keys, err := s.listAll(ctx, pathToPrefix(basePath))
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(strings.TrimSpace(q.Query))

	scored := make([]*sdkplugins.Resource, 0, len(keys))
	for _, key := range keys {
		rpath, _ := storageKeyToPathAndID(key)

		if hasWildcard {
			matched, _ := doublestar.Match(q.Path, rpath)
			if !matched {
				continue
			}
		} else if rpath != q.Path {
			continue
		}

		var r fileResource
		if err := s.storage.ReadJson(ctx, key, &r); err != nil {
			continue
		}

		if !tagsMatch(r.Tags, q.Tags) {
			continue
		}
		if !predicatesMatch(r.Metadata, q.Filter) {
			continue
		}
		if !q.CreatedAfter.IsZero() && r.CreatedAt.Before(q.CreatedAfter) {
			continue
		}
		if !q.CreatedBefore.IsZero() && r.CreatedAt.After(q.CreatedBefore) {
			continue
		}

		score := substringScore(r.Content, queryLower)
		if queryLower != "" && score == 0 {
			continue
		}

		scored = append(scored, &sdkplugins.Resource{
			ID:        r.ID,
			Path:      r.Path,
			Content:   r.Content,
			Tags:      r.Tags,
			Score:     score,
			Metadata:  r.Metadata,
			CreatedAt: r.CreatedAt,
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

func (s *fileResourceStore) List(ctx context.Context, path string) ([]*sdkplugins.Resource, error) {
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	prefix := pathToPrefix(path)
	entries, err := s.storage.ListEntry(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]*sdkplugins.Resource, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e, "/") || !strings.HasSuffix(e, ".json") {
			continue
		}
		var r fileResource
		if err := s.storage.ReadJson(ctx, prefix+e, &r); err != nil {
			continue
		}
		out = append(out, &sdkplugins.Resource{
			ID:        r.ID,
			Path:      r.Path,
			Content:   r.Content,
			Tags:      r.Tags,
			Metadata:  r.Metadata,
			CreatedAt: r.CreatedAt,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *fileResourceStore) Forget(ctx context.Context, path, id string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return s.storage.DeleteEntry(ctx, pathToKey(path, id))
}

func (s *fileResourceStore) Get(ctx context.Context, path, id string) (*sdkplugins.Resource, error) {
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	var r fileResource
	if err := s.storage.ReadJson(ctx, pathToKey(path, id), &r); err != nil {
		return nil, err
	}
	return &sdkplugins.Resource{
		ID:        r.ID,
		Path:      r.Path,
		Content:   r.Content,
		Tags:      r.Tags,
		Metadata:  r.Metadata,
		CreatedAt: r.CreatedAt,
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

func tagsMatch(resourceTags, queryTags []string) bool {
	if len(queryTags) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(resourceTags))
	for _, t := range resourceTags {
		set[t] = struct{}{}
	}
	for _, t := range queryTags {
		if _, ok := set[t]; !ok {
			return false
		}
	}
	return true
}

func predicatesMatch(meta map[string]any, preds []sdkplugins.FilterPredicate) bool {
	for _, p := range preds {
		v, ok := meta[p.Key]
		if !ok {
			return false
		}
		if !predicateMatches(v, p.Op, p.Value) {
			return false
		}
	}
	return true
}

func predicateMatches(got any, op sdkplugins.FilterOp, want any) bool {
	gs := fmt.Sprintf("%v", got)
	ws := fmt.Sprintf("%v", want)
	switch op {
	case sdkplugins.FilterOpEq:
		return gs == ws
	case sdkplugins.FilterOpPrefix:
		return strings.HasPrefix(gs, ws)
	case sdkplugins.FilterOpContains:
		return strings.Contains(gs, ws)
	case sdkplugins.FilterOpGte:
		return gs >= ws
	case sdkplugins.FilterOpLte:
		return gs <= ws
	default:
		return false
	}
}

// pluginResourceStore delegates to a bound ResourcePlugin.
type pluginResourceStore struct {
	plugin sdkplugins.ResourcePlugin
}

func (s *pluginResourceStore) Store(ctx context.Context, path, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error) {
	return s.plugin.Store(ctx, path, content, tags, metadata)
}

func (s *pluginResourceStore) Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error) {
	return s.plugin.Recall(ctx, q)
}

func (s *pluginResourceStore) Forget(ctx context.Context, path, id string) error {
	return s.plugin.Forget(ctx, path, id)
}

func (s *pluginResourceStore) List(ctx context.Context, path string) ([]*sdkplugins.Resource, error) {
	return nil, fmt.Errorf("list is not supported by the active resource plugin")
}

func (s *pluginResourceStore) Get(ctx context.Context, path, id string) (*sdkplugins.Resource, error) {
	return nil, fmt.Errorf("get is not supported by the active resource plugin")
}
