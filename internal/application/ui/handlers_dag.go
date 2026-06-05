package ui

import (
	"net/http"

	"github.com/gin-gonic/gin"
	uidag "github.com/mwantia/forge/internal/application/ui/templates/dag"
)

type dagHandlers struct {
	sessions sessionReader
}

func (h *dagHandlers) handleFull() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found: %v", err)
			return
		}

		refs, _ := h.sessions.ListRefs(ctx, id)
		messages, _ := h.sessions.ListMessages(ctx, id, 0, 500)
		nodes, edges := uidag.BuildLayout(messages, refs)

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = uidag.Full(meta, nodes, edges).Render(ctx, c.Writer)
	}
}

func (h *dagHandlers) handleMini() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		refs, _ := h.sessions.ListRefs(ctx, id)
		messages, _ := h.sessions.ListMessages(ctx, id, 0, 200)
		nodes, edges := uidag.BuildMiniLayout(messages, refs)
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = uidag.Mini(nodes, edges, id).Render(ctx, c.Writer)
	}
}
