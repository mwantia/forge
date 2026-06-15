package session

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
)

type createSessionRequest struct {
	Model       string   `json:"model"           binding:"required"`
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Parent      string   `json:"parent"`
	Plugins     []string `json:"plugins"`
	Template    string   `json:"template"`
}

type compactResult struct {
	Before  int `json:"before"`
	After   int `json:"after"`
	Deleted int `json:"deleted"`
}

// handleListSessions godoc
//
//	@Description	Returns sessions matching the given query. Accepts ?search=, ?archived=true|false, ?plugins=a,b, ?parent=, ?offset=, ?limit=.
func (s *SessionService) handleListSessions() gin.HandlerFunc {
	return func(c *gin.Context) {
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

		q := SessionQuery{
			ParentID: c.Query("parent"),
			Search:   c.Query("search"),
			Offset:   offset,
			Limit:    limit,
		}
		if a := c.Query("archived"); a != "" {
			v := a == "true"
			q.Archived = &v
		}
		if p := c.Query("plugins"); p != "" {
			for _, part := range strings.Split(p, ",") {
				if t := strings.TrimSpace(part); t != "" {
					q.Plugins = append(q.Plugins, t)
				}
			}
		}

		s.mu.RLock()
		defer s.mu.RUnlock()

		sessions, err := s.store.QuerySessions(c.Request.Context(), q)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"sessions": sessions})
	}
}

// handleCreateSession godoc
//
//	@Description	Creates a new session
func (s *SessionService) handleCreateSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createSessionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		name := req.Name
		if name == "" {
			name = infratemplate.GenerateUniqueName()
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
			Plugins:     PluginConfigsFromNames(req.Plugins),
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := s.store.SaveSession(c.Request.Context(), meta); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// HEAD starts as a symbolic ref pointing at "main". Dispatches advance
		// "main"; checkout rewrites the symref to point at another branch.
		if err := s.store.refs.WriteSymRef(c.Request.Context(), meta.ID, dag.HEAD, dag.MAIN); err != nil {
			s.logger.Warn("create session: HEAD symref init failed", "session", meta.ID, "error", err)
		}

		// Store the system message as the first DAG entry so the pipeline always
		// finds it at history[0]. If no template was supplied the pipeline will
		// write the agent-level default on the first commit instead.
		if req.Template != "" {
			sysMsg := &Message{
				Role:      "system",
				Content:   req.Template,
				CreatedAt: now,
			}

			if _, err := s.store.SaveMessage(c.Request.Context(), meta.ID, dag.HEAD, sysMsg); err != nil {
				s.logger.Warn("create session: failed to store system message", "session", meta.ID, "error", err)
			}
		}

		SessionsTotal.WithLabelValues("create").Inc()
		c.JSON(http.StatusCreated, meta)
	}
}

// handleGetSession godoc
//
//	@Description	Returns metadata for a single session by ID or name
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

type updateSessionRequest struct {
	Name        *string `json:"name"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Model       *string `json:"model"`
}

// handleUpdateSession godoc
//
//	@Description	Patches mutable metadata on a session. Only provided fields are updated.
func (s *SessionService) handleUpdateSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req updateSessionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		meta, err := s.resolveSessionLocked(c.Request.Context(), c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if meta.ArchivedAt != nil {
			c.JSON(http.StatusConflict, gin.H{"error": ErrSessionArchived.Error()})
			return
		}

		if req.Name != nil && *req.Name != meta.Name {
			if existing, err := s.store.FindSessionByName(c.Request.Context(), *req.Name); err == nil && existing != nil {
				c.JSON(http.StatusConflict, gin.H{"error": "session name already exists: " + *req.Name})
				return
			}
			meta.Name = *req.Name
		}
		if req.Title != nil {
			meta.Title = *req.Title
		}
		if req.Description != nil {
			meta.Description = *req.Description
		}
		if req.Model != nil {
			meta.Model = *req.Model
		}
		meta.UpdatedAt = time.Now()
		if err := s.store.SaveSession(c.Request.Context(), meta); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, meta)
	}
}

// handleListMessages godoc
//
//	@Description	Returns messages for a session in chronological order
func (s *SessionService) handleListMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("session_id")
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		branch := c.Query("branch")

		s.mu.RLock()
		defer s.mu.RUnlock()

		meta, err := s.resolveSessionLocked(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		var messages []*Message
		if branch != "" && branch != "HEAD" {
			messages, err = s.store.ListMessagesFromRef(c.Request.Context(), meta.ID, branch, offset, limit)
		} else {
			messages, err = s.store.ListMessages(c.Request.Context(), meta.ID, offset, limit)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"messages": messages})
	}
}

// handleGetMessage godoc
//
//	@Description	Returns a single message by ID
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
//	@Description	Removes intermediate tool call and tool result messages to reduce context size
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

// handleDeleteSession godoc
//
//	@Description	Permanently deletes a session and all its data. Only archived sessions may be deleted; archive the session first to preserve its history as a resource.
func (s *SessionService) handleDeleteSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.mu.Lock()
		defer s.mu.Unlock()

		meta, err := s.resolveSessionLocked(c.Request.Context(), c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if meta.ArchivedAt == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session must be archived before it can be deleted"})
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

type updatePluginRequest struct {
	Enabled  *bool `json:"enabled"`
	Verbose  *bool `json:"verbose"`
	Disabled *bool `json:"disabled"`
}

// handleUpdatePlugin godoc
//
//	@Description	Patches a single plugin's activation state (enabled/verbose/disabled) within a session.
func (s *SessionService) handleUpdatePlugin() gin.HandlerFunc {
	return func(c *gin.Context) {
		pluginName := strings.ToLower(strings.TrimSpace(c.Param("name")))
		if pluginName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "plugin name is required"})
			return
		}

		var req updatePluginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		meta, err := s.resolveSessionLocked(c.Request.Context(), c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		if meta.ArchivedAt != nil {
			c.JSON(http.StatusConflict, gin.H{"error": ErrSessionArchived.Error()})
			return
		}

		found := false
		for i, p := range meta.Plugins {
			if strings.ToLower(p.Name) == pluginName {
				if req.Enabled != nil {
					meta.Plugins[i].Enabled = *req.Enabled
					if *req.Enabled {
						meta.Plugins[i].Disabled = false
					}
				}

				if req.Disabled != nil {
					meta.Plugins[i].Disabled = *req.Disabled
					if *req.Disabled {
						meta.Plugins[i].Enabled = false
					}
				}

				if req.Verbose != nil {
					meta.Plugins[i].Verbose = *req.Verbose
				}

				found = true
				break
			}
		}
		if !found {
			cfg := PluginConfig{Name: pluginName}
			if req.Enabled != nil {
				cfg.Enabled = *req.Enabled
			}

			if req.Disabled != nil {
				cfg.Disabled = *req.Disabled
				if cfg.Disabled {
					cfg.Enabled = false
				}
			}

			if req.Verbose != nil {
				cfg.Verbose = *req.Verbose
			}

			meta.Plugins = append(meta.Plugins, cfg)
		}

		meta.UpdatedAt = time.Now()
		if err := s.store.SaveSession(c.Request.Context(), meta); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		
		c.JSON(http.StatusOK, meta)
	}
}

// handleSummarizeMessages godoc
//
//	@Description	Summarizes the session history into a compressed context message (not yet implemented)
func (s *SessionService) handleSummarizeMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "summarize not yet implemented"})
	}
}

type archiveRequest struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

type cloneRequest struct {
	Name string `json:"name"`
}

// handleArchiveSession godoc
//
//	@Description	Walks the named ref (default HEAD), stores an envelope through the resource layer, and flips the session to immutable.
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

		res, err := s.ArchiveSession(c.Request.Context(), meta.ID, req.Ref, req.Name)
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
//	@Description	Replays an archive envelope into a fresh live session whose HEAD points at the archived tip.
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
