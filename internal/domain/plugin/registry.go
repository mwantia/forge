package plugin

import (
	"context"

	plugins "github.com/mwantia/forge-sdk/pkg/plugin"
)

// PluginsRegistry is the narrow surface for looking up loaded plugin drivers.
type PluginsRegistry interface {
	GetDriver(name string) (*PluginDriver, bool)
	ListDrivers() []*PluginDriver

	GetPlugin(ctx context.Context, ptype PluginType, name string) (plugins.BasePlugin, error)
}
