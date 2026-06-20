package resource

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	domresource "github.com/mwantia/forge/internal/domain/resource"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
	"github.com/viterin/vek/vek32"
)

const vectorIndexKey = "flat/resources.bin"

// vectorIndex is a flat in-memory cosine similarity index over all resources.
// Node keys are resource IDs.
type vectorIndex struct {
	mu sync.Mutex
	m  map[string][]float32 // id → normalized vector
}

func newVectorIndex() *vectorIndex {
	return &vectorIndex{m: make(map[string][]float32)}
}

// load populates the in-memory map from storage if not already loaded.
func (idx *vectorIndex) load(ctx context.Context, s *ResourceService) map[string][]float32 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.m != nil && len(idx.m) > 0 {
		return idx.m
	}

	m := make(map[string][]float32)
	raw, err := s.storage.ReadRaw(ctx, vectorIndexKey)
	if err == nil && len(raw) > 0 {
		if loaded, decErr := decodeVectorMap(bytes.NewReader(raw)); decErr == nil {
			m = loaded
		} else {
			s.logger.Warn("Failed to decode vector index; starting fresh", "error", decErr)
		}
	}
	idx.m = m
	return m
}

// persist serializes the in-memory map to storage.
func (idx *vectorIndex) persist(ctx context.Context, s *ResourceService, m map[string][]float32) {
	var buf bytes.Buffer
	if err := encodeVectorMap(&buf, m); err != nil {
		s.logger.Warn("Failed to encode vector index", "error", err)
		return
	}
	if err := s.storage.WriteRaw(ctx, vectorIndexKey, buf.Bytes()); err != nil {
		s.logger.Warn("Failed to persist vector index", "error", err)
	}
}

func encodeVectorMap(w io.Writer, m map[string][]float32) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(m))); err != nil {
		return err
	}
	for key, vec := range m {
		kb := []byte(key)
		if err := binary.Write(w, binary.LittleEndian, uint32(len(kb))); err != nil {
			return err
		}
		if _, err := w.Write(kb); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(vec))); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, vec); err != nil {
			return err
		}
	}
	return nil
}

func decodeVectorMap(r io.Reader) (map[string][]float32, error) {
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return nil, err
	}
	m := make(map[string][]float32, count)
	for range count {
		var keyLen uint32
		if err := binary.Read(r, binary.LittleEndian, &keyLen); err != nil {
			return nil, err
		}
		kb := make([]byte, keyLen)
		if _, err := io.ReadFull(r, kb); err != nil {
			return nil, err
		}
		var dim uint32
		if err := binary.Read(r, binary.LittleEndian, &dim); err != nil {
			return nil, err
		}
		vec := make([]float32, dim)
		if err := binary.Read(r, binary.LittleEndian, vec); err != nil {
			return nil, err
		}
		m[string(kb)] = vec
	}
	return m, nil
}

func normalizeVec(vec []float32) []float32 {
	norm := vek32.Norm(vec)
	if norm == 0 {
		return vec
	}
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = v / norm
	}
	return out
}

// indexContent embeds content and stores the normalized vector for id.
func (s *ResourceService) indexContent(ctx context.Context, id string, res *domresource.Resource) {
	if s.embedProvider == "" || s.embedModel == "" {
		return
	}

	vecs, err := s.provider.Embed(ctx, s.embedProvider, s.embedModel, res.Content)
	if err != nil || len(vecs) == 0 {
		s.logger.Warn("Embed failed for resource; skipping index", "id", id, "error", err)
		return
	}

	vec := normalizeVec(vecs[0])

	m := s.idx.load(ctx, s)
	s.idx.mu.Lock()
	m[id] = vec
	s.idx.mu.Unlock()
	s.idx.persist(ctx, s, m)

	now := time.Now()
	indexedBy := fmt.Sprintf("%s/%s", s.embedProvider, s.embedModel)
	s.updateIndexedAt(ctx, id, now, indexedBy)
}

// removeFromIndex is intentionally a no-op. Forgotten resources stay in the
// vector map and are skipped at recall time when Get returns not-found.
func (s *ResourceService) removeFromIndex(_ context.Context, _ string) {}

type scoredHit struct {
	id    string
	score float32
}

// recallSemantic performs semantic recall using the flat vector index.
// Returns nil, false when semantic search is not available or query is empty.
func (s *ResourceService) recallSemantic(ctx context.Context, query string, q domresource.RecallQuery) ([]*domresource.Resource, bool) {
	if s.embedProvider == "" || s.embedModel == "" || query == "" {
		return nil, false
	}

	qvecs, err := s.provider.Embed(ctx, s.embedProvider, s.embedModel, query)
	if err != nil || len(qvecs) == 0 {
		s.logger.Warn("Embed failed for recall query; falling back to substring", "error", err)
		return nil, false
	}
	qvec := normalizeVec(qvecs[0])

	m := s.idx.load(ctx, s)

	limit := q.Limit
	if limit <= 0 {
		limit = 5
	}

	s.idx.mu.Lock()
	hits := make([]scoredHit, 0, len(m))
	for id, vec := range m {
		hits = append(hits, scoredHit{id, vek32.Dot(qvec, vec)})
	}
	s.idx.mu.Unlock()

	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > limit {
		hits = hits[:limit]
	}

	if len(hits) == 0 {
		return []*domresource.Resource{}, true
	}

	var out []*domresource.Resource
	for _, hit := range hits {
		res, err := s.defaultStore.Get(ctx, hit.id)
		if err != nil {
			continue // soft-deleted or moved; stale vector entry skipped
		}
		if !passesFilters(res, q) {
			continue
		}
		res.Score = float64(hit.score)
		out = append(out, res)
	}
	return out, true
}

func passesFilters(res *domresource.Resource, q domresource.RecallQuery) bool {
	if !tagsMatch(res.Meta.Tags, q.Tags) {
		return false
	}
	if !metaMatchesFilter(metaFromResource(res), q.Filter) {
		return false
	}
	if !q.CreatedAfter.IsZero() && res.Meta.CreatedAt.Before(q.CreatedAfter) {
		return false
	}
	if !q.CreatedBefore.IsZero() && res.Meta.CreatedAt.After(q.CreatedBefore) {
		return false
	}
	return true
}

// metaFromResource builds a minimal dag.ResourceMeta for filter evaluation.
func metaFromResource(res *domresource.Resource) *dag.ResourceMeta {
	return &dag.ResourceMeta{
		ID:          res.ID,
		Name:        res.Meta.Name,
		Type:        res.Meta.Type,
		Session:     res.Meta.Session,
		Description: res.Meta.Description,
		Extra:       res.Meta.Extra,
	}
}

func (s *ResourceService) updateIndexedAt(ctx context.Context, id string, indexedAt time.Time, indexedBy string) {
	dagStore, ok := s.defaultStore.(*dagResourceStore)
	if !ok {
		return
	}
	existing, err := dagStore.loadMeta(ctx, id)
	if err != nil {
		return
	}
	existing.IndexedAt = &indexedAt
	existing.IndexedBy = indexedBy
	if err := dagStore.storage.WriteJson(ctx, dag.ResourceMetaKey(id), existing); err != nil {
		s.logger.Warn("Failed to write IndexedAt to resource meta", "id", id, "error", err)
	}
}
