package resource

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// handleStatus godoc
//
//	@Summary		Resource status
//	@Description	Returns the active storage backend for resources.
//	@Tags			resource
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Security		BearerAuth
//	@Router			/v1/resources [get]
func (s *ResourceService) handleStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"backend": "dag"})
	}
}

type storeResourceRequest struct {
	Name     string         `json:"name,omitempty"`
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
		res, err := s.Store(c.Request.Context(), path, req.Name, req.Content, req.Tags, req.Metadata)
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
		raw := normalizePath(c.Param("path"))
		ctx := c.Request.Context()

		// Dispatch on path suffix — Gin catch-all captures /history, /diff, /versions/<hash>.
		if base, ok := strings.CutSuffix(raw, "/history"); ok {
			name := c.Query("name")
			if name == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing required query parameter: name"})
				return
			}
			revs, err := s.History(ctx, base, name)
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"history": revs})
			return
		}

		if base, ok := strings.CutSuffix(raw, "/diff"); ok {
			name := c.Query("name")
			if name == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing required query parameter: name"})
				return
			}
			toHash := c.Query("to")
			fromHash := c.Query("from")

			if toHash == "" {
				revs, err := s.History(ctx, base, name)
				if err != nil || len(revs) == 0 {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "could not resolve tip hash"})
					return
				}
				toHash = revs[0].Hash
			}

			toObj, err := s.GetAt(ctx, toHash)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "load 'to' version: " + err.Error()})
				return
			}

			if fromHash == "" {
				fromHash = toObj.ParentHash
			}

			fromContent := ""
			if fromHash != "" {
				fromObj, err := s.GetAt(ctx, fromHash)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "load 'from' version: " + err.Error()})
					return
				}
				fromContent = fromObj.Content
			}

			patch, text := computeDiff(fromContent, toObj.Content)
			c.JSON(http.StatusOK, gin.H{"from": fromHash, "to": toHash, "patch": patch, "text": text})
			return
		}

		// /versions/<hash> suffix: path is /<ns-segments>/versions/<64hex>
		if i := strings.Index(raw, "/versions/"); i >= 0 {
			hash := raw[i+len("/versions/"):]
			obj, err := s.GetAt(ctx, hash)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"hash":         hash,
				"content":      obj.Content,
				"content_type": obj.ContentType,
				"parent_hash":  obj.ParentHash,
			})
			return
		}

		path := raw
		if id := c.Query("id"); id != "" {
			res, err := s.Get(ctx, path, id)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, res)
			return
		}

		res, err := s.List(ctx, path)
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
		raw := normalizePath(c.Param("path"))
		ctx := c.Request.Context()

		// Revert is a POST with /revert suffix.
		if base, ok := strings.CutSuffix(raw, "/revert"); ok {
			name := c.Query("name")
			if name == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing required query parameter: name"})
				return
			}
			var req struct {
				To string `json:"to" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if err := s.Revert(ctx, base, name, req.To); err != nil {
				if strings.Contains(err.Error(), "not found") {
					c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.Status(http.StatusNoContent)
			return
		}

		var args map[string]any
		if err := c.ShouldBindJSON(&args); err != nil {
			// Empty body is fine — use URL path with no filters
			args = map[string]any{}
		}

		q, err := recallQueryFromArgs(args, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		res, err := s.Recall(ctx, q)
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

type patchResourceRequest struct {
	Tags     []string       `json:"tags,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// handlePatchResource godoc
//
//	@Summary		Patch resource metadata
//	@Description	Updates tags and metadata for an existing resource without changing its content or advancing the content ref. Requires ?name=<name>.
//	@Tags			resource
//	@Accept			json
//	@Produce		json
//	@Param			path	path		string				true	"Resource namespace path"
//	@Param			name	query		string				true	"Resource name (ref key)"
//	@Param			body	body		patchResourceRequest	true	"Fields to update"
//	@Success		204
//	@Failure		400	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/resources/{path} [patch]
func (s *ResourceService) handlePatchResource() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Query("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing required query parameter: name"})
			return
		}

		var req patchResourceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		path := normalizePath(c.Param("path"))
		if err := s.UpdateMeta(c.Request.Context(), path, name, req.Tags, req.Metadata); err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
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
