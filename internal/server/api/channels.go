package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/internal/channel"
	"github.com/mwantia/forge/internal/session"
)

type ChannelDispatcherIface interface {
	GetStore(plugin string) (*channel.BindingStore, bool)
	ListPlugins() []string
}

type bindRequest struct {
	ChannelID   string `json:"channel_id"   binding:"required"`
	SessionName string `json:"session_name" binding:"required"`
}

// ListChannelBindings handles GET /v1/channels/:plugin/bindings
func ListChannelBindings(d ChannelDispatcherIface) gin.HandlerFunc {
	return func(c *gin.Context) {
		plugin := c.Param("plugin")
		store, ok := d.GetStore(plugin)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
				"code":    "plugin_not_found",
				"message": "no channel plugin named " + plugin,
			}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"bindings": store.All()})
	}
}

// BindChannel handles POST /v1/channels/:plugin/bind
func BindChannel(d ChannelDispatcherIface, mgr *session.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		plugin := c.Param("plugin")
		store, ok := d.GetStore(plugin)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
				"code":    "plugin_not_found",
				"message": "no channel plugin named " + plugin,
			}})
			return
		}

		var req bindRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
				"code":    "invalid_request",
				"message": err.Error(),
			}})
			return
		}

		sess, err := mgr.Get(req.SessionName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
				"code":    "session_not_found",
				"message": "session not found: " + req.SessionName,
			}})
			return
		}

		binding := &channel.ChannelBinding{
			SessionID:   sess.ID,
			SessionName: sess.Name,
			BoundAt:     time.Now(),
		}
		if err := store.Set(req.ChannelID, binding); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{
				"code":    "store_error",
				"message": err.Error(),
			}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"binding": binding})
	}
}

// UnbindChannel handles DELETE /v1/channels/:plugin/bind/:channel_id
func UnbindChannel(d ChannelDispatcherIface) gin.HandlerFunc {
	return func(c *gin.Context) {
		plugin := c.Param("plugin")
		channelID := c.Param("channel_id")

		store, ok := d.GetStore(plugin)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
				"code":    "plugin_not_found",
				"message": "no channel plugin named " + plugin,
			}})
			return
		}

		if _, exists := store.Get(channelID); !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
				"code":    "binding_not_found",
				"message": "no binding for channel " + channelID,
			}})
			return
		}

		if err := store.Delete(channelID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{
				"code":    "store_error",
				"message": err.Error(),
			}})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
