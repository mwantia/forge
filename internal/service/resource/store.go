package resource

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session/dag"
	"github.com/mwantia/forge/internal/service/storage"
)

// resourceStore is the narrow storage contract ResourceService depends on.
// Current implementation: dagResourceStore (built-in, DAG-backed, sole backend).
type resourceStore interface {
	Store(ctx context.Context, path, name, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error)
	Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error)
	Forget(ctx context.Context, path, name string) error
	List(ctx context.Context, path string) ([]*sdkplugins.Resource, error)
	Get(ctx context.Context, path, name string) (*sdkplugins.Resource, error)
	UpdateMeta(ctx context.Context, path, name string, tags []string, metadata map[string]any) error
	History(ctx context.Context, path, name string) ([]*ResourceRevision, error)
	GetAt(ctx context.Context, hash string) (*dag.ResourceObj, error)
	Revert(ctx context.Context, path, name, toHash string) error
}

// ResourceRevision is one entry in a resource's parent-chain history.
type ResourceRevision struct {
	Hash      string         `json:"hash"`
	CreatedAt time.Time      `json:"created_at"`
	Tags      []string       `json:"tags,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	IndexedAt *time.Time     `json:"indexed_at,omitempty"`
	IndexedBy string         `json:"indexed_by,omitempty"`
}

// dagResourceStore persists resources on the shared content-addressed DAG.
// Objects land in the global object pool (objects/<aa>/<rest-of-hash>).
// Refs live at resources/<namespace>/refs/<name>. Log entries (ResourceMeta
// sidecars) live at resources/<namespace>/log/<unix_nano>_<hash>.json.
type dagResourceStore struct {
	storage storage.StorageBackend // for WriteJson/ReadJson on log entries
	objects *dag.ObjectStore
	refs    *dag.RefStore
}

func newDagResourceStore(s storage.StorageBackend) *dagResourceStore {
	return &dagResourceStore{
		storage: s,
		objects: dag.NewObjectStore(s),
		refs:    dag.NewRefStore(s),
	}
}

// namespace converts a path string into the DAG namespace key.
func namespace(path string) string {
	return strings.Trim(path, "/")
}

func (s *dagResourceStore) Store(ctx context.Context, path, name, content string, tags []string, metadata map[string]any) (*sdkplugins.Resource, error) {
	ns := namespace(path)
	now := time.Now()

	// Read current tip to set ParentHash and for CAS.
	prevHash := ""
	if name != "" {
		h, err := s.refs.ReadKey(ctx, dag.ResourceRefKey(ns, name))
		if err != nil && !errors.Is(err, dag.ErrNotFound) {
			return nil, fmt.Errorf("read current tip: %w", err)
		}
		prevHash = h
	}

	obj := &dag.ResourceObj{
		ContentType: "text",
		Content:     content,
		ParentHash:  prevHash,
	}

	hash, err := s.objects.PutResource(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("store resource object: %w", err)
	}

	// If no name was given, derive one from the content hash.
	if name == "" {
		name = hash[:16]
		// Now that we have the name, re-read the current tip.
		if h, err := s.refs.ReadKey(ctx, dag.ResourceRefKey(ns, name)); err == nil {
			prevHash = h
			obj.ParentHash = prevHash
			if hash, err = s.objects.PutResource(ctx, obj); err != nil {
				return nil, fmt.Errorf("store resource object with parent: %w", err)
			}
		}
	}

	meta := &dag.ResourceMeta{
		Hash:      hash,
		Namespace: ns,
		Name:      name,
		Tags:      tags,
		Metadata:  metadata,
		CreatedAt: now,
	}
	if err := s.writeResourceLogEntry(ctx, ns, now, hash, meta); err != nil {
		return nil, fmt.Errorf("write resource log: %w", err)
	}

	if err := s.refs.CASKey(ctx, dag.ResourceRefKey(ns, name), prevHash, hash); err != nil {
		return nil, fmt.Errorf("advance resource ref: %w", err)
	}

	return &sdkplugins.Resource{
		ID:        name,
		Path:      path,
		Content:   content,
		Tags:      tags,
		Metadata:  metadata,
		CreatedAt: now,
	}, nil
}

func (s *dagResourceStore) Get(ctx context.Context, path, name string) (*sdkplugins.Resource, error) {
	ns := namespace(path)

	hash, err := s.refs.ReadKey(ctx, dag.ResourceRefKey(ns, name))
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return nil, fmt.Errorf("resource %q not found in %s", name, path)
		}
		return nil, err
	}

	obj, err := s.objects.GetResource(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("load resource object %s: %w", hash, err)
	}

	meta := s.loadRecentMeta(ctx, ns, hash)
	return toSDKResource(path, name, hash, obj, meta), nil
}

func (s *dagResourceStore) Forget(ctx context.Context, path, name string) error {
	ns := namespace(path)
	return s.refs.DeleteKey(ctx, dag.ResourceRefKey(ns, name))
}

func (s *dagResourceStore) List(ctx context.Context, path string) ([]*sdkplugins.Resource, error) {
	ns := namespace(path)

	nameToHash, err := s.refs.ListKeys(ctx, dag.ResourceRefsPrefix(ns))
	if err != nil {
		return nil, err
	}

	out := make([]*sdkplugins.Resource, 0, len(nameToHash))
	for name, hash := range nameToHash {
		obj, err := s.objects.GetResource(ctx, hash)
		if err != nil {
			continue
		}
		meta := s.loadRecentMeta(ctx, ns, hash)
		out = append(out, toSDKResource(path, name, hash, obj, meta))
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *dagResourceStore) Recall(ctx context.Context, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, error) {
	if q.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 5
	}

	hasWildcard := strings.ContainsAny(q.Path, "*?[")
	queryLower := strings.ToLower(strings.TrimSpace(q.Query))

	var namespaces []string
	if hasWildcard {
		basePath := globBase(q.Path)
		ns := namespace(basePath)
		all, err := s.listNamespaces(ctx, ns)
		if err != nil {
			return nil, err
		}
		for _, n := range all {
			path := "/" + n
			matched, _ := doublestar.Match(q.Path, path)
			if matched {
				namespaces = append(namespaces, n)
			}
		}
	} else {
		namespaces = []string{namespace(q.Path)}
	}

	var scored []*sdkplugins.Resource
	for _, ns := range namespaces {
		path := "/" + ns
		nameToHash, err := s.refs.ListKeys(ctx, dag.ResourceRefsPrefix(ns))
		if err != nil {
			continue
		}
		for name, hash := range nameToHash {
			obj, err := s.objects.GetResource(ctx, hash)
			if err != nil {
				continue
			}
			meta := s.loadRecentMeta(ctx, ns, hash)

			if meta != nil {
				if !tagsMatch(meta.Tags, q.Tags) {
					continue
				}
				if !predicatesMatch(meta.Metadata, q.Filter) {
					continue
				}
				if !q.CreatedAfter.IsZero() && meta.CreatedAt.Before(q.CreatedAfter) {
					continue
				}
				if !q.CreatedBefore.IsZero() && meta.CreatedAt.After(q.CreatedBefore) {
					continue
				}
			}

			score := substringScore(obj.Content, queryLower)
			if queryLower != "" && score == 0 {
				continue
			}

			r := toSDKResource(path, name, hash, obj, meta)
			r.Score = score
			scored = append(scored, r)
		}
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

// listNamespaces returns all resource namespaces that are sub-paths of root.
// root="" lists everything; root="sessions" lists all namespaces under sessions/.
func (s *dagResourceStore) listNamespaces(ctx context.Context, root string) ([]string, error) {
	prefix := "resources/"
	if root != "" {
		prefix = "resources/" + root + "/"
	}

	allRefs, err := s.refs.ListKeys(ctx, prefix)
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	var result []string
	for key := range allRefs {
		// key is relative to prefix, e.g.:
		//   root=""       → "sessions/abc/refs/my-note"   → ns "sessions/abc"
		//   root="sessions" → "abc/refs/my-note"          → ns "sessions/abc"
		if i := strings.Index(key, "/refs/"); i >= 0 {
			ns := key[:i]
			if root != "" {
				ns = root + "/" + ns
			}
			if _, dup := seen[ns]; !dup {
				seen[ns] = struct{}{}
				result = append(result, ns)
			}
		}
	}
	return result, nil
}

func (s *dagResourceStore) writeResourceLogEntry(ctx context.Context, ns string, createdAt time.Time, hash string, meta *dag.ResourceMeta) error {
	key := dag.ResourceLogKey(ns, createdAt, hash)
	return s.storage.WriteJson(ctx, key, meta)
}

// loadRecentMeta returns the most-recent ResourceMeta log entry for hash
// in namespace ns. Returns nil if not found (non-fatal).
func (s *dagResourceStore) loadRecentMeta(ctx context.Context, ns, hash string) *dag.ResourceMeta {
	prefix := dag.ResourceLogPrefix(ns)
	entries, err := s.refs.Backend.ListEntry(ctx, prefix)
	if err != nil {
		return nil
	}

	suffix := "_" + hash + ".json"
	var best string
	for _, e := range entries {
		if strings.HasSuffix(e, suffix) {
			if e > best {
				best = e
			}
		}
	}

	if best == "" {
		return nil
	}

	var meta dag.ResourceMeta
	if err := s.storage.ReadJson(ctx, prefix+best, &meta); err != nil {
		return nil
	}
	return &meta
}

func (s *dagResourceStore) UpdateMeta(ctx context.Context, path, name string, tags []string, metadata map[string]any) error {
	ns := namespace(path)

	hash, err := s.refs.ReadKey(ctx, dag.ResourceRefKey(ns, name))
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return fmt.Errorf("resource %q not found in %s", name, path)
		}
		return err
	}

	existing := s.loadRecentMeta(ctx, ns, hash)
	now := time.Now()

	meta := &dag.ResourceMeta{
		Hash:      hash,
		Namespace: ns,
		Name:      name,
		Tags:      tags,
		Metadata:  metadata,
		CreatedAt: now,
	}
	if existing != nil {
		meta.IndexedAt = existing.IndexedAt
		meta.IndexedBy = existing.IndexedBy
	}

	return s.writeResourceLogEntry(ctx, ns, now, hash, meta)
}

// History walks the ParentHash chain from the current tip, joining each
// revision with its most-recent log entry for metadata.
func (s *dagResourceStore) History(ctx context.Context, path, name string) ([]*ResourceRevision, error) {
	ns := namespace(path)

	hash, err := s.refs.ReadKey(ctx, dag.ResourceRefKey(ns, name))
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return nil, fmt.Errorf("resource %q not found in %s", name, path)
		}
		return nil, err
	}

	var out []*ResourceRevision
	for hash != "" {
		obj, err := s.objects.GetResource(ctx, hash)
		if err != nil {
			break
		}
		rev := &ResourceRevision{Hash: hash}
		if m := s.loadRecentMeta(ctx, ns, hash); m != nil {
			rev.CreatedAt = m.CreatedAt
			rev.Tags = m.Tags
			rev.Metadata = m.Metadata
			rev.IndexedAt = m.IndexedAt
			rev.IndexedBy = m.IndexedBy
		}
		out = append(out, rev)
		hash = obj.ParentHash
	}
	return out, nil
}

// GetAt fetches any historical ResourceObj directly by content hash.
func (s *dagResourceStore) GetAt(ctx context.Context, hash string) (*dag.ResourceObj, error) {
	return s.objects.GetResource(ctx, hash)
}

// Revert CAS-moves the named ref to toHash and writes a new log entry.
func (s *dagResourceStore) Revert(ctx context.Context, path, name, toHash string) error {
	ns := namespace(path)

	currentHash, err := s.refs.ReadKey(ctx, dag.ResourceRefKey(ns, name))
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return fmt.Errorf("resource %q not found in %s", name, path)
		}
		return err
	}

	// Verify the target exists in the object pool.
	if _, err := s.objects.GetResource(ctx, toHash); err != nil {
		return fmt.Errorf("revert target %s not found: %w", toHash, err)
	}

	if err := s.refs.CASKey(ctx, dag.ResourceRefKey(ns, name), currentHash, toHash); err != nil {
		return fmt.Errorf("revert CAS failed: %w", err)
	}

	now := time.Now()
	meta := &dag.ResourceMeta{
		Hash:      toHash,
		Namespace: ns,
		Name:      name,
		CreatedAt: now,
	}
	// Preserve existing indexing state if available.
	if existing := s.loadRecentMeta(ctx, ns, toHash); existing != nil {
		meta.Tags = existing.Tags
		meta.Metadata = existing.Metadata
		meta.IndexedAt = existing.IndexedAt
		meta.IndexedBy = existing.IndexedBy
	}
	return s.writeResourceLogEntry(ctx, ns, now, toHash, meta)
}

func toSDKResource(path, name, hash string, obj *dag.ResourceObj, meta *dag.ResourceMeta) *sdkplugins.Resource {
	r := &sdkplugins.Resource{
		ID:      name,
		Path:    path,
		Content: obj.Content,
	}
	if meta != nil {
		r.Tags = meta.Tags
		r.Metadata = meta.Metadata
		r.CreatedAt = meta.CreatedAt
	}
	return r
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

// globBase returns the deepest path segment before the first wildcard char.
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


