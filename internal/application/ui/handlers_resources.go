package ui

import (
	"net/http"

	"github.com/gin-gonic/gin"
	domresource "github.com/mwantia/forge/internal/domain/resource"
	tmplresources "github.com/mwantia/forge/internal/application/ui/templates/resources"
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

