package plugins

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge-sdk/pkg/plugin/base"
)

// handleListPlugins godoc
//
//	@Description	Returns info for all loaded plugin drivers
func (s *PluginsService) handleListPlugins() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		infos := make([]PluginDriverInfo, 0, len(s.drivers))
		for _, d := range s.drivers {
			infos = append(infos, d.Info)
		}
		c.JSON(http.StatusOK, gin.H{"plugins": infos})
	}
}

// handleGetPlugin godoc
//
//	@Description	Returns info for a single plugin driver by name
func (s *PluginsService) handleGetPlugin() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		s.mu.RLock()
		driver, ok := s.drivers[name]
		s.mu.RUnlock()

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found: " + name})
			return
		}
		c.JSON(http.StatusOK, driver.Info)
	}
}

// handleGetPluginCapabilities godoc
//
//	@Description	Returns the capability set advertised by a plugin driver
func (s *PluginsService) handleGetPluginCapabilities() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		s.mu.RLock()
		driver, ok := s.drivers[name]
		s.mu.RUnlock()

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found: " + name})
			return
		}
		if driver.Capabilities == nil {
			c.JSON(http.StatusOK, &base.DriverCapabilities{})
			return
		}
		c.JSON(http.StatusOK, driver.Capabilities)
	}
}

// handleGetPluginHealth godoc
//
//	@Description	Returns live health status for a single plugin driver
func (s *PluginsService) handleGetPluginHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		s.mu.RLock()
		driver, ok := s.drivers[name]
		s.mu.RUnlock()

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found: " + name})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		h, _ := driver.Driver.GetPluginHealth(ctx)

		var types []string
		if driver.Capabilities != nil {
			types = driver.Capabilities.Types
		}
		resp := gin.H{
			"name":  name,
			"types": types,
		}
		if h != nil {
			resp["status"] = h.Status
			resp["code"] = h.Code
			resp["message"] = h.Message
			resp["action"] = h.Action
			resp["latency"] = h.Latency.Nanoseconds()
		} else {
			resp["status"] = base.StatusUnhealthy
			resp["code"] = base.HealthCodeConfigInvalid
			resp["message"] = "no health response"
		}
		c.JSON(http.StatusOK, resp)
	}
}
