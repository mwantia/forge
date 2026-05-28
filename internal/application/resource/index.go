package resource

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
	"github.com/viterin/vek/vek32"
)

// vectorIndex holds two shared flat vector maps keyed by graph name.
// "memory" covers all */memories namespaces; "resources" covers everything else.
// Both span all sessions so cross-session semantic search works naturally.
// Node keys are "namespace/name" (e.g. "forge/sessions/abc/memories/dark-mode").
type vectorIndex struct {
	mu   sync.Mutex
	maps map[string]map[string][]float32 // graph-key → (node-key → normalized vector)
}

func newVectorIndex() *vectorIndex {
	return &vectorIndex{maps: make(map[string]map[string][]float32)}
}

// graphKey returns the shared index name for a namespace path.
func graphKey(ns string) string {
	if ns == "memories" || strings.HasSuffix(ns, "/memories") {
		return "memory"
	}
	return "resources"
}

func vectorKey(gk string) string {
	return "hnsw/" + gk + ".bin"
}

// load returns the vector map for gk, loading from storage if necessary.
// Creates an empty map when nothing is stored yet. Caller must NOT hold mu.
func (idx *vectorIndex) load(ctx context.Context, s *ResourceService, gk string) map[string][]float32 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if m, ok := idx.maps[gk]; ok {
		return m
	}

	m := make(map[string][]float32)
	raw, err := s.storage.ReadRaw(ctx, vectorKey(gk))
	if err == nil && len(raw) > 0 {
		if loaded, decErr := decodeVectorMap(bytes.NewReader(raw)); decErr == nil {
			m = loaded
		} else {
			s.logger.Warn("Failed to decode vector index; starting fresh", "graph", gk, "error", decErr)
		}
	}
	idx.maps[gk] = m
	return m
}

// persist serializes the vector map for gk to the storage backend.
// Caller must NOT hold mu.
func (idx *vectorIndex) persist(ctx context.Context, s *ResourceService, gk string, m map[string][]float32) {
	var buf bytes.Buffer
	if err := encodeVectorMap(&buf, m); err != nil {
		s.logger.Warn("Failed to encode vector index", "graph", gk, "error", err)
		return
	}
	if err := s.storage.WriteRaw(ctx, vectorKey(gk), buf.Bytes()); err != nil {
		s.logger.Warn("Failed to persist vector index", "graph", gk, "error", err)
	}
}

// encodeVectorMap writes the binary format defined in docs/13-proposal-flat-vector-index.md.
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

// decodeVectorMap reads the binary format written by encodeVectorMap.
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

// normalizeVec returns a unit vector so that vek32.Dot == cosine similarity.
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

// indexContent embeds content and stores the normalized vector in the shared index for ns.
func (s *ResourceService) indexContent(ctx context.Context, ns, name string, res *sdkplugins.Resource) {
	if s.embedProvider == "" || s.embedModel == "" {
		return
	}

	vecs, err := s.provider.Embed(ctx, s.embedProvider, s.embedModel, res.Content)
	if err != nil || len(vecs) == 0 {
		s.logger.Warn("Embed failed for resource; skipping index", "namespace", ns, "name", name, "error", err)
		return
	}

	gk := graphKey(ns)
	nodeKey := ns + "/" + name
	vec := normalizeVec(vecs[0])

	m := s.idx.load(ctx, s, gk)
	s.idx.mu.Lock()
	m[nodeKey] = vec
	s.idx.mu.Unlock()
	s.idx.persist(ctx, s, gk, m)

	now := time.Now()
	indexedBy := fmt.Sprintf("%s/%s", s.embedProvider, s.embedModel)
	s.updateIndexedAt(ctx, "/"+ns, name, now, indexedBy, res.Tags, res.Metadata)
}

// removeFromIndex is intentionally a no-op. Forgotten resources are soft-deleted:
// their vector entry stays in the map and is skipped at recall time when
// hitStore.Get returns an error for the absent resource.
func (s *ResourceService) removeFromIndex(_ context.Context, _, _ string) {}

type scoredHit struct {
	nodeKey string
	score   float32
}

// recallSemantic performs cross-session semantic recall using the shared flat vector index.
// The index is selected by ns (via graphKey). Each hit's node key is parsed back into
// a path + name so the resource can be fetched from the correct store.
// Returns nil, false when semantic search is not available or query is empty.
func (s *ResourceService) recallSemantic(ctx context.Context, ns, query string, q sdkplugins.RecallQuery) ([]*sdkplugins.Resource, bool) {
	if s.embedProvider == "" || s.embedModel == "" || query == "" {
		return nil, false
	}

	qvecs, err := s.provider.Embed(ctx, s.embedProvider, s.embedModel, query)
	if err != nil || len(qvecs) == 0 {
		s.logger.Warn("Embed failed for recall query; falling back to substring", "error", err)
		return nil, false
	}
	qvec := normalizeVec(qvecs[0])

	gk := graphKey(ns)
	m := s.idx.load(ctx, s, gk)

	limit := q.Limit
	if limit <= 0 {
		limit = 5
	}

	s.idx.mu.Lock()
	hits := make([]scoredHit, 0, len(m))
	for key, vec := range m {
		hits = append(hits, scoredHit{key, vek32.Dot(qvec, vec)})
	}
	s.idx.mu.Unlock()

	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > limit {
		hits = hits[:limit]
	}

	if len(hits) == 0 {
		return []*sdkplugins.Resource{}, true
	}

	var out []*sdkplugins.Resource
	for _, hit := range hits {
		lastSlash := strings.LastIndex(hit.nodeKey, "/")
		if lastSlash < 0 {
			continue
		}
		hitPath := "/" + hit.nodeKey[:lastSlash]
		hitName := hit.nodeKey[lastSlash+1:]

		res, err := s.defaultStore.Get(ctx, hitPath, hitName)
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

// passesFilters checks tag, metadata, and time filters on a resource.
func passesFilters(res *sdkplugins.Resource, q sdkplugins.RecallQuery) bool {
	if !tagsMatch(res.Tags, q.Tags) {
		return false
	}
	if !predicatesMatch(res.Metadata, q.Filter) {
		return false
	}
	if !q.CreatedAfter.IsZero() && res.CreatedAt.Before(q.CreatedAfter) {
		return false
	}
	if !q.CreatedBefore.IsZero() && res.CreatedAt.After(q.CreatedBefore) {
		return false
	}
	return true
}

// updateIndexedAt writes a new ResourceMeta log entry stamping IndexedAt/IndexedBy.
func (s *ResourceService) updateIndexedAt(ctx context.Context, path, name string, indexedAt time.Time, indexedBy string, tags []string, metadata map[string]any) {
	dagStore, ok := s.defaultStore.(*dagResourceStore)
	if !ok {
		return
	}
	ns := namespace(path)
	hash, err := dagStore.refs.ReadKey(ctx, dag.ResourceRefKey(ns, name))
	if err != nil {
		return
	}

	existing := dagStore.loadRecentMeta(ctx, ns, hash)
	now := time.Now()
	m := &dag.ResourceMeta{
		Hash:      hash,
		Namespace: ns,
		Name:      name,
		Tags:      tags,
		Metadata:  metadata,
		CreatedAt: now,
		IndexedAt: &indexedAt,
		IndexedBy: indexedBy,
	}
	if existing != nil {
		if m.Tags == nil {
			m.Tags = existing.Tags
		}
		if m.Metadata == nil {
			m.Metadata = existing.Metadata
		}
		m.CreatedAt = existing.CreatedAt
	}
	if err := dagStore.writeResourceLogEntry(ctx, ns, now, hash, m); err != nil {
		s.logger.Warn("Failed to write IndexedAt to resource meta", "namespace", ns, "name", name, "error", err)
	}
}
