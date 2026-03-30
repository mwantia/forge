package registry

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge-sdk/pkg/plugins"
)

type PluginRegistry struct {
	mutex   sync.RWMutex
	drivers map[string]*PluginDriver

	logger hclog.Logger `fabric:"logger:reg"`
}

func (r *PluginRegistry) Provider() *PluginProviderNamespace {
	return &PluginProviderNamespace{
		registry: r,
	}
}

func (r *PluginRegistry) CleanupDrivers() {
	for _, driver := range r.drivers {
		driver.Cleanup()
	}
}

func (r *PluginRegistry) GetProvider(ctx context.Context, name string) (plugins.ProviderPlugin, error) {
	driver, ok := r.drivers[strings.ToLower(name)]
	if ok {
		return driver.Driver.GetProviderPlugin(ctx)
	}

	return nil, fmt.Errorf("unknown provider plugin name defined")
}

func (r *PluginRegistry) GetToolsPlugin(ctx context.Context, name string) (plugins.ToolsPlugin, error) {
	driver, ok := r.drivers[strings.ToLower(name)]
	if ok {
		return driver.Driver.GetToolsPlugin(ctx)
	}

	return nil, fmt.Errorf("unknown tools plugin name defined")
}

func (r *PluginRegistry) GetSandboxPlugin(ctx context.Context, name string) (plugins.SandboxPlugin, error) {
	driver, ok := r.drivers[strings.ToLower(name)]
	if ok {
		return driver.Driver.GetSandboxPlugin(ctx)
	}

	return nil, fmt.Errorf("unknown sandbox plugin name defined")
}

// GetAllToolsPlugins returns a map of driver name → ToolsPlugin for every loaded
// driver that advertises tools capability.
func (r *PluginRegistry) ListDrivers() []*PluginDriver {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make([]*PluginDriver, 0, len(r.drivers))
	for _, d := range r.drivers {
		result = append(result, d)
	}
	return result
}

func (r *PluginRegistry) GetDriver(name string) *PluginDriver {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.drivers[strings.ToLower(name)]
}

func (r *PluginRegistry) GetAllProviders(ctx context.Context) map[string]plugins.ProviderPlugin {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]plugins.ProviderPlugin)
	for name, driver := range r.drivers {
		if driver.Capabilities == nil || driver.Capabilities.Provider == nil {
			continue
		}
		p, err := driver.Driver.GetProviderPlugin(ctx)
		if err != nil {
			r.logger.Warn("Failed to get provider plugin", "driver", name, "error", err)
			continue
		}
		result[name] = p
	}
	return result
}

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
