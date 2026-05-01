package session

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/internal/service/template"
)

type createSessionRequest struct {
	Model       string `json:"model"          binding:"required"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Parent      string `json:"parent"`
	System      string `json:"system"`
}

type compactResult struct {
	Before  int `json:"before"`
	After   int `json:"after"`
	Deleted int `json:"deleted"`
}

// handleListSessions godoc
//
//	@Summary		List sessions
//	@Description	Returns all sessions, optionally filtered by parent ID
//	@Tags			sessions
//	@Produce		json
//	@Param			offset	query		int		false	"Pagination offset"
//	@Param			limit	query		int		false	"Max results (default 20)"
//	@Param			parent	query		string	false	"Filter by parent session ID"
//	@Success		200		{object}	map[string][]SessionMetadata
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions [get]
func (s *SessionService) handleListSessions() gin.HandlerFunc {
	return func(c *gin.Context) {
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		parentID := c.Query("parent")

		s.mu.RLock()
		defer s.mu.RUnlock()

		sessions, err := s.store.ListParentSessions(c.Request.Context(), parentID, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"sessions": sessions})
	}
}

// handleCreateSession godoc
//
//	@Summary		Create session
//	@Description	Creates a new session
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			body	body		createSessionRequest	true	"Session options"
//	@Success		201		{object}	SessionMetadata
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions [post]
func (s *SessionService) handleCreateSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createSessionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		name := req.Name
		if name == "" {
			name = template.GenerateUniqueName()
		}

		system := req.System
		if system == "" {
			system = s.config.DefaultSystem
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		// Name uniqueness within deployment (docs/03 §1.5). Conflict = 409.
		if existing, err := s.store.FindSessionByName(c.Request.Context(), name); err == nil && existing != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "session name already exists: " + name})
			return
		}

		now := time.Now()
		meta := &SessionMetadata{
			ID:          DeriveSessionID(name, now.UnixNano(), req.Parent),
			Name:        name,
			Title:       req.Title,
			Description: req.Description,
			Parent:      req.Parent,
			Model:       req.Model,
			System:      system,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := s.store.SaveSession(c.Request.Context(), meta); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		SessionsTotal.WithLabelValues("create").Inc()
		c.JSON(http.StatusCreated, meta)
	}
}

// handleGetSession godoc
//
//	@Summary		Get session
//	@Description	Returns metadata for a single session by ID or name
//	@Tags			sessions
//	@Produce		json
//	@Param			session_id	path		string	true	"Session ID or name"
//	@Success		200	{object}	SessionMetadata
//	@Failure		404	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id} [get]
func (s *SessionService) handleGetSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("session_id")

		s.mu.RLock()
		defer s.mu.RUnlock()

		meta, err := s.resolveSessionLocked(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, meta)
	}
}

// handleDeleteSession godoc
//
//	@Summary		Delete session
//	@Description	Deletes a session and all its messages
//	@Tags			sessions
//	@Param			session_id	path	string	true	"Session ID"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id} [delete]
func (s *SessionService) handleDeleteSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("session_id")

		s.mu.Lock()
		defer s.mu.Unlock()

		meta, err := s.resolveSessionLocked(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := s.store.DeleteSession(c.Request.Context(), meta.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		SessionsTotal.WithLabelValues("delete").Inc()
		c.Status(http.StatusNoContent)
	}
}

// handleListMessages godoc
//
//	@Summary		List messages
//	@Description	Returns messages for a session in chronological order
//	@Tags			sessions
//	@Produce		json
//	@Param			session_id		path		string	true	"Session ID"
//	@Param			offset	query		int		false	"Pagination offset"
//	@Param			limit	query		int		false	"Max results (default 50)"
//	@Success		200		{object}	map[string][]Message
//	@Failure		404		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/messages [get]
func (s *SessionService) handleListMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("session_id")
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

		s.mu.RLock()
		defer s.mu.RUnlock()

		meta, err := s.resolveSessionLocked(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		messages, err := s.store.ListMessages(c.Request.Context(), meta.ID, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"messages": messages})
	}
}

// handleGetMessage godoc
//
//	@Summary		Get message
//	@Description	Returns a single message by ID
//	@Tags			sessions
//	@Produce		json
//	@Param			session_id	path		string	true	"Session ID"
//	@Param			msg_id		path		string	true	"Message ID"
//	@Success		200			{object}	Message
//	@Failure		404			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/messages/{msg_id} [get]
func (s *SessionService) handleGetMessage() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("session_id")
		msgID := c.Param("msg_id")

		s.mu.RLock()
		defer s.mu.RUnlock()

		meta, err := s.resolveSessionLocked(c.Request.Context(), sessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		msg, err := s.store.LoadMessage(c.Request.Context(), meta.ID, msgID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, msg)
	}
}

// handleCompactMessages godoc
//
//	@Summary		Compact messages
//	@Description	Removes intermediate tool call and tool result messages to reduce context size
//	@Tags			sessions
//	@Produce		json
//	@Param			session_id	path		string	true	"Session ID"
//	@Success		200	{object}	compactResult
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/messages/compact [patch]
func (s *SessionService) handleCompactMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("session_id")

		s.mu.Lock()
		defer s.mu.Unlock()

		meta, err := s.resolveSessionLocked(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()
		before, _ := s.store.CountMessages(ctx, meta.ID)

		deleted, err := s.store.CompactToolsMessages(ctx, meta.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		after, _ := s.store.CountMessages(ctx, meta.ID)
		c.JSON(http.StatusOK, compactResult{Before: before, After: after, Deleted: deleted})
	}
}

// handleSummarizeMessages godoc
//
//	@Summary		Summarize messages
//	@Description	Summarizes the session history into a compressed context message (not yet implemented)
//	@Tags			sessions
//	@Produce		json
//	@Param			id	path		string	true	"Session ID"
//	@Failure		501	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{id}/messages/summarize [patch]
func (s *SessionService) handleSummarizeMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "summarize not yet implemented"})
	}
}
