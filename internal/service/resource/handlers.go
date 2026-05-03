package resource

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// handleStatus godoc
//
//	@Summary		Resource status
//	@Description	Returns the active mount table and the catch-all backend.
//	@Tags			resource
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Security		BearerAuth
//	@Router			/v1/resources [get]
func (s *ResourceService) handleStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.mu.RLock()
		mounts := make([]gin.H, len(s.mounts))
		for i, m := range s.mounts {
			mounts[i] = gin.H{"path": m.prefix, "plugin": m.plugin}
		}
		s.mu.RUnlock()
		c.JSON(http.StatusOK, gin.H{
			"mounts":  mounts,
			"default": "file",
		})
	}
}

type storeResourceRequest struct {
	Content  string         `json:"content" binding:"required"`
	Tags     []string       `json:"tags,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// handleStoreResource godoc
//
//	@Summary		Store a resource
//	@Description	Persists a resource at the path given in the URL.
//	@Tags			resource
//	@Accept			json
//	@Produce		json
//	@Param			path	path		string					true	"Resource path (e.g. /sessions/abc123)"
//	@Param			body	body		storeResourceRequest	true	"Resource body"
//	@Success		200		{object}	map[string]any
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/resources/{path} [put]
func (s *ResourceService) handleStoreResource() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req storeResourceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		path := normalizePath(c.Param("path"))
		res, err := s.Store(c.Request.Context(), path, req.Content, req.Tags, req.Metadata)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// handleListOrGet godoc
//
//	@Summary		List or get resources
//	@Description	Lists all resources at the given path. Pass ?id=<id> to fetch a single resource.
//	@Tags			resource
//	@Produce		json
//	@Param			path	path		string	true	"Resource path"
//	@Param			id		query		string	false	"Resource ID (fetch single)"
//	@Success		200		{object}	map[string]any
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/resources/{path} [get]
func (s *ResourceService) handleListOrGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := normalizePath(c.Param("path"))

		if id := c.Query("id"); id != "" {
			res, err := s.Get(c.Request.Context(), path, id)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, res)
			return
		}

		res, err := s.List(c.Request.Context(), path)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"resources": res})
	}
}

// handleRecallResources godoc
//
//	@Summary		Recall resources
//	@Description	Search resources by path glob, content query, tags, metadata predicates, and time range. Path from URL is the default; body fields override it.
//	@Tags			resource
//	@Accept			json
//	@Produce		json
//	@Param			path	path		string			true	"Default path or glob"
//	@Param			body	body		map[string]any	false	"RecallQuery overrides (path, query, tags, filter, created_after, created_before, limit)"
//	@Success		200		{object}	map[string]any
//	@Failure		400		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/resources/{path} [post]
func (s *ResourceService) handleRecallResources() gin.HandlerFunc {
	return func(c *gin.Context) {
		defaultPath := normalizePath(c.Param("path"))

		var args map[string]any
		if err := c.ShouldBindJSON(&args); err != nil {
			// Empty body is fine — use URL path with no filters
			args = map[string]any{}
		}

		q, err := recallQueryFromArgs(args, defaultPath)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		res, err := s.Recall(c.Request.Context(), q)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"resources": res})
	}
}

// handleForgetResource godoc
//
//	@Summary		Forget a resource
//	@Description	Removes the resource at the given path. Requires ?id=<id>.
//	@Tags			resource
//	@Param			path	path	string	true	"Resource path"
//	@Param			id		query	string	true	"Resource ID"
//	@Success		204
//	@Failure		400	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/resources/{path} [delete]
func (s *ResourceService) handleForgetResource() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing required query parameter: id"})
			return
		}
		path := normalizePath(c.Param("path"))
		if err := s.Forget(c.Request.Context(), path, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// normalizePath ensures the path has a leading slash and no trailing slash.
func normalizePath(raw string) string {
	p := "/" + strings.Trim(raw, "/")
	if p == "/" {
		return "/"
	}
	return strings.TrimRight(p, "/")
}
