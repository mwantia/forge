package resource

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	domresource "github.com/mwantia/forge/internal/domain/resource"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
	infrastorage "github.com/mwantia/forge/internal/infrastructure/storage"
)

type ResourceRevision = domresource.ResourceRevision

// resourceStore is the narrow storage contract ResourceService depends on.
type resourceStore interface {
	Store(ctx context.Context, content, commitMessage string, meta domresource.ResourceMeta) (*domresource.Resource, error)
	Commit(ctx context.Context, id, content, commitMessage string) (*domresource.Resource, error)
	Recall(ctx context.Context, q domresource.RecallQuery) ([]*domresource.Resource, error)
	Forget(ctx context.Context, id string) error
	List(ctx context.Context, filter []domresource.FilterPredicate) ([]*domresource.Resource, error)
	Get(ctx context.Context, id string) (*domresource.Resource, error)
	UpdateMeta(ctx context.Context, id string, meta domresource.ResourceMeta) error
	History(ctx context.Context, id string) ([]*ResourceRevision, error)
	GetAt(ctx context.Context, hash string) (*dag.ResourceObj, error)
	Revert(ctx context.Context, id, toHash string) error
}

// dagResourceStore persists resources on the shared content-addressed DAG.
// Objects land in the global object pool (objects/<aa>/<rest-of-hash>).
// Refs live at resources/refs/<id>. Mutable sidecars at resources/meta/<id>.json.
type dagResourceStore struct {
	storage infrastorage.StorageBackend
	objects *dag.ObjectStore
	refs    *dag.RefStore
}

func newDagResourceStore(s infrastorage.StorageBackend) *dagResourceStore {
	return &dagResourceStore{
		storage: s,
		objects: dag.NewObjectStore(s),
		refs:    dag.NewRefStore(s),
	}
}

func (s *dagResourceStore) Store(ctx context.Context, content, commitMessage string, meta domresource.ResourceMeta) (*domresource.Resource, error) {
	id := uuid.New().String()
	now := time.Now()

	obj := &dag.ResourceObj{
		ContentType:   "text",
		Content:       content,
		CommitMessage: commitMessage,
		CommittedAt:   now,
	}

	hash, err := s.objects.PutResource(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("store resource object: %w", err)
	}

	meta.CreatedAt = now
	meta.UpdatedAt = now

	dagMeta := toDagMeta(id, hash, meta, nil, "")
	if err := s.storage.WriteJson(ctx, dag.ResourceMetaKey(id), dagMeta); err != nil {
		return nil, fmt.Errorf("write resource meta: %w", err)
	}

	if err := s.refs.CASKey(ctx, dag.ResourceRefKey(id), "", hash); err != nil {
		return nil, fmt.Errorf("advance resource ref: %w", err)
	}

	return &domresource.Resource{
		ID:      id,
		Content: content,
		Meta:    meta,
	}, nil
}

func (s *dagResourceStore) Commit(ctx context.Context, id, content, commitMessage string) (*domresource.Resource, error) {
	currentHash, err := s.refs.ReadKey(ctx, dag.ResourceRefKey(id))
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return nil, fmt.Errorf("resource %q not found", id)
		}
		return nil, err
	}

	obj := &dag.ResourceObj{
		ContentType:   "text",
		Content:       content,
		CommitMessage: commitMessage,
		CommittedAt:   time.Now(),
		ParentHash:    currentHash,
	}

	newHash, err := s.objects.PutResource(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("commit resource object: %w", err)
	}

	if err := s.refs.CASKey(ctx, dag.ResourceRefKey(id), currentHash, newHash); err != nil {
		return nil, fmt.Errorf("advance resource ref: %w", err)
	}

	dagMeta, err := s.loadMeta(ctx, id)
	if err != nil {
		return nil, err
	}
	dagMeta.Hash = newHash
	dagMeta.UpdatedAt = time.Now()
	if err := s.storage.WriteJson(ctx, dag.ResourceMetaKey(id), dagMeta); err != nil {
		return nil, fmt.Errorf("update resource meta: %w", err)
	}

	return toResource(id, obj, dagMeta), nil
}

func (s *dagResourceStore) Get(ctx context.Context, id string) (*domresource.Resource, error) {
	hash, err := s.refs.ReadKey(ctx, dag.ResourceRefKey(id))
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return nil, fmt.Errorf("resource %q not found", id)
		}
		return nil, err
	}

	obj, err := s.objects.GetResource(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("load resource object %s: %w", hash, err)
	}

	dagMeta, err := s.loadMeta(ctx, id)
	if err != nil {
		return nil, err
	}

	return toResource(id, obj, dagMeta), nil
}

func (s *dagResourceStore) Forget(ctx context.Context, id string) error {
	if err := s.refs.DeleteKey(ctx, dag.ResourceRefKey(id)); err != nil {
		return err
	}
	// Best-effort removal of the sidecar; not fatal if missing.
	_ = s.storage.DeleteEntry(ctx, dag.ResourceMetaKey(id))
	return nil
}

func (s *dagResourceStore) List(ctx context.Context, filter []domresource.FilterPredicate) ([]*domresource.Resource, error) {
	idToHash, err := s.refs.ListKeys(ctx, dag.ResourceRefsPrefix())
	if err != nil {
		return nil, err
	}

	out := make([]*domresource.Resource, 0, len(idToHash))
	for id := range idToHash {
		dagMeta, err := s.loadMeta(ctx, id)
		if err != nil {
			continue
		}
		if !metaMatchesFilter(dagMeta, filter) {
			continue
		}
		obj, err := s.objects.GetResource(ctx, dagMeta.Hash)
		if err != nil {
			continue
		}
		out = append(out, toResource(id, obj, dagMeta))
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Meta.CreatedAt.Before(out[j].Meta.CreatedAt)
	})
	return out, nil
}

func (s *dagResourceStore) Recall(ctx context.Context, q domresource.RecallQuery) ([]*domresource.Resource, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 5
	}

	queryLower := strings.ToLower(strings.TrimSpace(q.Query))

	idToHash, err := s.refs.ListKeys(ctx, dag.ResourceRefsPrefix())
	if err != nil {
		return nil, err
	}

	var scored []*domresource.Resource
	for id := range idToHash {
		dagMeta, err := s.loadMeta(ctx, id)
		if err != nil {
			continue
		}

		if !tagsMatch(dagMeta.Tags, q.Tags) {
			continue
		}
		if !metaMatchesFilter(dagMeta, q.Filter) {
			continue
		}
		if !q.CreatedAfter.IsZero() && dagMeta.CreatedAt.Before(q.CreatedAfter) {
			continue
		}
		if !q.CreatedBefore.IsZero() && dagMeta.CreatedAt.After(q.CreatedBefore) {
			continue
		}

		obj, err := s.objects.GetResource(ctx, dagMeta.Hash)
		if err != nil {
			continue
		}

		score := substringScore(obj.Content, queryLower)
		if queryLower != "" && score == 0 {
			continue
		}

		r := toResource(id, obj, dagMeta)
		r.Score = score
		scored = append(scored, r)
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

func (s *dagResourceStore) UpdateMeta(ctx context.Context, id string, meta domresource.ResourceMeta) error {
	existing, err := s.loadMeta(ctx, id)
	if err != nil {
		return err
	}

	updated := toDagMeta(id, existing.Hash, meta, existing.IndexedAt, existing.IndexedBy)
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = time.Now()
	return s.storage.WriteJson(ctx, dag.ResourceMetaKey(id), updated)
}

func (s *dagResourceStore) History(ctx context.Context, id string) ([]*ResourceRevision, error) {
	hash, err := s.refs.ReadKey(ctx, dag.ResourceRefKey(id))
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return nil, fmt.Errorf("resource %q not found", id)
		}
		return nil, err
	}

	dagMeta, _ := s.loadMeta(ctx, id)

	var out []*ResourceRevision
	for hash != "" {
		obj, err := s.objects.GetResource(ctx, hash)
		if err != nil {
			break
		}
		rev := &ResourceRevision{
			Hash:          hash,
			CommitMessage: obj.CommitMessage,
			CommittedAt:   obj.CommittedAt,
		}
		if dagMeta != nil {
			rev.IndexedAt = dagMeta.IndexedAt
			rev.IndexedBy = dagMeta.IndexedBy
		}
		out = append(out, rev)
		hash = obj.ParentHash
	}
	return out, nil
}

func (s *dagResourceStore) GetAt(ctx context.Context, hash string) (*dag.ResourceObj, error) {
	return s.objects.GetResource(ctx, hash)
}

func (s *dagResourceStore) Revert(ctx context.Context, id, toHash string) error {
	currentHash, err := s.refs.ReadKey(ctx, dag.ResourceRefKey(id))
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return fmt.Errorf("resource %q not found", id)
		}
		return err
	}

	if _, err := s.objects.GetResource(ctx, toHash); err != nil {
		return fmt.Errorf("revert target %s not found: %w", toHash, err)
	}

	if err := s.refs.CASKey(ctx, dag.ResourceRefKey(id), currentHash, toHash); err != nil {
		return fmt.Errorf("revert CAS failed: %w", err)
	}

	existing, err := s.loadMeta(ctx, id)
	if err != nil {
		return err
	}
	existing.Hash = toHash
	existing.UpdatedAt = time.Now()
	return s.storage.WriteJson(ctx, dag.ResourceMetaKey(id), existing)
}

func (s *dagResourceStore) loadMeta(ctx context.Context, id string) (*dag.ResourceMeta, error) {
	var meta dag.ResourceMeta
	if err := s.storage.ReadJson(ctx, dag.ResourceMetaKey(id), &meta); err != nil {
		return nil, fmt.Errorf("load meta for resource %q: %w", id, err)
	}
	return &meta, nil
}

func toDagMeta(id, hash string, m domresource.ResourceMeta, indexedAt *time.Time, indexedBy string) *dag.ResourceMeta {
	return &dag.ResourceMeta{
		ID:          id,
		Hash:        hash,
		Name:        m.Name,
		Type:        m.Type,
		Tags:        m.Tags,
		Description: m.Description,
		Session:     m.Session,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
		Extra:       m.Extra,
		IndexedAt:   indexedAt,
		IndexedBy:   indexedBy,
	}
}

func toResource(id string, obj *dag.ResourceObj, dagMeta *dag.ResourceMeta) *domresource.Resource {
	var meta domresource.ResourceMeta
	if dagMeta != nil {
		meta = domresource.ResourceMeta{
			Name:        dagMeta.Name,
			Type:        dagMeta.Type,
			Tags:        dagMeta.Tags,
			Description: dagMeta.Description,
			Session:     dagMeta.Session,
			CreatedAt:   dagMeta.CreatedAt,
			UpdatedAt:   dagMeta.UpdatedAt,
			Extra:       dagMeta.Extra,
		}
	}
	return &domresource.Resource{
		ID:      id,
		Content: obj.Content,
		Meta:    meta,
	}
}

func metaMatchesFilter(m *dag.ResourceMeta, preds []domresource.FilterPredicate) bool {
	for _, p := range preds {
		if !metaPredicateMatches(m, p) {
			return false
		}
	}
	return true
}

func metaPredicateMatches(m *dag.ResourceMeta, p domresource.FilterPredicate) bool {
	var got any
	switch p.Key {
	case "name":
		got = m.Name
	case "type":
		got = m.Type
	case "session":
		got = m.Session
	case "description":
		got = m.Description
	default:
		if m.Extra != nil {
			got = m.Extra[p.Key]
		}
	}
	return predicateMatches(got, p.Op, p.Value)
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

func predicateMatches(got any, op domresource.FilterOp, want any) bool {
	gs := fmt.Sprintf("%v", got)
	ws := fmt.Sprintf("%v", want)
	switch op {
	case domresource.FilterOpEq:
		return gs == ws
	case domresource.FilterOpPrefix:
		return strings.HasPrefix(gs, ws)
	case domresource.FilterOpContains:
		return strings.Contains(gs, ws)
	case domresource.FilterOpGte:
		return gs >= ws
	case domresource.FilterOpLte:
		return gs <= ws
	default:
		return false
	}
}
