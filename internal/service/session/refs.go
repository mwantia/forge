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

type refUpdateRequest struct {
	Hash         string `json:"hash"          binding:"required"`
	ExpectedHash string `json:"expected_hash"`
}

// handleListRefs godoc
//
//	@Summary		List session refs
//	@Description	Returns all refs (HEAD + branches + tags) for a session as a name -> hash map.
//	@Tags			sessions
//	@Produce		json
//	@Param			session_id	path		string	true	"Session ID"
//	@Success		200			{object}	map[string]any
//	@Failure		404			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/refs [get]
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
		c.JSON(http.StatusOK, gin.H{"refs": refs})
	}
}

// handleCreateRef godoc
//
//	@Summary		Create session ref
//	@Description	Creates a named branch or tag pointing at a message hash. Fails with 409 when the ref already exists.
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			session_id	path		string				true	"Session ID"
//	@Param			body		body		refCreateRequest	true	"Ref to create"
//	@Success		201			{object}	map[string]string
//	@Failure		400			{object}	map[string]string
//	@Failure		409			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/refs [post]
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
//	@Summary		Move ref (CAS)
//	@Description	Atomically advances a ref. When expected_hash is set the move only succeeds if the ref currently equals it; on mismatch returns 409 with the actual current value.
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			session_id	path		string				true	"Session ID"
//	@Param			ref			path		string				true	"Ref name"
//	@Param			body		body		refUpdateRequest	true	"New hash + optional expected"
//	@Success		200			{object}	map[string]string
//	@Failure		400			{object}	map[string]string
//	@Failure		409			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/refs/{ref} [patch]
func (s *SessionService) handleUpdateRef() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req refUpdateRequest
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
		name := c.Param("ref")

		if req.ExpectedHash == "" {
			// Force move.
			if err := s.WriteRef(ctx, meta.ID, name, req.Hash); err != nil {
				if errors.Is(err, ErrSessionArchived) {
					c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"name": name, "hash": req.Hash})
			return
		}
		if err := s.CASRef(ctx, meta.ID, name, req.ExpectedHash, req.Hash); err != nil {
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
		c.JSON(http.StatusOK, gin.H{"name": name, "hash": req.Hash})
	}
}

// handleDeleteRef godoc
//
//	@Summary		Delete session ref
//	@Description	Removes a ref. Missing refs are not an error. Deleting HEAD is allowed but should generally be avoided.
//	@Tags			sessions
//	@Param			session_id	path	string	true	"Session ID"
//	@Param			ref			path	string	true	"Ref name"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/refs/{ref} [delete]
func (s *SessionService) handleDeleteRef() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		meta, err := s.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := s.DeleteRef(ctx, meta.ID, c.Param("ref")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
