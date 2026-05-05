package dag

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
)

const (
	// HEAD is the symbolic ref that tracks the active branch.
	HEAD = "HEAD"
	// MAIN is the default primary branch created with every new session.
	MAIN = "main"
	// symrefPrefix is the byte prefix written to a ref file to mark it as a
	// symbolic pointer (same convention as git). Value: "ref: <branch>".
	symrefPrefix = "ref: "
)

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
// as raw ASCII bytes. Ref names may contain "/" to form logical groups
// (e.g. "event/my-hook-2026-05-05T06:00:00Z"), which the file backend
// stores as real subdirectories. List walks the tree recursively and
// returns the full relative name including any "/" segments.
// CAS is enforced via a per-key in-process mutex; this is sufficient for
// single-process Forge agents and is the contract callers should rely on.
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

// resolveRef follows one level of symbolic indirection without acquiring
// any lock. Returns name unchanged if the ref is absent or is a plain hash.
func (r *RefStore) resolveRef(ctx context.Context, sessionID, name string) string {
	b, err := r.Backend.ReadRaw(ctx, refKey(sessionID, name))
	if err != nil || len(b) == 0 {
		return name
	}
	s := strings.TrimSpace(string(b))
	if target, ok := strings.CutPrefix(s, symrefPrefix); ok {
		return target
	}
	return name
}

// Read returns the hash a ref points at, or ErrNotFound if the ref does not
// exist. Symbolic refs (written by WriteSymRef) are dereferenced one level.
func (r *RefStore) Read(ctx context.Context, sessionID, name string) (string, error) {
	b, err := r.Backend.ReadRaw(ctx, refKey(sessionID, name))
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", ErrNotFound
	}
	s := strings.TrimSpace(string(b))
	if target, ok := strings.CutPrefix(s, symrefPrefix); ok {
		return r.Read(ctx, sessionID, target)
	}
	return s, nil
}

// WriteSymRef writes a symbolic ref: the ref file contains "ref: <target>"
// so that Read/Write/CAS on name are forwarded to target.
func (r *RefStore) WriteSymRef(ctx context.Context, sessionID, name, target string) error {
	if name == "" || target == "" {
		return fmt.Errorf("dag: empty ref name or target in WriteSymRef")
	}
	key := refKey(sessionID, name)
	m := r.lockFor(key)
	m.Lock()
	defer m.Unlock()
	return r.Backend.WriteRaw(ctx, key, []byte(symrefPrefix+target))
}

// ReadSymRef reports whether name is a symbolic ref and returns its target.
// Returns ("", false, nil) when name is a plain hash ref or does not exist.
func (r *RefStore) ReadSymRef(ctx context.Context, sessionID, name string) (target string, isSymref bool, err error) {
	b, readErr := r.Backend.ReadRaw(ctx, refKey(sessionID, name))
	if readErr != nil {
		return "", false, readErr
	}
	if len(b) == 0 {
		return "", false, nil
	}
	s := strings.TrimSpace(string(b))
	if target, ok := strings.CutPrefix(s, symrefPrefix); ok {
		return target, true, nil
	}
	return "", false, nil
}

// Write unconditionally points name at hash. When name is a symbolic ref,
// the write is forwarded to the symref target. Use CAS for branch advance.
func (r *RefStore) Write(ctx context.Context, sessionID, name, hash string) error {
	if name == "" {
		return fmt.Errorf("dag: empty ref name")
	}
	target := r.resolveRef(ctx, sessionID, name)
	key := refKey(sessionID, target)
	m := r.lockFor(key)
	m.Lock()
	defer m.Unlock()
	return r.Backend.WriteRaw(ctx, key, []byte(hash))
}

// CAS advances ref name from expected to next, or returns CASConflict.
// An expected value of "" means "ref must not currently exist".
// When name is a symbolic ref, the CAS is forwarded to the symref target.
func (r *RefStore) CAS(ctx context.Context, sessionID, name, expected, next string) error {
	if name == "" {
		return fmt.Errorf("dag: empty ref name")
	}
	target := r.resolveRef(ctx, sessionID, name)
	key := refKey(sessionID, target)
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
			Ref:      target,
			Expected: expected,
			Actual:   curStr,
		}
	}
	return r.Backend.WriteRaw(ctx, key, []byte(next))
}

// List returns all refs for a session as name -> hash.
// Ref names containing "/" (e.g. "event/my-hook-...") are returned with
// their full relative path — the walk is recursive.
func (r *RefStore) List(ctx context.Context, sessionID string) (map[string]string, error) {
	out := make(map[string]string)
	if err := r.listRecursive(ctx, sessionID, refsPrefix(sessionID), "", out); err != nil {
		return nil, err
	}
	return out, nil
}

// listRecursive descends into the storage prefix, prepending relPrefix to
// every leaf name it finds, and recurses into any subdirectory entries
// (identified by a trailing "/" from the backend).
func (r *RefStore) listRecursive(ctx context.Context, sessionID, storagePrefix, relPrefix string, out map[string]string) error {
	entries, err := r.Backend.ListEntry(ctx, storagePrefix)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if sub, ok := strings.CutSuffix(e, "/"); ok {
			if err := r.listRecursive(ctx, sessionID, storagePrefix+e, relPrefix+sub+"/", out); err != nil {
				return err
			}
			continue
		}
		name := relPrefix + e
		hash, err := r.Read(ctx, sessionID, name)
		if err != nil {
			if err == ErrNotFound {
				continue
			}
			return err
		}
		out[name] = hash
	}
	return nil
}

// Delete removes a ref. Missing refs are not an error.
func (r *RefStore) Delete(ctx context.Context, sessionID, name string) error {
	key := refKey(sessionID, name)
	m := r.lockFor(key)
	m.Lock()
	defer m.Unlock()
	return r.Backend.DeleteEntry(ctx, key)
}

// Rename atomically renames oldName to newName: reads old, writes new, deletes
// old. Returns 409-style error if newName already exists.
func (r *RefStore) Rename(ctx context.Context, sessionID, oldName, newName string) error {
	if oldName == "" || newName == "" {
		return fmt.Errorf("dag: empty ref name in rename")
	}

	// Lock both keys in lexicographic order to avoid deadlock.
	oldKey := refKey(sessionID, oldName)
	newKey := refKey(sessionID, newName)
	first, second := oldKey, newKey
	if bytes.Compare([]byte(first), []byte(second)) > 0 {
		first, second = second, first
	}
	r.lockFor(first).Lock()
	defer r.lockFor(first).Unlock()
	r.lockFor(second).Lock()
	defer r.lockFor(second).Unlock()

	// Target must not already exist.
	existing, err := r.Backend.ReadRaw(ctx, newKey)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return &CASConflict{
			Session:  sessionID,
			Ref:      newName,
			Expected: "",
			Actual:   strings.TrimSpace(string(existing)),
		}
	}

	// Read source.
	hash, err := r.Backend.ReadRaw(ctx, oldKey)
	if err != nil {
		return err
	}
	if len(hash) == 0 {
		return fmt.Errorf("dag: ref %q not found", oldName)
	}

	// Write new, delete old.
	if err := r.Backend.WriteRaw(ctx, newKey, hash); err != nil {
		return err
	}
	return r.Backend.DeleteEntry(ctx, oldKey)
}
