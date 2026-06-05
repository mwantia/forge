package ui

import (
	"net/http"

	"github.com/gin-gonic/gin"
	tmplrefs "github.com/mwantia/forge/internal/application/ui/templates/refs"
)

type refHandlers struct {
	sessions sessionReader
}

func (h *refHandlers) handlePanel() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		refs, err := h.sessions.ListRefs(ctx, id)
		if err != nil {
			c.String(http.StatusInternalServerError, "list refs failed: %v", err)
			return
		}
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplrefs.Panel(id, refs).Render(ctx, c.Writer)
	}
}

func (h *refHandlers) handleCreate() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		name := c.PostForm("name")
		hash := c.PostForm("hash")
		if name == "" {
			c.String(http.StatusBadRequest, "name required")
			return
		}
		if err := h.sessions.WriteRef(ctx, id, name, hash); err != nil {
			c.String(http.StatusInternalServerError, "create ref failed: %v", err)
			return
		}
		refs, _ := h.sessions.ListRefs(ctx, id)
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplrefs.Panel(id, refs).Render(ctx, c.Writer)
	}
}

func (h *refHandlers) handleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		ref := c.Param("ref")
		if err := h.sessions.DeleteRef(ctx, id, ref); err != nil {
			c.String(http.StatusInternalServerError, "delete ref failed: %v", err)
			return
		}
		refs, _ := h.sessions.ListRefs(ctx, id)
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplrefs.Panel(id, refs).Render(ctx, c.Writer)
	}
}

func (h *refHandlers) handleCheckout() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		ref := c.Param("ref")
		if err := h.sessions.CheckoutRef(ctx, id, ref); err != nil {
			c.String(http.StatusInternalServerError, "checkout failed: %v", err)
			return
		}
		c.Header("HX-Redirect", "/ui/sessions/"+id+"?ref="+ref)
		c.Status(http.StatusOK)
	}
}
