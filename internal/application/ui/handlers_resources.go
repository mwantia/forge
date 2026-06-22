package ui

import (
	"net/http"

	"github.com/gin-gonic/gin"
	tmplresources "github.com/mwantia/forge/internal/application/ui/templates/resources"
	domresource "github.com/mwantia/forge/internal/domain/resource"
)

type resourceHandlers struct {
	resources domresource.ResourceRegistar
}

func (h *resourceHandlers) handleUpload() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Content       string                   `json:"content" binding:"required"`
			CommitMessage string                   `json:"commit_message,omitempty"`
			Meta          domresource.ResourceMeta `json:"meta,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		res, err := h.resources.Store(c.Request.Context(), req.Content, req.CommitMessage, req.Meta)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

func (h *resourceHandlers) handleUICommit() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Content       string `json:"content" binding:"required"`
			CommitMessage string `json:"commit_message,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		res, err := h.resources.Commit(c.Request.Context(), id, req.Content, req.CommitMessage)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		history, _ := h.resources.History(c.Request.Context(), id)
		items := tmplresources.BuildResourceItems(
			[]*domresource.Resource{res},
			map[string][]*domresource.ResourceRevision{res.ID: history},
		)
		if len(items) == 0 {
			c.JSON(http.StatusOK, res)
			return
		}
		c.JSON(http.StatusOK, items[0])
	}
}

func (h *resourceHandlers) handleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		resources, err := h.resources.List(ctx, nil)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		histories := make(map[string][]*domresource.ResourceRevision, len(resources))
		for _, r := range resources {
			if revs, err := h.resources.History(ctx, r.ID); err == nil {
				histories[r.ID] = revs
			}
		}

		_ = tmplresources.List(resources, histories).Render(ctx, c.Writer)
	}
}

type recallRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func (h *resourceHandlers) handleRecall() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req recallRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Limit <= 0 {
			req.Limit = 10
		}

		ctx := c.Request.Context()
		resources, err := h.resources.Recall(ctx, domresource.RecallQuery{
			Query: req.Query,
			Limit: req.Limit,
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		histories := make(map[string][]*domresource.ResourceRevision, len(resources))
		for _, r := range resources {
			if revs, err := h.resources.History(ctx, r.ID); err == nil {
				histories[r.ID] = revs
			}
		}
		
		items := tmplresources.BuildResourceItems(resources, histories)
		c.JSON(http.StatusOK, gin.H{"resources": items})
	}
}

