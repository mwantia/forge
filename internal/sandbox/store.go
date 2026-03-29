package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// FileStore persists sandbox records to disk under the owning session directory.
//
// Layout:
//
//	{dataDir}/{sessionID}/sandboxes/{sandboxID}/sandbox.json
//
// Nesting under the session directory means os.RemoveAll on the session dir
// automatically cascades to all sandbox records — no extra cleanup needed.
type FileStore struct {
	dataDir string
}

func NewFileStore(dataDir string) *FileStore {
	return &FileStore{dataDir: dataDir}
}

func (s *FileStore) sandboxDir(sessionID, sandboxID string) string {
	return filepath.Join(s.dataDir, sessionID, "sandboxes", sandboxID)
}

func (s *FileStore) sandboxPath(sessionID, sandboxID string) string {
	return filepath.Join(s.sandboxDir(sessionID, sandboxID), "sandbox.json")
}

func (s *FileStore) Save(sb *Sandbox) error {
	if err := os.MkdirAll(s.sandboxDir(sb.SessionID, sb.ID), 0755); err != nil {
		return fmt.Errorf("failed to create sandbox directory: %w", err)
	}
	return writeJSON(s.sandboxPath(sb.SessionID, sb.ID), sb)
}

func (s *FileStore) Load(sessionID, sandboxID string) (*Sandbox, error) {
	var sb Sandbox
	if err := readJSON(s.sandboxPath(sessionID, sandboxID), &sb); err != nil {
		return nil, err
	}
	return &sb, nil
}

// LoadByID finds a sandbox by ID alone, scanning across all session directories.
func (s *FileStore) LoadByID(sandboxID string) (*Sandbox, error) {
	sessions, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("sandbox %q not found", sandboxID)
		}
		return nil, err
	}
	for _, se := range sessions {
		if !se.IsDir() {
			continue
		}
		sb, err := s.Load(se.Name(), sandboxID)
		if err == nil {
			return sb, nil
		}
	}
	return nil, fmt.Errorf("sandbox %q not found", sandboxID)
}

func (s *FileStore) Delete(sessionID, sandboxID string) error {
	return os.RemoveAll(s.sandboxDir(sessionID, sandboxID))
}

// ListOptions controls filtering and pagination for list operations.
type ListOptions struct {
	Limit     int
	Offset    int
	SessionID string // empty = all sessions
}

func (s *FileStore) List(opts ListOptions) ([]*Sandbox, error) {
	sessions, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Sandbox{}, nil
		}
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	var all []*Sandbox
	for _, se := range sessions {
		if !se.IsDir() {
			continue
		}
		sessionID := se.Name()
		if opts.SessionID != "" && sessionID != opts.SessionID {
			continue
		}
		sbs, err := s.listBySession(sessionID)
		if err != nil {
			continue
		}
		all = append(all, sbs...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	if opts.Offset >= len(all) {
		return []*Sandbox{}, nil
	}
	all = all[opts.Offset:]
	if opts.Limit > 0 && len(all) > opts.Limit {
		all = all[:opts.Limit]
	}
	return all, nil
}

func (s *FileStore) listBySession(sessionID string) ([]*Sandbox, error) {
	sandboxesDir := filepath.Join(s.dataDir, sessionID, "sandboxes")
	entries, err := os.ReadDir(sandboxesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []*Sandbox
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sb, err := s.Load(sessionID, e.Name())
		if err != nil {
			continue
		}
		result = append(result, sb)
	}
	return result, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
