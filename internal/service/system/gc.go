package system

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/internal/service/session/dag"
)

type gcResult struct {
	Total int `json:"total"`
	Kept  int `json:"kept"`
	Swept int `json:"swept"`
}

// handleGC godoc
//
//	@Summary		Garbage-collect unreachable objects
//	@Description	Walks every session ref, marks reachable objects, and deletes everything else from the object store.
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Failure		500	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/system/gc [post]
func (s *SystemService) handleGC() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

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
			if err := s.storage.DeleteEntry(ctx, dag.ObjectKey(hash)); err != nil {
				s.logger.Warn("gc: failed to delete object", "hash", hash, "err", err)
				continue
			}
			swept++
		}

		result := gcResult{
			Total: len(all),
			Kept:  len(all) - swept,
			Swept: swept,
		}
		s.logger.Info("gc complete", "total", result.Total, "kept", result.Kept, "swept", result.Swept)
		c.JSON(http.StatusOK, result)
	}
}

func (s *SystemService) listAllObjects(ctx context.Context) (map[string]struct{}, error) {
	all := make(map[string]struct{})

	prefixes, err := s.storage.ListEntry(ctx, "objects/")
	if err != nil {
		return nil, err
	}
	for _, p := range prefixes {
		prefix := strings.TrimSuffix(p, "/")
		if prefix == p {
			continue // not a directory entry
		}
		entries, err := s.storage.ListEntry(ctx, fmt.Sprintf("objects/%s/", prefix))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasSuffix(e, "/") {
				continue
			}
			hash := prefix + e
			if len(hash) == 64 {
				all[hash] = struct{}{}
			}
		}
	}
	return all, nil
}

func (s *SystemService) collectReachable(ctx context.Context) (map[string]struct{}, error) {
	reachable := make(map[string]struct{})

	objects := dag.NewObjectStore(s.storage)
	refs := dag.NewRefStore(s.storage)

	sessionEntries, err := s.storage.ListEntry(ctx, "sessions/")
	if err != nil {
		return nil, err
	}

	for _, e := range sessionEntries {
		sessionID := strings.TrimSuffix(e, "/")
		if sessionID == e {
			continue
		}

		allRefs, err := refs.List(ctx, sessionID)
		if err != nil {
			continue
		}

		// Walk every ref chain and mark message objects reachable.
		seen := make(map[string]bool)
		for _, hash := range allRefs {
			if hash == "" || seen[hash] {
				continue
			}
			seen[hash] = true
			cur := hash
			for cur != "" {
				if _, ok := reachable[cur]; ok {
					break
				}
				reachable[cur] = struct{}{}
				msg, err := objects.GetMessage(ctx, cur)
				if err != nil {
					break
				}
				cur = msg.ParentHash
			}
		}

		// Scan log sidecars to collect ContextHash for reachable messages.
		logEntries, err := s.storage.ListEntry(ctx, fmt.Sprintf("sessions/%s/log/", sessionID))
		if err != nil {
			continue
		}
		for _, logFile := range logEntries {
			if strings.HasSuffix(logFile, "/") {
				continue
			}
			var meta dag.MessageMeta
			key := fmt.Sprintf("sessions/%s/log/%s", sessionID, logFile)
			if err := s.storage.ReadJson(ctx, key, &meta); err != nil {
				continue
			}
			if _, ok := reachable[meta.Hash]; !ok {
				continue
			}
			if meta.ContextHash == "" {
				continue
			}
			reachable[meta.ContextHash] = struct{}{}

			// Mark the ToolCatalog and message hashes referenced by the PromptContext.
			pc, err := objects.GetPromptContext(ctx, meta.ContextHash)
			if err != nil {
				continue
			}
			if pc.ToolCatalogHash != "" {
				reachable[pc.ToolCatalogHash] = struct{}{}
			}
			for _, mh := range pc.MessageHashes {
				reachable[mh] = struct{}{}
			}
		}
	}

	return reachable, nil
}
