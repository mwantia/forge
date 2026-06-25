package system

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge-sdk/pkg/plugin/base"
	domplugin "github.com/mwantia/forge/internal/domain/plugin"
)

type pluginHealthResponse struct {
	Name    string            `json:"name"`
	Types   []string          `json:"types,omitempty"`
	Status  base.PluginStatus `json:"status"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Action  string            `json:"action,omitempty"`
	Latency int64             `json:"latency"`
}

type systemHealthResponse struct {
	Status  base.PluginStatus      `json:"status"`
	Plugins []pluginHealthResponse `json:"plugins"`
}

func toHealthEntry(name string, caps *base.DriverCapabilities, h *base.PluginHealth) pluginHealthResponse {
	var types []string
	if caps != nil {
		types = caps.Types
	}
	if h == nil {
		return pluginHealthResponse{
			Name:    name,
			Types:   types,
			Status:  base.StatusUnhealthy,
			Code:    base.HealthCodeConfigInvalid,
			Message: "no health response",
		}
	}
	return pluginHealthResponse{
		Name:    name,
		Types:   types,
		Status:  h.Status,
		Code:    h.Code,
		Message: h.Message,
		Action:  h.Action,
		Latency: h.Latency.Nanoseconds(),
	}
}

func worstOf(a, b base.PluginStatus) base.PluginStatus {
	rank := map[base.PluginStatus]int{
		base.StatusHealthy:   0,
		base.StatusDegraded:  1,
		base.StatusUnhealthy: 2,
	}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func fanOutHealth(ctx context.Context, drivers []*domplugin.PluginDriver) ([]pluginHealthResponse, base.PluginStatus) {
	type result struct {
		name   string
		caps   *base.DriverCapabilities
		health *base.PluginHealth
	}

	ch := make(chan result, len(drivers))
	for _, d := range drivers {
		go func(drv *domplugin.PluginDriver) {
			h, _ := drv.Driver.GetPluginHealth(ctx)
			ch <- result{drv.Info.Name, drv.Capabilities, h}
		}(d)
	}

	entries := make([]pluginHealthResponse, 0, len(drivers))
	worst := base.StatusHealthy
	for range drivers {
		r := <-ch
		e := toHealthEntry(r.name, r.caps, r.health)
		entries = append(entries, e)
		worst = worstOf(worst, e.Status)
	}
	return entries, worst
}

// handleSystemHealth godoc
//
//	@Description	Fan-out GetPluginHealth across all drivers and return aggregate status
func (s *SystemService) handleSystemHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		entries, worst := fanOutHealth(ctx, s.plugins.ListDrivers())
		c.JSON(http.StatusOK, systemHealthResponse{
			Status:  worst,
			Plugins: entries,
		})
	}
}
