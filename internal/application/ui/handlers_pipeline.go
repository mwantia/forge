package ui

import (
	"encoding/json"
	"html"
	"net/http"

	"github.com/gin-gonic/gin"
)

type pipelineHandlers struct {
	pipeline pipelineCommitter
}

func (h *pipelineHandlers) handleCommit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		ref := c.PostForm("ref")
		content := c.PostForm("content")

		if content == "" {
			c.String(http.StatusBadRequest, "content is required")
			return
		}

		events, err := h.pipeline.CommitStream(ctx, id, ref, content)
		if err != nil {
			c.String(http.StatusInternalServerError, "pipeline error: %v", err)
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)

		flusher, _ := c.Writer.(http.Flusher)

		for ev := range events {
			switch ev.Type {
			case "token":
				var chunk struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(ev.Data, &chunk); err == nil {
					c.SSEvent("chunk", html.EscapeString(chunk.Text))
				}
			case "tool_call":
				var tc struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal(ev.Data, &tc); err == nil {
					c.SSEvent("tool", html.EscapeString(tc.Name))
				}
			case "done":
				c.SSEvent("done", "")
				if flusher != nil {
					flusher.Flush()
				}
				return
			case "error":
				var e struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(ev.Data, &e); err == nil {
					c.SSEvent("error", html.EscapeString(e.Message))
				}
				if flusher != nil {
					flusher.Flush()
				}
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}
