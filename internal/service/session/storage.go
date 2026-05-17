package session

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mwantia/forge/internal/service/session/dag"
	"github.com/mwantia/forge/internal/service/storage"
)

// DagStore persists sessions on the content-addressed Merkle DAG
// (docs/03-proposal-merkle-DAG-concept.md). Objects (MessageObj,
// PromptContext, ToolCatalog) live in the global object pool; per-session
// state is the SessionMetadata, the refs, and the audit log.
//
// Layout:
//
//	objects/<aa>/<rest-of-hash>                       # immutable blobs
//	sessions/<id>/session.json                        # SessionMetadata
//	sessions/<id>/refs/<ref>                          # message-hash bytes
//	sessions/<id>/log/<020d-unix_nano>_<hash>.json    # MessageMeta sidecar
type DagStore struct {
	storage storage.StorageBackend
	objects *dag.ObjectStore
	refs    *dag.RefStore
}

func NewDagStore(s storage.StorageBackend) *DagStore {
	return &DagStore{
		storage: s,
		objects: dag.NewObjectStore(s),
		refs:    dag.NewRefStore(s),
	}
}

// --- session metadata ---

func (m *DagStore) LoadSession(ctx context.Context, id string) (*SessionMetadata, error) {
	meta := &SessionMetadata{}
	if err := m.storage.ReadJson(ctx, constructSessionKey(id), meta); err != nil {
		return nil, fmt.Errorf("failed to load session %q: %w", id, err)
	}

	if meta.ID == "" {
		return nil, fmt.Errorf("invalid session id received: %s", id)
	}

	return meta, nil
}

func (m *DagStore) SaveSession(ctx context.Context, s *SessionMetadata) error {
	if err := m.storage.WriteJson(ctx, constructSessionKey(s.ID), s); err != nil {
		return fmt.Errorf("failed to save session %q: %w", s.ID, err)
	}

	return nil
}

func (m *DagStore) DeleteSession(ctx context.Context, id string) error {
	if err := m.storage.DeletePrefix(ctx, constructSessionPrefix(id)); err != nil {
		return fmt.Errorf("failed to delete session %q: %w", id, err)
	}

	return nil
}

func (m *DagStore) ListParentSessions(ctx context.Context, parentID string, archived bool, offset, limit int) ([]*SessionMetadata, error) {
	entries, err := m.storage.ListEntry(ctx, "sessions/")
	if err != nil {
		return nil, err
	}

	sessions := make([]*SessionMetadata, 0)
	for _, entry := range entries {
		if !strings.HasSuffix(entry, "/") {
			continue
		}

		id := strings.TrimSuffix(entry, "/")
		meta, err := m.LoadSession(ctx, id)
		if err != nil {
			continue
		}

		if parentID != "" && meta.Parent != parentID {
			continue
		}

		if archived != (meta.ArchivedAt != nil) {
			continue
		}

		sessions = append(sessions, meta)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	if offset > 0 {
		if offset >= len(sessions) {
			return []*SessionMetadata{}, nil
		}
		sessions = sessions[offset:]
	}

	if limit > 0 && limit < len(sessions) {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

func (m *DagStore) ListSessions(ctx context.Context, archived bool, offset, limit int) ([]*SessionMetadata, error) {
	return m.ListParentSessions(ctx, "", archived, offset, limit)
}

func (m *DagStore) FindSessionByName(ctx context.Context, name string) (*SessionMetadata, error) {
	entries, err := m.storage.ListEntry(ctx, "sessions/")
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if !strings.HasSuffix(e, "/") {
			continue
		}

		id := strings.TrimSuffix(e, "/")
		meta, err := m.LoadSession(ctx, id)
		if err != nil {
			continue
		}

		if meta.Name == name {
			return meta, nil
		}
	}

	return nil, nil
}

func (m *DagStore) HeadHash(ctx context.Context, sessionID string) (string, error) {
	hash, err := m.refs.Read(ctx, sessionID, dag.HEAD)
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return "", nil
		}

		return "", err
	}

	return hash, nil
}

// SaveMessage writes a MessageObj to the global object store, records a
// log-entry meta sidecar, and CAS-advances ref from its current value to
// the new hash. Empty ref defaults to HEAD. Returns the new hash and
// mutates msg.Hash + msg.ParentHash + msg.CreatedAt for the caller.
func (m *DagStore) SaveMessage(ctx context.Context, sessionID, ref string, msg *Message) (string, error) {
	if ref == "" {
		ref = dag.HEAD
	}

	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	prevTip, err := m.refs.Read(ctx, sessionID, ref)
	if err != nil && !errors.Is(err, dag.ErrNotFound) {
		return "", fmt.Errorf("read ref %q: %w", ref, err)
	}

	if msg.ParentHash == "" {
		msg.ParentHash = prevTip
	}

	obj := toDagMessageObj(msg)
	hash, err := m.objects.PutMessage(ctx, obj)
	if err != nil {
		return "", fmt.Errorf("put message object: %w", err)
	}

	msg.Hash = hash
	meta := &dag.MessageMeta{
		Hash:        hash,
		SessionID:   sessionID,
		ContextHash: msg.ContextHash,
		CreatedAt:   msg.CreatedAt,
		Usage:       msg.Usage,
	}
	if err := m.writeLogEntry(ctx, sessionID, meta); err != nil {
		return "", fmt.Errorf("write log entry: %w", err)
	}

	if err := m.refs.CAS(ctx, sessionID, ref, prevTip, hash); err != nil {
		return "", fmt.Errorf("advance %s: %w", ref, err)
	}

	return hash, nil
}

// LoadMessage returns the message at hashOrPrefix. Prefix matching is
// delegated to a log-scan when a non-full hash is supplied.
func (m *DagStore) LoadMessage(ctx context.Context, sessionID, hashOrPrefix string) (*Message, error) {
	hash := hashOrPrefix
	if len(hash) != 64 {
		resolved, err := m.ResolvePrefix(ctx, sessionID, hashOrPrefix)
		if err != nil {
			return nil, err
		}
		hash = resolved
	}

	obj, err := m.objects.GetMessage(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("load message %s: %w", hashOrPrefix, err)
	}

	meta, _ := m.findMeta(ctx, sessionID, hash)

	return fromDagMessageObj(hash, obj, meta), nil
}

// ListMessages walks HEAD chronologically. offset skips the most-recent
// messages; limit caps the result. limit==0 means "all".
func (m *DagStore) ListMessages(ctx context.Context, sessionID string, offset, limit int) ([]*Message, error) {
	head, err := m.HeadHash(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if head == "" {
		return []*Message{}, nil
	}

	entries, err := dag.Walk(ctx, m.objects, m.refs, sessionID, head, limit, offset)
	if err != nil {
		return nil, err
	}

	metaByHash, err := m.loadAllMetas(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]*Message, 0, len(entries))
	for _, e := range entries {
		out = append(out, fromDagMessageObj(e.Hash, e.Message, metaByHash[e.Hash]))
	}

	return out, nil
}

func (m *DagStore) ListMessagesFromRef(ctx context.Context, sessionID, branch string, offset, limit int) ([]*Message, error) {
	tip, err := m.refs.Read(ctx, sessionID, branch)
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return []*Message{}, nil
		}

		return nil, err
	}

	if tip == "" {
		return []*Message{}, nil
	}

	entries, err := dag.Walk(ctx, m.objects, m.refs, sessionID, tip, limit, offset)
	if err != nil {
		return nil, err
	}

	metaByHash, err := m.loadAllMetas(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]*Message, 0, len(entries))
	for _, e := range entries {
		out = append(out, fromDagMessageObj(e.Hash, e.Message, metaByHash[e.Hash]))
	}

	return out, nil
}

func (m *DagStore) CountMessages(ctx context.Context, sessionID string) (int, error) {
	head, err := m.HeadHash(ctx, sessionID)
	if err != nil {
		return 0, err
	}

	if head == "" {
		return 0, nil
	}

	entries, err := dag.Walk(ctx, m.objects, m.refs, sessionID, head, 0, 0)
	if err != nil {
		return 0, err
	}

	return len(entries), nil
}

// CompactToolsMessages rewrites the active branch with all "tool" turns
// and assistant turns whose only payload was tool calls removed. Because
// the DAG is immutable, the old chain stays in the object store as
// orphaned blobs (collected by `forge gc` in a future phase).
func (m *DagStore) CompactToolsMessages(ctx context.Context, sessionID string) (int, error) {
	head, err := m.HeadHash(ctx, sessionID)
	if err != nil {
		return 0, err
	}

	if head == "" {
		return 0, nil
	}

	entries, err := dag.Walk(ctx, m.objects, m.refs, sessionID, head, 0, 0)
	if err != nil {
		return 0, err
	}

	deleted := 0
	parent := ""
	var newHead string
	metaByHash, err := m.loadAllMetas(ctx, sessionID)
	if err != nil {
		return 0, err
	}

	// Walk chronologically; rebuild a new chain.
	for _, e := range entries {
		obj := e.Message
		if obj.Role == "tool" || (obj.Role == "assistant" && len(obj.ToolCalls) > 0 && obj.Content == "") {
			deleted++
			continue
		}
		rebuilt := &dag.MessageObj{
			Role:       obj.Role,
			Content:    obj.Content,
			ToolCalls:  obj.ToolCalls,
			ParentHash: parent,
		}
		hash, err := m.objects.PutMessage(ctx, rebuilt)
		if err != nil {
			return deleted, err
		}

		// Carry forward original meta (CreatedAt, ContextHash) under new hash.
		oldMeta := metaByHash[e.Hash]
		newMeta := &dag.MessageMeta{
			Hash:      hash,
			SessionID: sessionID,
			CreatedAt: time.Now(),
		}

		if oldMeta != nil {
			newMeta.CreatedAt = oldMeta.CreatedAt
			newMeta.ContextHash = oldMeta.ContextHash
			newMeta.Usage = oldMeta.Usage
		}

		if err := m.writeLogEntry(ctx, sessionID, newMeta); err != nil {
			return deleted, err
		}

		parent = hash
		newHead = hash
	}

	if newHead == "" {
		// Compaction removed every message; reset HEAD to "".
		if err := m.refs.Write(ctx, sessionID, dag.HEAD, ""); err != nil {
			return deleted, err
		}

		return deleted, nil
	}

	if newHead == head {
		return 0, nil
	}

	if err := m.refs.CAS(ctx, sessionID, dag.HEAD, head, newHead); err != nil {
		return deleted, fmt.Errorf("failed compact: HEAD moved during rebuild: %w", err)
	}
	// if err := m.refs.Write(ctx, sessionID, dag.HEAD, newHead); err != nil {
	//  	return deleted, err
	// }

	return deleted, nil
}

func logPrefix(sessionID string) string {
	return constructSessionPrefix(sessionID) + "log/"
}

func logKey(sessionID string, createdAt time.Time, hash string) string {
	return fmt.Sprintf("%s%020d_%s.json", logPrefix(sessionID), createdAt.UnixNano(), hash)
}

func (m *DagStore) writeLogEntry(ctx context.Context, sessionID string, meta *dag.MessageMeta) error {
	return m.storage.WriteJson(ctx, logKey(sessionID, meta.CreatedAt, meta.Hash), meta)
}

func (m *DagStore) loadAllMetas(ctx context.Context, sessionID string) (map[string]*dag.MessageMeta, error) {
	prefix := logPrefix(sessionID)
	entries, err := m.storage.ListEntry(ctx, prefix)
	if err != nil {
		return nil, err
	}

	out := make(map[string]*dag.MessageMeta, len(entries))
	for _, e := range entries {
		if !strings.HasSuffix(e, ".json") {
			continue
		}

		var key string
		if strings.HasPrefix(e, prefix) {
			key = e
		} else {
			key = prefix + e
		}

		meta := &dag.MessageMeta{}
		if err := m.storage.ReadJson(ctx, key, meta); err != nil {
			continue // corrupt sidecar; skip rather than poisoning the map
		}

		if meta.Hash != "" {
			out[meta.Hash] = meta
		}
	}

	return out, nil
}

func (m *DagStore) findMeta(ctx context.Context, sessionID, hash string) (*dag.MessageMeta, error) {
	all, err := m.loadAllMetas(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return all[hash], nil
}

func (m *DagStore) ResolvePrefix(ctx context.Context, sessionID, prefix string) (string, error) {
	if len(prefix) == 64 {
		return prefix, nil
	}

	if len(prefix) < 4 {
		return "", fmt.Errorf("hash prefix %q too short (min 4)", prefix)
	}

	all, err := m.loadAllMetas(ctx, sessionID)
	if err != nil {
		return "", err
	}

	var matches []string
	for hash := range all {
		if strings.HasPrefix(hash, prefix) {
			matches = append(matches, hash)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no message matches prefix %q", prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous prefix %q: %d matches", prefix, len(matches))
	}
}

func toDagMessageObj(m *Message) *dag.MessageObj {
	tcs := make([]dag.MessageToolCall, len(m.ToolCalls))
	for i, t := range m.ToolCalls {
		tcs[i] = dag.MessageToolCall{
			ID:        t.ID,
			Name:      t.Name,
			Arguments: t.Arguments,
			Result:    t.Result,
			IsError:   t.IsError,
		}
	}

	return &dag.MessageObj{
		Role:       m.Role,
		Content:    m.Content,
		ToolCalls:  tcs,
		ParentHash: m.ParentHash,
	}
}

func fromDagMessageObj(hash string, obj *dag.MessageObj, meta *dag.MessageMeta) *Message {
	tcs := make([]MessageToolCall, len(obj.ToolCalls))
	for i, t := range obj.ToolCalls {
		tcs[i] = MessageToolCall{
			ID:        t.ID,
			Name:      t.Name,
			Arguments: t.Arguments,
			Result:    t.Result,
			IsError:   t.IsError,
		}
	}

	out := &Message{
		Hash:       hash,
		ParentHash: obj.ParentHash,
		Role:       obj.Role,
		Content:    obj.Content,
		ToolCalls:  tcs,
	}
	if meta != nil {
		out.CreatedAt = meta.CreatedAt
		out.ContextHash = meta.ContextHash
		out.Usage = meta.Usage
	}

	return out
}

