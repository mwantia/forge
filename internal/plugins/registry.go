package plugins

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/pkg/plugins"
	"github.com/mwantia/forge/pkg/plugins/proto"
)

type PluginRegistry struct {
	mutex sync.RWMutex

	logger  hclog.Logger
	drivers map[string]*PluginDriver
}

type PluginDriver struct {
	Name         string
	Capabilities *proto.DriverCapabilities
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

func (r *PluginRegistry) GetToolsPlugins(ctx context.Context, name string) (plugins.ProviderPlugin, error) {
	driver, ok := r.drivers[strings.ToLower(name)]
	if ok {
		return driver.Driver.GetProviderPlugin(ctx)
	}

	return nil, fmt.Errorf("unknown provider name defined")
}
