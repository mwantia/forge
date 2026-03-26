package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/pkg/plugins"
)

type toolEntry struct {
	Driver string `json:"driver"`
	plugins.ToolDefinition
}

type listToolsResponse struct {
	Tools []toolEntry `json:"tools"`
}

func ListTools(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		all := reg.GetAllToolsPlugins(ctx)
		resp := listToolsResponse{Tools: make([]toolEntry, 0)}
		for name, tp := range all {
			result, err := tp.ListTools(ctx, plugins.ListToolsFilter{})
			if err != nil {
				continue
			}
			for _, t := range result.Tools {
				resp.Tools = append(resp.Tools, toolEntry{Driver: name, ToolDefinition: t})
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}

func ListDriverTools(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		driverName := c.Param("driver")
		tp, err := reg.GetToolsPlugin(ctx, driverName)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", "tools plugin not found")
			return
		}
		result, err := tp.ListTools(ctx, plugins.ListToolsFilter{})
		if err != nil {
			respondError(c, http.StatusBadGateway, "plugin_error", err.Error())
			return
		}
		resp := listToolsResponse{Tools: make([]toolEntry, 0, len(result.Tools))}
		for _, t := range result.Tools {
			resp.Tools = append(resp.Tools, toolEntry{Driver: driverName, ToolDefinition: t})
		}
		c.JSON(http.StatusOK, resp)
	}
}

func GetDriverTool(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		driverName := c.Param("driver")
		toolName := c.Param("tool")
		tp, err := reg.GetToolsPlugin(ctx, driverName)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", "tools plugin not found")
			return
		}
		def, err := tp.GetTool(ctx, toolName)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", "tool not found")
			return
		}
		c.JSON(http.StatusOK, toolEntry{Driver: driverName, ToolDefinition: *def})
	}
}

type executeToolRequest struct {
	CallID    string         `json:"call_id"`
	Arguments map[string]any `json:"arguments"`
}

func ValidateTool(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		driverName := c.Param("driver")
		toolName := c.Param("tool")
		var req executeToolRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		tp, err := reg.GetToolsPlugin(ctx, driverName)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", "tools plugin not found")
			return
		}
		resp, err := tp.Validate(ctx, plugins.ExecuteRequest{
			Tool:      toolName,
			Arguments: req.Arguments,
			CallID:    req.CallID,
		})
		if err != nil {
			respondError(c, http.StatusBadGateway, "plugin_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func ExecuteTool(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		driverName := c.Param("driver")
		toolName := c.Param("tool")
		var req executeToolRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		tp, err := reg.GetToolsPlugin(ctx, driverName)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", "tools plugin not found")
			return
		}

		execReq := plugins.ExecuteRequest{
			Tool:      toolName,
			Arguments: req.Arguments,
			CallID:    req.CallID,
		}

		// If client wants SSE, drain ExecuteStream.
		if c.GetHeader("Accept") == "text/event-stream" {
			ch, err := tp.ExecuteStream(ctx, execReq)
			if err != nil {
				respondError(c, http.StatusBadGateway, "plugin_error", err.Error())
				return
			}
			streamExecute(c, ch)
			return
		}

		resp, err := tp.Execute(ctx, execReq)
		if err != nil {
			respondError(c, http.StatusBadGateway, "plugin_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func CancelTool(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		driverName := c.Param("driver")
		callID := c.Param("call_id")
		tp, err := reg.GetToolsPlugin(ctx, driverName)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", "tools plugin not found")
			return
		}
		if err := tp.Cancel(ctx, callID); err != nil {
			respondError(c, http.StatusBadGateway, "plugin_error", err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func streamExecute(c *gin.Context, ch <-chan plugins.ExecuteChunk) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}

	for chunk := range ch {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		if chunk.Done {
			break
		}
	}
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}
