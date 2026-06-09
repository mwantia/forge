package pipeline

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type systemResetRequest struct {
	ToolsVerbosity string   `json:"tools_verbosity"`
	Plugins        []string `json:"plugins"`
}

// handleResetSystemSnapshot godoc
//
//	@Description	Updates session-level pipeline settings (tools_verbosity, plugins filter). Returns the currently active session metadata.
func (s *PipelineService) handleResetSystemSnapshot() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var req systemResetRequest
		_ = c.ShouldBindJSON(&req)

		meta, err := s.sessions.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		if req.ToolsVerbosity != "" {
			meta.ToolsVerbosity = req.ToolsVerbosity
		}
		if len(req.Plugins) > 0 {
			meta.Plugins = req.Plugins
		}
		meta.UpdatedAt = time.Now()

		if err := s.sessions.SaveSession(ctx, meta); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, meta)
	}
}
