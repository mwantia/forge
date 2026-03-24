package plugins

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/pkg/plugins"
)

type PluginRegistry struct {
	mutex sync.RWMutex

	logger  hclog.Logger
	drivers map[string]*PluginDriver
}

type PluginDriver struct {
	Name         string
	Capabilities *plugins.DriverCapabilities
	Driver       plugins.Driver
	Cleanup      PluginDriverCleanup
}

type PluginDriverCleanup func()

func NewRegistry(logger hclog.Logger) *PluginRegistry {
	return &PluginRegistry{
		drivers: make(map[string]*PluginDriver),
		logger:  logger.Named("registry"),
	}
}

func (r *PluginRegistry) CleanupDrivers() {
	for _, driver := range r.drivers {
		driver.Cleanup()
	}
}

func (r *PluginRegistry) GetProviderPlugin(ctx context.Context, name string) (plugins.ProviderPlugin, error) {
	driver, ok := r.drivers[strings.ToLower(name)]
	if ok {
		return driver.Driver.GetProviderPlugin(ctx)
	}

	return nil, fmt.Errorf("unknown provider name defined")
}

func (r *PluginRegistry) GetToolsPlugin(ctx context.Context, name string) (plugins.ToolsPlugin, error) {
	driver, ok := r.drivers[strings.ToLower(name)]
	if ok {
		return driver.Driver.GetToolsPlugin(ctx)
	}

	return nil, fmt.Errorf("unknown tools plugin name defined")
}

// GetAllToolsPlugins returns a map of driver name → ToolsPlugin for every loaded
// driver that advertises tools capability.
func (r *PluginRegistry) GetAllToolsPlugins(ctx context.Context) map[string]plugins.ToolsPlugin {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]plugins.ToolsPlugin)
	for name, driver := range r.drivers {
		if driver.Capabilities == nil || driver.Capabilities.Tools == nil {
			continue
		}
		tp, err := driver.Driver.GetToolsPlugin(ctx)
		if err != nil {
			r.logger.Warn("Failed to get tools plugin", "driver", name, "error", err)
			continue
		}
		result[name] = tp
	}
	return result
}
