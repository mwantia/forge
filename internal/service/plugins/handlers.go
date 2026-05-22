package plugins

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge-sdk/pkg/plugins"
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
			c.JSON(http.StatusOK, &plugins.DriverCapabilities{})
			return
		}
		c.JSON(http.StatusOK, driver.Capabilities)
	}
}
