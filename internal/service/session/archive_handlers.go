package session

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type archiveRequest struct {
	Ref string `json:"ref"`
}

type cloneRequest struct {
	Name string `json:"name"`
}

// handleArchiveSession godoc
//
//	@Summary		Archive session
//	@Description	Walks the named ref (default HEAD), stores an envelope through the resource layer, and flips the session to immutable.
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			session_id	path		string			true	"Session ID or name"
//	@Param			body		body		archiveRequest	false	"Archive options"
//	@Success		200			{object}	ArchiveResult
//	@Failure		404			{object}	map[string]string
//	@Failure		409			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/archive [post]
func (s *SessionService) handleArchiveSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req archiveRequest
		_ = c.ShouldBindJSON(&req)

		s.mu.RLock()
		meta, err := s.resolveSessionLocked(c.Request.Context(), c.Param("session_id"))
		s.mu.RUnlock()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		res, err := s.ArchiveSession(c.Request.Context(), meta.ID, req.Ref)
		if err != nil {
			if errors.Is(err, ErrSessionArchived) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		SessionsTotal.WithLabelValues("archive").Inc()
		c.JSON(http.StatusOK, res)
	}
}

// handleCloneSession godoc
//
//	@Summary		Clone archived session
//	@Description	Replays an archive envelope into a fresh live session whose HEAD points at the archived tip.
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			session_id	path		string			true	"Source session ID, name, or archive resource ID"
//	@Param			body		body		cloneRequest	false	"Clone options"
//	@Success		201			{object}	SessionMetadata
//	@Failure		404			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/clone [post]
func (s *SessionService) handleCloneSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req cloneRequest
		_ = c.ShouldBindJSON(&req)

		sourceID := c.Param("session_id")
		// Try resolve as live session first (allows name lookup); fall through
		// to raw resource ID lookup inside CloneSession.
		s.mu.RLock()
		if meta, err := s.resolveSessionLocked(c.Request.Context(), sourceID); err == nil {
			sourceID = meta.ID
		}
		s.mu.RUnlock()

		clone, err := s.CloneSession(c.Request.Context(), sourceID, req.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, clone)
	}
}
