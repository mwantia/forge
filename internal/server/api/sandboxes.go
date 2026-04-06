package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/sandbox"
)

func ListSandboxes(mgr *sandbox.SandboxManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		opts := sandbox.ListOptions{
			Limit:     limit,
			Offset:    offset,
			SessionID: c.Query("session_id"),
		}

		sbs, err := mgr.List(opts)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"sandboxes": sbs})
	}
}

func ListSessionSandboxes(mgr *sandbox.SandboxManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		sbs, err := mgr.List(sandbox.ListOptions{SessionID: c.Param("id")})
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"sandboxes": sbs})
	}
}

func CreateSandbox(mgr *sandbox.SandboxManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var opts sandbox.CreateOptions
		if err := c.ShouldBindJSON(&opts); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if opts.SessionID == "" {
			respondError(c, http.StatusBadRequest, "bad_request", "session_id is required")
			return
		}
		sb, err := mgr.Create(c.Request.Context(), opts)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.JSON(http.StatusCreated, sb)
	}
}

func GetSandbox(mgr *sandbox.SandboxManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		sb, err := mgr.Get(c.Param("id"))
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", err.Error())
			return
		}
		c.JSON(http.StatusOK, sb)
	}
}

func DeleteSandbox(mgr *sandbox.SandboxManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := mgr.Delete(c.Request.Context(), c.Param("id")); err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func ExecSandbox(mgr *sandbox.SandboxManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		stream := c.Query("stream") == "true"

		var req plugins.SandboxExecRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		ch, err := mgr.Execute(c.Request.Context(), id, req)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		if stream {
			streamExecChunks(c, ch)
			return
		}

		// Non-streaming: collect and return.
		var result struct {
			Stdout   string `json:"stdout"`
			Stderr   string `json:"stderr"`
			ExitCode int    `json:"exit_code"`
		}
		for chunk := range ch {
			if chunk.IsError {
				respondError(c, http.StatusInternalServerError, "exec_error", chunk.Data)
				return
			}
			switch chunk.Stream {
			case "stdout":
				result.Stdout += chunk.Data
			case "stderr":
				result.Stderr += chunk.Data
			}
			if chunk.Done {
				result.ExitCode = chunk.ExitCode
			}
		}
		c.JSON(http.StatusOK, result)
	}
}

func CopyInSandbox(mgr *sandbox.SandboxManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var body struct {
			HostSrc    string `json:"host_src"    binding:"required"`
			SandboxDst string `json:"sandbox_dst" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := mgr.CopyIn(c.Request.Context(), id, body.HostSrc, body.SandboxDst); err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"copied": true})
	}
}

func CopyOutSandbox(mgr *sandbox.SandboxManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var body struct {
			SandboxSrc string `json:"sandbox_src" binding:"required"`
			HostDst    string `json:"host_dst"    binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := mgr.CopyOut(c.Request.Context(), id, body.SandboxSrc, body.HostDst); err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"copied": true})
	}
}

func StatSandbox(mgr *sandbox.SandboxManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Query("path")
		if path == "" {
			respondError(c, http.StatusBadRequest, "bad_request", "path query parameter is required")
			return
		}
		result, err := mgr.Stat(c.Request.Context(), c.Param("id"), path)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func ReadFileSandbox(mgr *sandbox.SandboxManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Query("path")
		if path == "" {
			respondError(c, http.StatusBadRequest, "bad_request", "path query parameter is required")
			return
		}
		data, err := mgr.ReadFile(c.Request.Context(), c.Param("id"), path)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"path": path, "content": string(data)})
	}
}

// streamExecChunks writes sandbox execution output as SSE events.
func streamExecChunks(c *gin.Context, ch <-chan plugins.SandboxExecChunk) {
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
		b, _ := json.Marshal(chunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", b)
		flusher.Flush()
		if chunk.Done || chunk.IsError {
			fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
	}
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}
