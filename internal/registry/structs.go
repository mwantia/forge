package registry

import (
	"time"

	"github.com/mwantia/forge-sdk/pkg/plugins"
)

type PluginDriver struct {
	Info         PluginDriverInfo
	Capabilities *plugins.DriverCapabilities
	Driver       plugins.Driver
	Cleanup      PluginDriverCleanup
}

type PluginDriverCleanup func()

type PluginDriverInfo struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
	Path    string         `json:"path"`
	Args    []string       `json:"args"`
	Timeout time.Duration  `json:"timeout"`
	MinPort uint           `json:"min_port"`
	MaxPort uint           `json:"max_port"`
	Env     map[string]any `json:"env"`
	Config  map[string]any `json:"config"`
}
