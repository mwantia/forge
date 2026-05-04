package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mwantia/forge/internal/service/session/dag"
)

// ErrSessionArchived is returned by mutating operations when the target
// session has been archived. Surfaces as 409 at the HTTP layer.
var ErrSessionArchived = errors.New("session is archived (immutable)")

// ArchivePath is the resource path where archive envelopes are stored.
const ArchivePath = "/archives"

// ArchiveEnvelope is the schema-versioned wire form persisted in the resource
// store and replayed on clone. Locked against docs/03 §7.1.
type ArchiveEnvelope struct {
	SchemaVersion int                  `json:"schema_version"`
	SessionID     string               `json:"session_id"`
	Name          string               `json:"name"`
	RefName       string               `json:"ref_name"`
	HeadHash      string               `json:"head_hash"`
	Messages      []ArchiveMessage     `json:"messages"`
	ContextHashes []string             `json:"context_hashes"`
	Metadata      *SessionMetadata     `json:"metadata"`
}

type ArchiveMessage struct {
	Hash       string                `json:"hash"`
	Role       string                `json:"role"`
	Content    string                `json:"content,omitempty"`
	ToolCalls  []dag.MessageToolCall `json:"tool_calls,omitempty"`
	ParentHash string                `json:"parent_hash,omitempty"`
}

// ArchiveResult is the response payload for a successful archive operation.
type ArchiveResult struct {
	SessionID  string    `json:"session_id"`
	RefName    string    `json:"ref_name"`
	HeadHash   string    `json:"head_hash"`
	ResourceID string    `json:"resource_id"`
	Path       string    `json:"path"`
	ArchivedAt time.Time `json:"archived_at"`
	Messages   int       `json:"messages"`
}

// ensureNotArchived returns ErrSessionArchived if the session is archived.
// Mutation paths (AppendMessage*, WriteRef, CASRef, DeleteRef, SaveSession
// for non-archive fields) call this before touching state.
func (s *SessionService) ensureNotArchived(ctx context.Context, sessionID string) error {
	meta, err := s.store.LoadSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if meta.ArchivedAt != nil {
		return ErrSessionArchived
	}
	return nil
}

// ArchiveSession walks ref (default HEAD) to root, builds an envelope, and
// stores it through the resource layer. On success the session is flipped
// to immutable and the resource ID is recorded on the metadata.
func (s *SessionService) ArchiveSession(ctx context.Context, sessionID, refName string) (*ArchiveResult, error) {
	if s.resources == nil {
		return nil, fmt.Errorf("resource service unavailable; cannot archive")
	}
	if refName == "" {
		refName = dag.HEAD
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.store.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if meta.ArchivedAt != nil {
		return nil, ErrSessionArchived
	}

	d, ok := s.store.(*dagSessionStore)
	if !ok {
		return nil, fmt.Errorf("dag store unavailable")
	}

	tip, err := d.refs.Read(ctx, sessionID, refName)
	if err != nil {
		if errors.Is(err, dag.ErrNotFound) {
			return nil, fmt.Errorf("ref %q does not exist", refName)
		}
		return nil, err
	}
	entries, err := dag.Walk(ctx, d.objects, d.refs, sessionID, tip, 0, 0)
	if err != nil {
		return nil, err
	}

	metaByHash, _ := d.loadAllMetas(ctx, sessionID)
	messages := make([]ArchiveMessage, 0, len(entries))
	contextHashes := make([]string, 0, len(entries))
	seenCtx := make(map[string]struct{})
	for _, e := range entries {
		messages = append(messages, ArchiveMessage{
			Hash:       e.Hash,
			Role:       e.Message.Role,
			Content:    e.Message.Content,
			ToolCalls:  e.Message.ToolCalls,
			ParentHash: e.Message.ParentHash,
		})
		if m := metaByHash[e.Hash]; m != nil && m.ContextHash != "" {
			if _, dup := seenCtx[m.ContextHash]; !dup {
				seenCtx[m.ContextHash] = struct{}{}
				contextHashes = append(contextHashes, m.ContextHash)
			}
		}
	}

	envelope := ArchiveEnvelope{
		SchemaVersion: 1,
		SessionID:     sessionID,
		Name:          meta.Name,
		RefName:       refName,
		HeadHash:      tip,
		Messages:      messages,
		ContextHashes: contextHashes,
		Metadata:      meta,
	}

	now := time.Now()
	hashes := make([]string, len(messages))
	for i, m := range messages {
		hashes[i] = m.Hash
	}
	envBytes, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode envelope: %w", err)
	}
	resMeta := map[string]any{
		"forge_session_id":     sessionID,
		"forge_message_hashes": hashes,
		"archived_at":          now.UTC().Format(time.RFC3339Nano),
		"ref_name":             refName,
		"head_hash":            tip,
	}
	res, err := s.resources.Store(ctx, ArchivePath, string(envBytes), nil, resMeta)
	if err != nil {
		return nil, fmt.Errorf("store archive envelope: %w", err)
	}

	meta.ArchivedAt = &now
	meta.ArchiveResourceID = res.ID
	meta.ArchivePath = ArchivePath
	meta.UpdatedAt = now
	if err := s.store.SaveSession(ctx, meta); err != nil {
		return nil, fmt.Errorf("flip session to archived: %w", err)
	}

	return &ArchiveResult{
		SessionID:  sessionID,
		RefName:    refName,
		HeadHash:   tip,
		ResourceID: res.ID,
		Path:       ArchivePath,
		ArchivedAt: now,
		Messages:   len(messages),
	}, nil
}

// CloneSession resolves an archive (by source session ID, falling back to
// resource ID), replays its envelope into the global object store, and
// creates a fresh live session whose HEAD points at the archived tip.
func (s *SessionService) CloneSession(ctx context.Context, sourceID, name string) (*SessionMetadata, error) {
	if s.resources == nil {
		return nil, fmt.Errorf("resource service unavailable; cannot clone")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	envelope, err := s.loadArchiveEnvelope(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	d, ok := s.store.(*dagSessionStore)
	if !ok {
		return nil, fmt.Errorf("dag store unavailable")
	}

	// Replay messages — PutMessage is idempotent (PutIfAbsent), so any
	// already-known hashes dedup automatically.
	for _, m := range envelope.Messages {
		obj := &dag.MessageObj{
			Role:       m.Role,
			Content:    m.Content,
			ToolCalls:  m.ToolCalls,
			ParentHash: m.ParentHash,
		}
		hash, err := d.objects.PutMessage(ctx, obj)
		if err != nil {
			return nil, fmt.Errorf("replay message %s: %w", m.Hash, err)
		}
		if hash != m.Hash {
			return nil, fmt.Errorf("hash mismatch on replay: archived %s, recomputed %s", m.Hash, hash)
		}
	}

	now := time.Now()
	cloneName := strings.TrimSpace(name)
	if cloneName == "" {
		cloneName = uniqueCloneName(ctx, s.store, envelope.Name)
	}
	if existing, _ := s.store.FindSessionByName(ctx, cloneName); existing != nil {
		return nil, fmt.Errorf("session name already exists: %s", cloneName)
	}

	src := envelope.Metadata
	clone := &SessionMetadata{
		ID:          DeriveSessionID(cloneName, now.UnixNano(), envelope.SessionID),
		Name:        cloneName,
		Title:       src.Title,
		Description: src.Description,
		Parent:      envelope.SessionID,
		Model:       src.Model,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.SaveSession(ctx, clone); err != nil {
		return nil, fmt.Errorf("save clone: %w", err)
	}

	// Replay the message log so chronological listing works on the clone.
	// CreatedAt is now (replay time); ContextHash is preserved when we have
	// it on the archived envelope, but the per-message link is lost — only
	// ContextHashes en masse survive.
	for _, m := range envelope.Messages {
		mm := &dag.MessageMeta{
			Hash:      m.Hash,
			SessionID: clone.ID,
			CreatedAt: now,
		}
		if err := d.writeLogEntry(ctx, clone.ID, mm); err != nil {
			return nil, fmt.Errorf("write clone log %s: %w", m.Hash, err)
		}
	}

	if err := d.refs.Write(ctx, clone.ID, dag.MAIN, envelope.HeadHash); err != nil {
		return nil, fmt.Errorf("write clone main: %w", err)
	}
	if err := d.refs.WriteSymRef(ctx, clone.ID, dag.HEAD, dag.MAIN); err != nil {
		return nil, fmt.Errorf("write clone HEAD symref: %w", err)
	}

	SessionsTotal.WithLabelValues("clone").Inc()
	return clone, nil
}

// loadArchiveEnvelope tries (in order): a live archived session by ID
// (reads back its recorded resource ID), then a direct resource ID lookup
// at the archives path.
func (s *SessionService) loadArchiveEnvelope(ctx context.Context, sourceID string) (*ArchiveEnvelope, error) {
	if meta, err := s.store.LoadSession(ctx, sourceID); err == nil && meta.ArchivedAt != nil && meta.ArchiveResourceID != "" {
		return s.fetchEnvelope(ctx, meta.ArchivePath, meta.ArchiveResourceID)
	}
	return s.fetchEnvelope(ctx, ArchivePath, sourceID)
}

func (s *SessionService) fetchEnvelope(ctx context.Context, path, id string) (*ArchiveEnvelope, error) {
	if path == "" {
		path = ArchivePath
	}
	res, err := s.resources.Get(ctx, path, id)
	if err != nil {
		return nil, fmt.Errorf("fetch archive %s/%s: %w", path, id, err)
	}
	env := &ArchiveEnvelope{}
	if err := json.Unmarshal([]byte(res.Content), env); err != nil {
		return nil, fmt.Errorf("decode archive envelope: %w", err)
	}
	if env.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported archive schema_version %d", env.SchemaVersion)
	}
	return env, nil
}

func uniqueCloneName(ctx context.Context, store sessionBackend, base string) string {
	if base == "" {
		base = "session"
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-clone-%d", base, i)
		if existing, _ := store.FindSessionByName(ctx, candidate); existing == nil {
			return candidate
		}
	}
}
