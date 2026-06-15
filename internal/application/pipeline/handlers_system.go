package pipeline

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appsession "github.com/mwantia/forge/internal/application/session"
)

type systemResetRequest struct {
	Plugins []string `json:"plugins"`
}

// handleResetSystemSnapshot godoc
//
//	@Description	Updates the session plugin filter. Replaces the active plugin list; pass an empty array to switch to all-plugins mode. Returns the updated session metadata.
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

		if len(req.Plugins) > 0 {
			meta.Plugins = appsession.PluginConfigsFromNames(req.Plugins)
		}
		meta.UpdatedAt = time.Now()

		if err := s.sessions.SaveSession(ctx, meta); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, meta)
	}
}
