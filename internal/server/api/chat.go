package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/pkg/plugins"
)

type chatRequest struct {
	Model    string               `json:"model"    binding:"required"`
	Messages []plugins.ChatMessage `json:"messages" binding:"required"`
	Tools    []string             `json:"tools"`
	Stream   bool                 `json:"stream"`
}

func Chat(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req chatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		ctx := c.Request.Context()

		// Resolve tool definitions from requested driver names.
		var toolCalls []plugins.ToolCall
		for _, driverName := range req.Tools {
			tp, err := reg.GetToolsPlugin(ctx, driverName)
			if err != nil {
				respondError(c, http.StatusBadRequest, "bad_request", fmt.Sprintf("tools plugin '%s' not found", driverName))
				return
			}
			resp, err := tp.ListTools(ctx, plugins.ListToolsFilter{})
			if err != nil {
				respondError(c, http.StatusBadGateway, "plugin_error", fmt.Sprintf("failed to list tools from '%s': %s", driverName, err))
				return
			}
			for _, t := range resp.Tools {
				toolCalls = append(toolCalls, plugins.ToolCall{
					Name:        fmt.Sprintf("%s/%s", driverName, t.Name),
					Description: t.Description,
					Parameters:  t.Parameters,
				})
			}
		}

		stream, err := reg.Provider().Chat(ctx, req.Model, req.Messages, toolCalls)
		if err != nil {
			respondError(c, http.StatusBadGateway, "provider_error", err.Error())
			return
		}

		if req.Stream {
			streamChat(c, stream)
			return
		}

		result, err := plugins.CollectStream(stream)
		if err != nil {
			respondError(c, http.StatusBadGateway, "provider_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func streamChat(c *gin.Context, stream plugins.ChatStream) {
	defer stream.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		if err != nil {
			data, _ := json.Marshal(errorResponse{Error: errorDetail{Code: "stream_error", Message: err.Error()}})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			flusher.Flush()
			return
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		if chunk.Done {
			fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
	}
}
