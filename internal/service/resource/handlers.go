package resource

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// handleStatus godoc
//
//	@Description	Returns the active storage backend for resources.
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
//	@Description	Persists a resource at the path given in the URL.
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
//	@Description	Lists all resources at the given path. Pass ?id=<id> to fetch a single resource.
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
//	@Description	Search resources by path glob, content query, tags, metadata predicates, and time range. Path from URL is the default; body fields override it.
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
//	@Description	Removes the resource at the given path. Requires ?id=<id>.
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
//	@Description	Updates tags and metadata for an existing resource without changing its content or advancing the content ref. Requires ?name=<name>.
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
