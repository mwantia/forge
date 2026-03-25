package registry

import "github.com/mwantia/forge/pkg/plugins"

type PluginDriver struct {
	Name         string
	Capabilities *plugins.DriverCapabilities
	Driver       plugins.Driver
	Cleanup      PluginDriverCleanup
}

type PluginDriverCleanup func()

type PluginDriverInfo struct {
	Name   string
	Type   string
	Config map[string]any
}
