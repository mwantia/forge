package dag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/contenthash"
)

// Storage is the narrow K/V surface dag needs from a backend. Both
// storage.StorageBackend and storage.StorageService satisfy it.
type Storage interface {
	ReadRaw(ctx context.Context, key string) ([]byte, error)
	WriteRaw(ctx context.Context, key string, value []byte) error
	ListEntry(ctx context.Context, prefix string) ([]string, error)
	DeleteEntry(ctx context.Context, key string) error
}

// ErrNotFound is returned when a hash or ref has no value.
var ErrNotFound = errors.New("dag: not found")

// ObjectStore is the global, content-addressed object pool.
//
// Layout: objects/<aa>/<rest-of-hash> where aa is the first two hex chars.
// Writes are PutIfAbsent semantics (writing the same hash twice is a no-op),
// giving idempotent retry for free.
type ObjectStore struct {
	Backend Storage
}

func NewObjectStore(s Storage) *ObjectStore { return &ObjectStore{Backend: s} }

// ObjectKey returns the storage key for a full-length hex hash.
func ObjectKey(hash string) string {
	if len(hash) < 3 {
		return ""
	}
	return "objects/" + hash[:2] + "/" + hash[2:]
}

// Has reports whether an object with the given hash exists. Treats empty
// content as absent — file-backed storage may return a zero-length slice
// for missing keys, so the only safe presence test is "non-empty body".
func (s *ObjectStore) Has(ctx context.Context, hash string) (bool, error) {
	b, err := s.Backend.ReadRaw(ctx, ObjectKey(hash))
	if err != nil {
		return false, err
	}
	return len(b) > 0, nil
}

// Put writes blob unconditionally at the key for hash.
func (s *ObjectStore) Put(ctx context.Context, hash string, blob []byte) error {
	if hash == "" {
		return fmt.Errorf("dag: empty hash")
	}
	return s.Backend.WriteRaw(ctx, ObjectKey(hash), blob)
}

// PutIfAbsent writes blob at the key for hash only if no value already
// exists. Same-hash double writes are a no-op.
func (s *ObjectStore) PutIfAbsent(ctx context.Context, hash string, blob []byte) error {
	has, err := s.Has(ctx, hash)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	return s.Put(ctx, hash, blob)
}

// GetRaw returns the canonical bytes for hash, or ErrNotFound.
func (s *ObjectStore) GetRaw(ctx context.Context, hash string) ([]byte, error) {
	b, err := s.Backend.ReadRaw(ctx, ObjectKey(hash))
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, ErrNotFound
	}
	return b, nil
}

// PutMessage canonicalizes m, hashes it, stores via PutIfAbsent, and
// returns the hash. The hash matches m.Hash().
func (s *ObjectStore) PutMessage(ctx context.Context, m *MessageObj) (string, error) {
	blob, err := contenthash.Canonical(m)
	if err != nil {
		return "", err
	}
	hash := contenthash.HashBytes(blob)
	if err := s.PutIfAbsent(ctx, hash, blob); err != nil {
		return "", err
	}
	return hash, nil
}

// GetMessage decodes the MessageObj at hash.
func (s *ObjectStore) GetMessage(ctx context.Context, hash string) (*MessageObj, error) {
	blob, err := s.GetRaw(ctx, hash)
	if err != nil {
		return nil, err
	}
	var m MessageObj
	if err := json.Unmarshal(blob, &m); err != nil {
		return nil, fmt.Errorf("dag: decode message %s: %w", hash, err)
	}
	return &m, nil
}

// PutPromptContext canonicalizes p, hashes it, stores it, returns the hash.
func (s *ObjectStore) PutPromptContext(ctx context.Context, p *PromptContext) (string, error) {
	blob, err := contenthash.Canonical(p)
	if err != nil {
		return "", err
	}
	hash := contenthash.HashBytes(blob)
	if err := s.PutIfAbsent(ctx, hash, blob); err != nil {
		return "", err
	}
	return hash, nil
}

// GetPromptContext decodes the PromptContext at hash.
func (s *ObjectStore) GetPromptContext(ctx context.Context, hash string) (*PromptContext, error) {
	blob, err := s.GetRaw(ctx, hash)
	if err != nil {
		return nil, err
	}
	var p PromptContext
	if err := json.Unmarshal(blob, &p); err != nil {
		return nil, fmt.Errorf("dag: decode prompt context %s: %w", hash, err)
	}
	return &p, nil
}

// PutToolCatalog canonicalizes t, hashes it, stores it, returns the hash.
func (s *ObjectStore) PutToolCatalog(ctx context.Context, t *ToolCatalog) (string, error) {
	blob, err := contenthash.Canonical(t)
	if err != nil {
		return "", err
	}
	hash := contenthash.HashBytes(blob)
	if err := s.PutIfAbsent(ctx, hash, blob); err != nil {
		return "", err
	}
	return hash, nil
}

// GetToolCatalog decodes the ToolCatalog at hash.
func (s *ObjectStore) GetToolCatalog(ctx context.Context, hash string) (*ToolCatalog, error) {
	blob, err := s.GetRaw(ctx, hash)
	if err != nil {
		return nil, err
	}
	var t ToolCatalog
	if err := json.Unmarshal(blob, &t); err != nil {
		return nil, fmt.Errorf("dag: decode tool catalog %s: %w", hash, err)
	}
	return &t, nil
}
