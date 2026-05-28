package plugins

import (
	"context"
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	domplugin "github.com/mwantia/forge/internal/domain/plugin"
)

type (
	PluginsRegistry = domplugin.PluginsRegistry
	PluginType      = domplugin.PluginType
)

const (
	PluginTypeChannel  = domplugin.PluginTypeChannel
	PluginTypeResource = domplugin.PluginTypeResource
	PluginTypeProvider = domplugin.PluginTypeProvider
	PluginTypeTools    = domplugin.PluginTypeTools
	PluginTypeSandbox  = domplugin.PluginTypeSandbox
)

func (s *PluginsService) GetDriver(name string) (*PluginDriver, bool) {
	driver, ok := s.drivers[name]
	return driver, ok
}

func (s *PluginsService) ListDrivers() []*PluginDriver {
	s.mu.RLock()
	defer s.mu.RUnlock()

	drivers := make([]*PluginDriver, 0, len(s.drivers))
	for _, d := range s.drivers {
		drivers = append(drivers, d)
	}
	return drivers
}

func (s *PluginsService) GetPlugin(ctx context.Context, ptype PluginType, name string) (plugins.BasePlugin, error) {
	driver, ok := s.GetDriver(name)
	if !ok {
		return nil, fmt.Errorf("unable to find driver with name %q", name)
	}

	switch ptype {
	case PluginTypeChannel:
		return driver.Driver.GetChannelPlugin(ctx)
	case PluginTypeResource:
		return driver.Driver.GetResourcePlugin(ctx)
	case PluginTypeProvider:
		return driver.Driver.GetProviderPlugin(ctx)
	case PluginTypeTools:
		return driver.Driver.GetToolsPlugin(ctx)
	case PluginTypeSandbox:
		return driver.Driver.GetSandboxPlugin(ctx)
	}

	return nil, fmt.Errorf("invalid plugin type defined: %q", ptype)
}
