package system

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge-sdk/pkg/contenthash"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
)

// dagObjectKey wraps dag.ObjectKey for use within this package.
func dagObjectKey(hash string) string { return dag.ObjectKey(hash) }

// --- object cat / type ---

// handleDagCat godoc
//
//	@Description	Returns the raw canonical JSON for a content-addressed object. ?pretty=true indents the output. Prefix matching (>=4 hex chars) resolved server-side. Sets X-Forge-Object-Type header.
func (s *SystemService) handleDagCat() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		objects := dag.NewObjectStore(s.storage)

		hash, err := s.resolveDagPrefix(ctx, objects, c.Param("hash"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		raw, err := objects.GetRaw(ctx, hash)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "object not found: " + hash})
			return
		}

		objType := detectObjectType(raw)
		c.Header("X-Forge-Object-Type", objType)

		if c.Query("pretty") == "true" {
			var buf bytes.Buffer
			if err := json.Indent(&buf, raw, "", "  "); err == nil {
				raw = buf.Bytes()
			}
		}

		c.Data(http.StatusOK, "application/json", raw)
	}
}

// handleDagType godoc
//
//	@Description	Returns the type of a DAG object without fetching its full body.
func (s *SystemService) handleDagType() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		objects := dag.NewObjectStore(s.storage)

		hash, err := s.resolveDagPrefix(ctx, objects, c.Param("hash"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		raw, err := objects.GetRaw(ctx, hash)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "object not found: " + hash})
			return
		}

		c.JSON(http.StatusOK, gin.H{"hash": hash, "type": detectObjectType(raw)})
	}
}

// --- log ---

type dagLogEntry struct {
	Hash      string    `json:"hash"`
	ShortHash string    `json:"short_hash"`
	Role      string    `json:"role"`
	Preview   string    `json:"preview"`
	CreatedAt time.Time `json:"created_at"`
}

// handleDagLog godoc
//
//	@Description	Returns NDJSON of {hash, short_hash, role, preview, created_at} walking from the ref tip to the root. Mirrors git log --oneline.
func (s *SystemService) handleDagLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		sessionID := c.Param("id")
		ref := c.DefaultQuery("ref", dag.HEAD)

		objects := dag.NewObjectStore(s.storage)
		refs := dag.NewRefStore(s.storage)

		tip, err := refs.Read(ctx, sessionID, ref)
		if err != nil || tip == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("ref %q not found for session %s", ref, sessionID)})
			return
		}

		entries, err := dag.Walk(ctx, objects, refs, sessionID, tip, 0, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Load sidecars for timestamps.
		metaByHash := s.loadSessionMetas(ctx, sessionID)

		c.Header("Content-Type", "application/x-ndjson")
		c.Status(http.StatusOK)

		enc := json.NewEncoder(c.Writer)
		for i := len(entries) - 1; i >= 0; i-- {
			e := entries[i]
			entry := dagLogEntry{
				Hash:      e.Hash,
				ShortHash: e.Hash[:8],
				Role:      e.Message.Role,
				Preview:   preview(e.Message.Content, 80),
			}
			if m, ok := metaByHash[e.Hash]; ok {
				entry.CreatedAt = m.CreatedAt
			}
			_ = enc.Encode(entry)
		}
	}
}

// --- diff ---

// handleDagDiff godoc
//
//	@Description	Fetches both objects, computes a unified diff of their canonical JSON, and returns it as text/plain.
func (s *SystemService) handleDagDiff() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		hashA, hashB := c.Query("a"), c.Query("b")
		if hashA == "" || hashB == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query params 'a' and 'b' are required"})
			return
		}

		objects := dag.NewObjectStore(s.storage)

		rawA, err := objects.GetRaw(ctx, hashA)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "object a not found: " + hashA})
			return
		}
		rawB, err := objects.GetRaw(ctx, hashB)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "object b not found: " + hashB})
			return
		}

		diff := unifiedDiff(hashA, hashB, prettyJSON(rawA), prettyJSON(rawB))
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(diff))
	}
}

// --- verify ---

type verifyRequest struct {
	SessionID string `json:"session_id"`
	Ref       string `json:"ref"`
	All       bool   `json:"all"`
}

type verifyResult struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors"`
}

// handleDagVerify godoc
//
//	@Description	Walks reachable objects from the given ref, re-hashes each blob, and reports mismatches or missing parents.
func (s *SystemService) handleDagVerify() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req verifyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Ref == "" {
			req.Ref = dag.HEAD
		}

		ctx := c.Request.Context()
		objects := dag.NewObjectStore(s.storage)
		refs := dag.NewRefStore(s.storage)

		var sessions []string
		if req.All {
			entries, err := s.storage.ListEntry(ctx, "sessions/")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			for _, e := range entries {
				if strings.HasSuffix(e, "/") {
					sessions = append(sessions, strings.TrimSuffix(e, "/"))
				}
			}
		} else {
			sessions = []string{req.SessionID}
		}

		var errs []string
		for _, sessionID := range sessions {
			tip, err := refs.Read(ctx, sessionID, req.Ref)
			if err != nil || tip == "" {
				continue
			}
			entries, err := dag.Walk(ctx, objects, refs, sessionID, tip, 0, 0)
			if err != nil {
				errs = append(errs, fmt.Sprintf("session %s: walk error: %v", sessionID, err))
				continue
			}
			for _, e := range entries {
				raw, err := objects.GetRaw(ctx, e.Hash)
				if err != nil {
					errs = append(errs, fmt.Sprintf("%s: missing object", e.Hash[:8]))
					continue
				}
				canonical, err := contenthash.Canonical(json.RawMessage(raw))
				if err != nil {
					errs = append(errs, fmt.Sprintf("%s: cannot re-canonicalize: %v", e.Hash[:8], err))
					continue
				}
				got := contenthash.HashBytes(canonical)
				if got != e.Hash {
					errs = append(errs, fmt.Sprintf("%s: hash mismatch (got %s)", e.Hash[:8], got[:8]))
				}
			}
		}

		result := verifyResult{OK: len(errs) == 0, Errors: errs}
		status := http.StatusOK
		if !result.OK {
			status = http.StatusUnprocessableEntity
		}
		c.JSON(status, result)
	}
}

// --- objects count/list ---

// handleDagObjects godoc
//
//	@Description	?list=false (default) returns {"count":N}. ?list=true streams NDJSON of {hash, shard}. ?prefix=<xx> filters by 2-char shard prefix.
func (s *SystemService) handleDagObjects() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		prefix := c.Query("prefix")
		list := c.Query("list") == "true"

		all, err := s.listAllObjects(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		type entry struct {
			Hash  string `json:"hash"`
			Shard string `json:"shard"`
		}

		var filtered []entry
		for hash := range all {
			if prefix != "" && !strings.HasPrefix(hash, prefix) {
				continue
			}
			filtered = append(filtered, entry{Hash: hash, Shard: hash[:2]})
		}

		if !list {
			c.JSON(http.StatusOK, gin.H{"count": len(filtered)})
			return
		}

		c.Header("Content-Type", "application/x-ndjson")
		c.Status(http.StatusOK)
		enc := json.NewEncoder(c.Writer)
		for _, e := range filtered {
			_ = enc.Encode(e)
		}
	}
}

// --- GC with dry-run ---

// handleDagGC godoc
//
//	@Description	Walks every session ref, marks reachable objects, and deletes (or reports) the rest. ?dry_run=true returns stats without deleting.
func (s *SystemService) handleDagGC() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		dryRun := c.Query("dry_run") == "true"

		all, err := s.listAllObjects(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "listing objects: " + err.Error()})
			return
		}
		reachable, err := s.collectReachable(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "marking reachable: " + err.Error()})
			return
		}

		swept := 0
		for hash := range all {
			if _, ok := reachable[hash]; ok {
				continue
			}
			if dryRun {
				swept++
				continue
			}
			if err := s.storage.DeleteEntry(ctx, dag.ObjectKey(hash)); err != nil {
				s.logger.Warn("dag-gc: failed to delete object", "hash", hash, "err", err)
				continue
			}
			swept++
		}

		result := gcResult{
			Total: len(all),
			Kept:  len(all) - swept,
			Swept: swept,
		}
		if !dryRun {
			s.logger.Info("dag-gc complete", "total", result.Total, "kept", result.Kept, "swept", result.Swept)
		}
		c.JSON(http.StatusOK, result)
	}
}

// --- helpers ---

func (s *SystemService) resolveDagPrefix(ctx context.Context, objects *dag.ObjectStore, hashOrPrefix string) (string, error) {
	if len(hashOrPrefix) == 64 {
		return hashOrPrefix, nil
	}
	if len(hashOrPrefix) < 4 {
		return "", fmt.Errorf("hash prefix %q too short (min 4 chars)", hashOrPrefix)
	}

	shard := hashOrPrefix[:2]
	entries, err := s.storage.ListEntry(ctx, fmt.Sprintf("objects/%s/", shard))
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("no object matches prefix %q", hashOrPrefix)
	}

	rest := hashOrPrefix[2:]
	var matches []string
	for _, e := range entries {
		if strings.HasPrefix(e, rest) {
			matches = append(matches, shard+strings.TrimSuffix(e, "/"))
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no object matches prefix %q", hashOrPrefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous prefix %q: %d matches", hashOrPrefix, len(matches))
	}
}

func (s *SystemService) loadSessionMetas(ctx context.Context, sessionID string) map[string]*dag.MessageMeta {
	prefix := fmt.Sprintf("sessions/%s/log/", sessionID)
	entries, _ := s.storage.ListEntry(ctx, prefix)
	out := make(map[string]*dag.MessageMeta, len(entries))
	for _, e := range entries {
		if !strings.HasSuffix(e, ".json") {
			continue
		}
		var m dag.MessageMeta
		if err := s.storage.ReadJson(ctx, prefix+e, &m); err == nil && m.Hash != "" {
			out[m.Hash] = &m
		}
	}
	return out
}

func detectObjectType(raw []byte) string {
	var probe struct {
		Role          string `json:"role"`
		Provider      string `json:"provider"`
		MessageHashes []any  `json:"message_hashes"`
		Tools         []any  `json:"tools"`
	}
	if json.Unmarshal(raw, &probe) != nil {
		return "unknown"
	}
	switch {
	case probe.Role != "":
		return "message"
	case probe.Provider != "" || probe.MessageHashes != nil:
		return "prompt_context"
	case probe.Tools != nil:
		return "tool_catalog"
	default:
		return "unknown"
	}
}

func prettyJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}

func unifiedDiff(nameA, nameB, textA, textB string) string {
	linesA := strings.Split(textA, "\n")
	linesB := strings.Split(textB, "\n")

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n", nameA[:8], nameB[:8])

	// Simple line-by-line diff: emit context of removed/added lines.
	maxLen := len(linesA)
	if len(linesB) > maxLen {
		maxLen = len(linesB)
	}
	for i := 0; i < maxLen; i++ {
		var a, b string
		if i < len(linesA) {
			a = linesA[i]
		}
		if i < len(linesB) {
			b = linesB[i]
		}
		if a == b {
			fmt.Fprintf(&sb, " %s\n", a)
		} else {
			if i < len(linesA) {
				fmt.Fprintf(&sb, "-%s\n", a)
			}
			if i < len(linesB) {
				fmt.Fprintf(&sb, "+%s\n", b)
			}
		}
	}
	return sb.String()
}

func preview(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
