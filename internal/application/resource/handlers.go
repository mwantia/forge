package resource

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	domresource "github.com/mwantia/forge/internal/domain/resource"
)

type storeResourceRequest struct {
	Content       string                   `json:"content" binding:"required"`
	CommitMessage string                   `json:"commit_message,omitempty"`
	Meta          domresource.ResourceMeta `json:"meta,omitempty"`
}

type commitResourceRequest struct {
	Content       string `json:"content" binding:"required"`
	CommitMessage string `json:"commit_message,omitempty"`
}

// handleStoreResource godoc
//
//	@Description	Creates a new resource. Returns the created resource including its generated ID.
func (s *ResourceService) handleStoreResource() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req storeResourceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if s.uploadMaxBytes > 0 && uint64(len(req.Content)) > s.uploadMaxBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "content exceeds upload.filesize limit"})
			return
		}
		res, err := s.Store(c.Request.Context(), req.Content, req.CommitMessage, req.Meta)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// handleCommitResource godoc
//
//	@Description	Commits a new content revision for an existing resource, advancing HEAD. Returns the updated resource.
func (s *ResourceService) handleCommitResource() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req commitResourceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if s.uploadMaxBytes > 0 && uint64(len(req.Content)) > s.uploadMaxBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "content exceeds upload.filesize limit"})
			return
		}
		res, err := s.Commit(c.Request.Context(), id, req.Content, req.CommitMessage)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// handleGetResource godoc
//
//	@Description	Returns a single resource by ID.
func (s *ResourceService) handleGetResource() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		res, err := s.Get(c.Request.Context(), id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// handleListResources godoc
//
//	@Description	Lists resources, optionally filtered by metadata predicates in the request body.
func (s *ResourceService) handleListResources() gin.HandlerFunc {
	return func(c *gin.Context) {
		var filter []domresource.FilterPredicate
		// Body is optional — empty body returns all resources.
		_ = c.ShouldBindJSON(&filter)

		res, err := s.List(c.Request.Context(), filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"resources": res})
	}
}

// handleRecallResources godoc
//
//	@Description	Searches resources by content query, tags, metadata predicates, and time range.
func (s *ResourceService) handleRecallResources() gin.HandlerFunc {
	return func(c *gin.Context) {
		var args map[string]any
		if err := c.ShouldBindJSON(&args); err != nil {
			args = map[string]any{}
		}

		q, err := recallQueryFromArgs(args)
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

// handlePatchResource godoc
//
//	@Description	Updates metadata for an existing resource without changing its content.
func (s *ResourceService) handlePatchResource() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var meta domresource.ResourceMeta
		if err := c.ShouldBindJSON(&meta); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := s.UpdateMeta(c.Request.Context(), id, meta); err != nil {
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

// handleForgetResource godoc
//
//	@Description	Removes a resource by ID.
func (s *ResourceService) handleForgetResource() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := s.Forget(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// handleHistory godoc
//
//	@Description	Returns the content revision chain for a resource.
func (s *ResourceService) handleHistory() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		revs, err := s.History(c.Request.Context(), id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"history": revs})
	}
}

// handleDiff godoc
//
//	@Description	Returns a Myers diff between two content revisions. ?from and ?to are content hashes; defaults to tip vs its parent.
func (s *ResourceService) handleDiff() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ctx := c.Request.Context()

		toHash := c.Query("to")
		fromHash := c.Query("from")

		if toHash == "" {
			revs, err := s.History(ctx, id)
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
	}
}

// handleGetVersion godoc
//
//	@Description	Returns a specific historical content object by its content hash.
func (s *ResourceService) handleGetVersion() gin.HandlerFunc {
	return func(c *gin.Context) {
		hash := c.Param("hash")
		obj, err := s.GetAt(c.Request.Context(), hash)
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
	}
}

// handleRevert godoc
//
//	@Description	Reverts a resource to a prior content hash. Body: {"to":"<hash>"}.
func (s *ResourceService) handleRevert() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			To string `json:"to" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := s.Revert(c.Request.Context(), id, req.To); err != nil {
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

// recallQueryFromArgs parses a flat map (from JSON body) into a RecallQuery.
func recallQueryFromArgs(args map[string]any) (domresource.RecallQuery, error) {
	q := domresource.RecallQuery{
		Query: argStringOptional(args, "query"),
		Tags:  argStringSlice(args, "tags"),
		Limit: argInt(args, "limit", 5),
	}

	if raw, ok := args["filter"].([]any); ok {
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key, _ := m["key"].(string)
			op, _ := m["op"].(string)
			val := m["value"]
			if key != "" && op != "" {
				q.Filter = append(q.Filter, domresource.FilterPredicate{
					Key: key, Op: domresource.FilterOp(op), Value: val,
				})
			}
		}
	}

	if v, ok := argString(args, "created_after"); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.CreatedAfter = t
		}
	}
	if v, ok := argString(args, "created_before"); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.CreatedBefore = t
		}
	}

	return q, nil
}
