package plugins

import (
	"context"
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/plugins"
)

type PluginsRegistry interface {
	GetDriver(name string) (*PluginDriver, bool)
	ListDrivers() []*PluginDriver

	GetPlugin(ctx context.Context, ptype PluginType, name string) (plugins.BasePlugin, error)
}

type PluginType string

const (
	PluginTypeChannel  PluginType = "channel"
	PluginTypeMemory   PluginType = "memory"
	PluginTypeSessions PluginType = "sessions"
	PluginTypeProvider PluginType = "provider"
	PluginTypeTools    PluginType = "tools"
	PluginTypeSandbox  PluginType = "sandbox"
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
	case PluginTypeMemory:
		return driver.Driver.GetMemoryPlugin(ctx)
	case PluginTypeSessions:
		return driver.Driver.GetSessionsPlugin(ctx)
	case PluginTypeProvider:
		return driver.Driver.GetProviderPlugin(ctx)
	case PluginTypeTools:
		return driver.Driver.GetToolsPlugin(ctx)
	case PluginTypeSandbox:
		return driver.Driver.GetSandboxPlugin(ctx)
	}

	return nil, fmt.Errorf("invalid plugin type defined: %q", ptype)
}
