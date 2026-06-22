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

