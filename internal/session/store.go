package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const defaultDataDir = "./data"

// FileStore persists sessions and messages to disk under dataDir.
//
// Layout:
//
//	{dataDir}/{sessionID}/session.json
//	{dataDir}/{sessionID}/messages/{unixNano}_{msgID}.json
type FileStore struct {
	dataDir string
}

func NewFileStore(dataDir string) *FileStore {
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	return &FileStore{dataDir: dataDir}
}

func (s *FileStore) sessionDir(id string) string {
	return filepath.Join(s.dataDir, id)
}

func (s *FileStore) sessionPath(id string) string {
	return filepath.Join(s.sessionDir(id), "session.json")
}

func (s *FileStore) messagesDir(id string) string {
	return filepath.Join(s.sessionDir(id), "messages")
}

func (s *FileStore) SaveSession(sess *Session) error {
	if err := os.MkdirAll(s.messagesDir(sess.ID), 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}
	return writeJSON(s.sessionPath(sess.ID), sess)
}

func (s *FileStore) LoadSession(id string) (*Session, error) {
	var sess Session
	if err := readJSON(s.sessionPath(id), &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *FileStore) FindByName(name string) (*Session, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sess, err := s.LoadSession(e.Name())
		if err != nil {
			continue
		}
		if sess.Name == name {
			return sess, nil
		}
	}
	return nil, nil
}

func (s *FileStore) DeleteSession(id string) error {
	return os.RemoveAll(s.sessionDir(id))
}

func (s *FileStore) ListSessions(limit, offset int) ([]*Session, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	var sessions []*Session
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sess, err := s.LoadSession(e.Name())
		if err != nil {
			continue
		}
		sessions = append(sessions, sess)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	if offset >= len(sessions) {
		return []*Session{}, nil
	}
	sessions = sessions[offset:]
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

// SaveMessage writes a message file named {unixNano}_{id}.json so that
// directory listing order equals chronological order.
func (s *FileStore) SaveMessage(sessionID string, msg *Message) error {
	name := fmt.Sprintf("%020d_%s.json", msg.CreatedAt.UnixNano(), msg.ID)
	return writeJSON(filepath.Join(s.messagesDir(sessionID), name), msg)
}

func (s *FileStore) ListMessages(sessionID string, limit, offset int) ([]*Message, error) {
	dir := s.messagesDir(sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Message{}, nil
		}
		return nil, fmt.Errorf("failed to read messages directory: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}

	if offset >= len(names) {
		return []*Message{}, nil
	}
	names = names[offset:]
	if limit > 0 && len(names) > limit {
		names = names[:limit]
	}

	messages := make([]*Message, 0, len(names))
	for _, name := range names {
		var msg Message
		if err := readJSON(filepath.Join(dir, name), &msg); err != nil {
			continue
		}
		messages = append(messages, &msg)
	}
	return messages, nil
}

// GetMessage finds and returns a single message by ID within a session.
// It scans for a file matching *_{messageID}.json in the messages directory.
func (s *FileStore) GetMessage(sessionID, messageID string) (*Message, error) {
	matches, err := filepath.Glob(filepath.Join(s.messagesDir(sessionID), "*_"+messageID+".json"))
	if err != nil {
		return nil, fmt.Errorf("failed to search for message: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}
	var msg Message
	if err := readJSON(matches[0], &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// CompactMessages deletes message files matching the compaction criteria.
// When stripTools is true, it removes all role=tool messages and all
// role=assistant messages that contain tool_calls (intermediate turns).
// Returns the number of files deleted.
func (s *FileStore) CompactMessages(sessionID string, stripTools bool) (int, error) {
	dir := s.messagesDir(sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read messages directory: %w", err)
	}

	deleted := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		var msg Message
		if err := readJSON(path, &msg); err != nil {
			continue
		}
		remove := false
		if stripTools {
			remove = msg.Role == "tool" || (msg.Role == "assistant" && len(msg.ToolCalls) > 0)
		}
		if remove {
			if err := os.Remove(path); err != nil {
				return deleted, fmt.Errorf("failed to delete message %s: %w", msg.ID, err)
			}
			deleted++
		}
	}
	return deleted, nil
}

func (s *FileStore) CountMessages(sessionID string) int {
	dir := s.messagesDir(sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			count++
		}
	}
	return count
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
