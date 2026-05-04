package session

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/internal/service/session/dag"
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
//	@Summary		List session branches
//	@Description	Returns all refs (HEAD + branches) for a session as a name -> hash map.
//	@Tags			sessions
//	@Produce		json
//	@Param			session_id	path		string	true	"Session ID"
//	@Success		200			{object}	map[string]any
//	@Failure		404			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/branch [get]
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
		symrefs := map[string]string{}
		if d, ok := s.store.(*dagSessionStore); ok {
			for name := range refs {
				if target, isSym, _ := d.refs.ReadSymRef(ctx, meta.ID, name); isSym {
					symrefs[name] = target
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"refs": refs, "symrefs": symrefs})
	}
}

// handleCreateRef godoc
//
//	@Summary		Create session branch
//	@Description	Creates a named branch pointing at a message hash. Fails with 409 when the ref already exists.
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			session_id	path		string				true	"Session ID"
//	@Param			body		body		refCreateRequest	true	"Branch to create"
//	@Success		201			{object}	map[string]string
//	@Failure		400			{object}	map[string]string
//	@Failure		409			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/branch [post]
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
//	@Summary		Move or rename branch
//	@Description	Atomically advances or renames a branch. Set "hash" to move (CAS when "expected_hash" is also set). Set "name" to rename — returns 409 if the target name already exists.
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			session_id	path		string				true	"Session ID"
//	@Param			ref			path		string				true	"Current ref name"
//	@Param			body		body		refUpdateRequest	true	"Move or rename payload"
//	@Success		200			{object}	map[string]string
//	@Failure		400			{object}	map[string]string
//	@Failure		409			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/branch/{ref} [patch]
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
//	@Summary		Delete session branch
//	@Description	Removes a branch ref. Missing refs are not an error.
//	@Tags			sessions
//	@Param			session_id	path	string	true	"Session ID"
//	@Param			ref			path	string	true	"Ref name"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/branch/{ref} [delete]
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
