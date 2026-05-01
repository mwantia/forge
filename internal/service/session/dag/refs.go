package dag

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
)

// HEAD is the conventional ref name for a session's active branch.
const HEAD = "HEAD"

// CASConflict is returned by RefStore.CAS when the current value does not
// match the expected value. Callers retry against Actual or surface a
// conflict to the user.
type CASConflict struct {
	Session  string
	Ref      string
	Expected string
	Actual   string
}

func (e *CASConflict) Error() string {
	return fmt.Sprintf("dag: CAS conflict on %s/%s: expected %q, got %q",
		e.Session, e.Ref, e.Expected, e.Actual)
}

// RefStore manages mutable session refs (HEAD + branches + tags).
//
// Layout: sessions/<session_id>/refs/<ref_name>, value = the message hash
// as raw ASCII bytes. CAS is enforced via a per-key in-process mutex; this
// is sufficient for single-process Forge agents and is the contract
// callers should rely on.
type RefStore struct {
	Backend Storage

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewRefStore(s Storage) *RefStore {
	return &RefStore{Backend: s, locks: make(map[string]*sync.Mutex)}
}

func refKey(sessionID, name string) string {
	return fmt.Sprintf("sessions/%s/refs/%s", sessionID, name)
}

func refsPrefix(sessionID string) string {
	return fmt.Sprintf("sessions/%s/refs/", sessionID)
}

func (r *RefStore) lockFor(key string) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.locks[key]
	if !ok {
		m = &sync.Mutex{}
		r.locks[key] = m
	}
	return m
}

// Read returns the hash a ref points at, or ErrNotFound if the ref does
// not exist.
func (r *RefStore) Read(ctx context.Context, sessionID, name string) (string, error) {
	b, err := r.Backend.ReadRaw(ctx, refKey(sessionID, name))
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", ErrNotFound
	}
	return strings.TrimSpace(string(b)), nil
}

// Write unconditionally points name at hash. Use CAS for branch advance.
func (r *RefStore) Write(ctx context.Context, sessionID, name, hash string) error {
	if name == "" {
		return fmt.Errorf("dag: empty ref name")
	}
	key := refKey(sessionID, name)
	m := r.lockFor(key)
	m.Lock()
	defer m.Unlock()
	return r.Backend.WriteRaw(ctx, key, []byte(hash))
}

// CAS advances ref name from expected to next, or returns CASConflict.
// An expected value of "" means "ref must not currently exist".
func (r *RefStore) CAS(ctx context.Context, sessionID, name, expected, next string) error {
	if name == "" {
		return fmt.Errorf("dag: empty ref name")
	}
	key := refKey(sessionID, name)
	m := r.lockFor(key)
	m.Lock()
	defer m.Unlock()

	cur, err := r.Backend.ReadRaw(ctx, key)
	if err != nil {
		return err
	}
	curStr := ""
	if cur != nil {
		curStr = string(bytes.TrimSpace(cur))
	}
	if curStr != expected {
		return &CASConflict{
			Session:  sessionID,
			Ref:      name,
			Expected: expected,
			Actual:   curStr,
		}
	}
	return r.Backend.WriteRaw(ctx, key, []byte(next))
}

// List returns all refs for a session as name -> hash.
func (r *RefStore) List(ctx context.Context, sessionID string) (map[string]string, error) {
	prefix := refsPrefix(sessionID)
	entries, err := r.Backend.ListEntry(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		// Skip sub-prefix entries; refs are flat leaves.
		if strings.HasSuffix(e, "/") {
			continue
		}
		name := strings.TrimPrefix(e, prefix)
		// Some backends return the bare leaf name; handle both shapes.
		if name == e {
			name = e
		}
		hash, err := r.Read(ctx, sessionID, name)
		if err != nil {
			if err == ErrNotFound {
				continue
			}
			return nil, err
		}
		out[name] = hash
	}
	return out, nil
}

// Delete removes a ref. Missing refs are not an error.
func (r *RefStore) Delete(ctx context.Context, sessionID, name string) error {
	key := refKey(sessionID, name)
	m := r.lockFor(key)
	m.Lock()
	defer m.Unlock()
	return r.Backend.DeleteEntry(ctx, key)
}
