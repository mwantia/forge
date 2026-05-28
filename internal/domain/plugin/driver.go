package plugin

import (
	"time"

	"github.com/mwantia/forge-sdk/pkg/plugins"
)

// PluginType identifies the role a plugin serves.
type PluginType string

const (
	PluginTypeChannel  PluginType = "channel"
	PluginTypeResource PluginType = "resource"
	PluginTypeProvider PluginType = "provider"
	PluginTypeTools    PluginType = "tools"
	PluginTypeSandbox  PluginType = "sandbox"
)

// PluginDriverCleanup tears down the subprocess that backs a plugin driver.
type PluginDriverCleanup func()

// PluginDriverInfo is the resolved launch parameters for one plugin instance.
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

// PluginDriver is a live, connected plugin subprocess with its metadata.
type PluginDriver struct {
	Info         PluginDriverInfo
	Capabilities *plugins.DriverCapabilities
	Driver       plugins.Driver
	Cleanup      PluginDriverCleanup
}
