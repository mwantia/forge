package plugins

import (
	"context"

	"github.com/mwantia/forge/pkg/plugins/proto"
)

// Lifecycle provides access to driver-level lifecycle checks.
// Plugins use this to reference back to their parent driver.
type Lifecycle interface {
	Name() string
	ProbePlugin(ctx context.Context) (bool, error)
	GetCapabilities(ctx context.Context) (*proto.DriverCapabilities, error)
}

// Driver is the main interface that plugins implement.
// A single driver can support multiple plugin types simultaneously.
type Driver interface {
	Lifecycle

	// Lifecycle management
	OpenDriver(ctx context.Context) error
	CloseDriver(ctx context.Context) error

	// Configuration
	ConfigDriver(ctx context.Context, config PluginConfig) error

	// Plugin type accessors - return implementations only if supported
	GetProviderPlugin(ctx context.Context) (ProviderPlugin, error)
	GetMemoryPlugin(ctx context.Context) (MemoryPlugin, error)
	GetChannelPlugin(ctx context.Context) (ChannelPlugin, error)
	GetToolsPlugin(ctx context.Context) (ToolsPlugin, error)
}
