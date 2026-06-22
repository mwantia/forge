package ui

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	tmplsessions "github.com/mwantia/forge/internal/application/ui/templates/sessions"
)

type pipelineHandlers struct {
	pipeline pipelineCommitter
}

func (h *pipelineHandlers) handleCommit() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ref := c.PostForm("ref")
		content := c.PostForm("content")
		mode := c.PostForm("mode")

		if content == "" {
			c.String(http.StatusBadRequest, "content is required")
			return
		}

		token := newStreamToken(id, ref, content, mode)
		streamURL := fmt.Sprintf("/ui/sessions/%s/stream?token=%s&ref=%s", id, token, ref)

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusOK)
		_ = tmplsessions.StreamBubble(streamURL).Render(c.Request.Context(), c.Writer)
		// OOB: immediately show the user's message in #thread before streaming begins.
		_ = tmplsessions.PendingUserBubble(content).Render(c.Request.Context(), c.Writer)
	}
}
