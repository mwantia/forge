package session

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/internal/service/session/dag"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type refCreateRequest struct {
	Name string `json:"name" binding:"required"`
	Hash string `json:"hash" binding:"required"`
}

// refUpdateRequest handles CAS-move (hash set), rename (name set), or
// checkout (checkout set — rewrites HEAD as a symref to another branch).
type refUpdateRequest struct {
	Hash         string `json:"hash"`
	ExpectedHash string `json:"expected_hash"`
	Name         string `json:"name"`
	Checkout     string `json:"checkout"`
}

// handleListRefs godoc
//
//	@Description	Returns all refs (HEAD + branches) for a session as a name -> hash map.
func (s *SessionService) handleListRefs() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		meta, err := s.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		refs, err := s.ListRefs(ctx, meta.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if prefix := c.Query("prefix"); prefix != "" {
			filtered := make(map[string]string, len(refs))
			for name, hash := range refs {
				if strings.HasPrefix(name, prefix) {
					filtered[name] = hash
				}
			}
			refs = filtered
		}
		symrefs := map[string]string{}
		for name := range refs {
			if target, isSym, _ := s.store.refs.ReadSymRef(ctx, meta.ID, name); isSym {
				symrefs[name] = target
			}
		}
		c.JSON(http.StatusOK, gin.H{"refs": refs, "symrefs": symrefs})
	}
}

// handleCreateRef godoc
//
//	@Description	Creates a named branch pointing at a message hash. Fails with 409 when the ref already exists.
func (s *SessionService) handleCreateRef() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req refCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx := c.Request.Context()
		meta, err := s.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := s.CASRef(ctx, meta.ID, req.Name, "", req.Hash); err != nil {
			if errors.Is(err, ErrSessionArchived) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			var conflict *dag.CASConflict
			if errors.As(err, &conflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "actual": conflict.Actual})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"name": req.Name, "hash": req.Hash})
	}
}

// handleUpdateRef godoc
//
//	@Description	Atomically advances or renames a branch. Set "hash" to move (CAS when "expected_hash" is also set). Set "name" to rename — returns 409 if the target name already exists.
func (s *SessionService) handleUpdateRef() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req refUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Hash == "" && req.Name == "" && req.Checkout == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hash, name, or checkout is required"})
			return
		}

		ctx := c.Request.Context()
		meta, err := s.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		current := c.Param("ref")

		// Checkout: rewrite HEAD as a symref pointing at another branch.
		if req.Checkout != "" {
			if current != dag.HEAD {
				c.JSON(http.StatusBadRequest, gin.H{"error": "checkout is only valid on HEAD"})
				return
			}
			if req.Checkout == dag.HEAD {
				c.JSON(http.StatusBadRequest, gin.H{"error": "HEAD is not a branch; choose a named branch (e.g. main)"})
				return
			}
			if err := s.CheckoutRef(ctx, meta.ID, req.Checkout); err != nil {
				var conflict *dag.CASConflict
				if errors.As(err, &conflict) || errors.Is(err, ErrSessionArchived) {
					c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"head": req.Checkout})
			return
		}

		// Guard HEAD against rename.
		if req.Name != "" && current == dag.HEAD {
			c.JSON(http.StatusBadRequest, gin.H{"error": "HEAD cannot be renamed; use checkout to switch branches"})
			return
		}

		// Rename operation: move ref to a new name, preserving the hash.
		if req.Name != "" && req.Hash == "" {
			if err := s.RenameRef(ctx, meta.ID, current, req.Name); err != nil {
				if errors.Is(err, ErrSessionArchived) {
					c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
					return
				}
				var conflict *dag.CASConflict
				if errors.As(err, &conflict) {
					c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"name": req.Name})
			return
		}

		// CAS-move or force-move.
		if req.ExpectedHash == "" {
			if err := s.WriteRef(ctx, meta.ID, current, req.Hash); err != nil {
				if errors.Is(err, ErrSessionArchived) {
					c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"name": current, "hash": req.Hash})
			return
		}
		if err := s.CASRef(ctx, meta.ID, current, req.ExpectedHash, req.Hash); err != nil {
			if errors.Is(err, ErrSessionArchived) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			var conflict *dag.CASConflict
			if errors.As(err, &conflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "actual": conflict.Actual})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": current, "hash": req.Hash})
	}
}

// handleDeleteRef godoc
//
//	@Description	Removes a branch ref. Missing refs are not an error.
func (s *SessionService) handleDeleteRef() gin.HandlerFunc {
	return func(c *gin.Context) {
		ref := c.Param("ref")
		if ref == dag.HEAD {
			c.JSON(http.StatusBadRequest, gin.H{"error": "HEAD cannot be deleted"})
			return
		}
		ctx := c.Request.Context()
		meta, err := s.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := s.DeleteRef(ctx, meta.ID, ref); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// handleRevertRef godoc
//
//	@Description	CAS-moves the named ref to the target hash, orphaning messages between the current tip and the target. Prefer fork_from on /pipeline/commit for non-destructive branching.
func (s *SessionService) handleRevertRef() gin.HandlerFunc {
	return func(c *gin.Context) {
		ref := c.Param("ref")
		ctx := c.Request.Context()

		var req struct {
			To string `json:"to" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		meta, err := s.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		current, err := s.ReadRef(ctx, meta.ID, ref)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := s.CASRef(ctx, meta.ID, ref, current, req.To); err != nil {
			var cas *dag.CASConflict
			if errors.As(err, &cas) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// handleMessageDiff godoc
//
//	@Description	Returns a Myers diff on the Content field of two message objects identified by their content hashes.
func (s *SessionService) handleMessageDiff() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("session_id")
		hashA := c.Param("msg_id")
		hashB := c.Query("to")
		if hashB == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing required query parameter: to"})
			return
		}
		ctx := c.Request.Context()

		s.mu.RLock()
		meta, err := s.resolveSessionLocked(ctx, sessionID)
		s.mu.RUnlock()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		msgA, err := s.store.LoadMessage(ctx, meta.ID, hashA)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "message a: " + err.Error()})
			return
		}
		msgB, err := s.store.LoadMessage(ctx, meta.ID, hashB)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "message b: " + err.Error()})
			return
		}

		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(msgA.Content, msgB.Content, false)
		dmp.DiffCleanupSemantic(diffs)

		c.JSON(http.StatusOK, gin.H{
			"hash_a": msgA.Hash,
			"hash_b": msgB.Hash,
			"patch":  dmp.DiffToDelta(diffs),
			"text":   dmp.DiffPrettyText(diffs),
		})
	}
}
