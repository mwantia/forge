package ui

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	tmplsessions "github.com/mwantia/forge/internal/application/ui/templates/sessions"
)

type sessionHandlers struct {
	sessions sessionReader
}

func (h *sessionHandlers) handleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		archived := c.Query("archived") == "true"

		sessions, err := h.sessions.ListParentSessions(c.Request.Context(), "", archived, offset, limit)
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to list sessions: %v", err)
			return
		}

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.List(sessions, archived).Render(c.Request.Context(), c.Writer)
	}
}

func (h *sessionHandlers) handleCreate() gin.HandlerFunc {
	return func(c *gin.Context) {
		model := c.PostForm("model")
		name := c.PostForm("name")
		title := c.PostForm("title")

		if model == "" {
			c.String(http.StatusBadRequest, "model is required")
			return
		}

		meta, err := h.sessions.CreateSession(c.Request.Context(), model, name, title, "", "", "", nil)
		if err != nil {
			c.String(http.StatusConflict, "create failed: %v", err)
			return
		}

		c.Header("HX-Redirect", "/ui/sessions/"+meta.ID)
		c.Status(http.StatusSeeOther)
	}
}

func (h *sessionHandlers) handleDetail() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found: %v", err)
			return
		}

		ref := c.DefaultQuery("ref", "HEAD")
		messages, _ := h.sessions.ListMessagesFromRef(ctx, meta.ID, ref, 0, 0)
		refs, _ := h.sessions.ListRefs(ctx, meta.ID)
		activeRef := resolveActiveRef(refs, ref)

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.Detail(meta, messages, refs, activeRef).Render(ctx, c.Writer)
	}
}

func (h *sessionHandlers) handleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.sessions.DeleteSession(c.Request.Context(), c.Param("id")); err != nil {
			c.String(http.StatusInternalServerError, "delete failed: %v", err)
			return
		}
		c.Header("HX-Redirect", "/ui/sessions")
		c.Status(http.StatusOK)
	}
}

// resolveActiveRef returns the best display branch name from the refs map.
func resolveActiveRef(refs map[string]string, requested string) string {
	if requested != "" && requested != "HEAD" {
		if _, ok := refs[requested]; ok {
			return requested
		}
	}
	if _, ok := refs["main"]; ok {
		return "main"
	}
	for name := range refs {
		if name != "HEAD" {
			return name
		}
	}
	return "HEAD"
}
