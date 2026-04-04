package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/registry"
)

type pluginResponse struct {
	Name         string                      `json:"name"`
	Capabilities *plugins.DriverCapabilities `json:"capabilities,omitempty"`
}

type listPluginsResponse struct {
	Plugins []pluginResponse `json:"plugins"`
}

func ListPlugins(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		drivers := reg.ListDrivers()
		resp := listPluginsResponse{
			Plugins: make([]pluginResponse, 0, len(drivers)),
		}
		for _, d := range drivers {
			resp.Plugins = append(resp.Plugins, pluginResponse{
				Name:         d.Info.Name,
				Capabilities: d.Capabilities,
			})
		}
		c.JSON(http.StatusOK, resp)
	}
}

func GetPlugin(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		d := reg.GetDriver(name)
		if d == nil {
			respondError(c, http.StatusNotFound, "not_found", "plugin not found")
			return
		}
		c.JSON(http.StatusOK, pluginResponse{
			Name:         d.Info.Name,
			Capabilities: d.Capabilities,
		})
	}
}
