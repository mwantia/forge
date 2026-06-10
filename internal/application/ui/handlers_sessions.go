package ui

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	tmplsessions "github.com/mwantia/forge/internal/application/ui/templates/sessions"
)

type sessionHandlers struct {
	sessions sessionReader
	renderer pipelineRenderer
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
		raw, _ := h.sessions.ListMessagesFromRef(ctx, meta.ID, ref, 0, 0)
		refs, _ := h.sessions.ListRefs(ctx, meta.ID)
		activeRef := resolveActiveRef(refs, ref)

		messages := make([]*tmplsessions.RenderedMessage, len(raw))
		for i, msg := range raw {
			rm := &tmplsessions.RenderedMessage{Message: msg, Rendered: msg.Content}
			if h.renderer != nil {
				if r, err := h.renderer.RenderContent(ctx, meta.ID, msg.Content); err == nil {
					rm.Rendered = r
				}
			}
			messages[i] = rm
		}

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

func (h *sessionHandlers) handleThread() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found: %v", err)
			return
		}

		refs, _ := h.sessions.ListRefs(ctx, meta.ID)
		activeRef := resolveActiveRef(refs, c.DefaultQuery("ref", ""))
		raw, _ := h.sessions.ListMessagesFromRef(ctx, meta.ID, activeRef, 0, 0)

		messages := make([]*tmplsessions.RenderedMessage, len(raw))
		for i, msg := range raw {
			rm := &tmplsessions.RenderedMessage{Message: msg, Rendered: msg.Content}
			if h.renderer != nil {
				if r, err := h.renderer.RenderContent(ctx, meta.ID, msg.Content); err == nil {
					rm.Rendered = r
				}
			}
			messages[i] = rm
		}

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.Thread(messages, meta.ArchivedAt != nil).Render(ctx, c.Writer)
	}
}

func (h *sessionHandlers) handleNodePanel() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found: %v", err)
			return
		}

		refs, _ := h.sessions.ListRefs(ctx, id)
		ref := c.DefaultQuery("ref", "HEAD")
		activeRef := resolveActiveRef(refs, ref)

		messages, _ := h.sessions.ListMessagesFromRef(ctx, id, activeRef, 0, 0)

		// Count sibling nodes: other ref tips that share the same parent as HEAD tip.
		siblingCount := 0
		if len(messages) > 0 {
			headTip := messages[len(messages)-1]
			for name, refHash := range refs {
				if name == "HEAD" || name == activeRef || refHash == headTip.Hash {
					continue
				}
				obj, err := h.sessions.GetMessageObj(ctx, refHash)
				if err != nil {
					continue
				}
				if obj.ParentHash == headTip.ParentHash {
					siblingCount++
				}
			}
		}

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.NodePanel(id, meta, messages, activeRef, siblingCount).Render(ctx, c.Writer)
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
