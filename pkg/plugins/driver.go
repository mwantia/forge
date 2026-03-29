package plugins

import "context"

// DriverCapabilities describes what a driver supports.
type DriverCapabilities struct {
	Types    []string
	Provider *ProviderCapabilities
	Memory   *MemoryCapabilities
	Channel  *ChannelCapabilities
	Tools    *ToolsCapabilities
	Sandbox  *SandboxCapabilities
}

type ProviderCapabilities struct {
	SupportsStreaming bool
	SupportsVision    bool
}

type MemoryCapabilities struct {
	SupportsVectorSearch bool
	SupportSessions      bool
	MaxContextSize       int
}

type ChannelCapabilities struct {
	SupportsDirectMessages bool
	SupportsThreads        bool
}

type ToolsCapabilities struct {
	SupportsAsyncExecution bool
}

// PluginInfo describes plugin metadata at the driver level.
type PluginInfo struct {
	Name        string `json:"name"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// PluginConfig holds driver configuration as a generic map.
type PluginConfig struct {
	ConfigMap map[string]any `json:"-"`
}

// Lifecycle provides access to driver-level lifecycle checks.
// Plugins use this to reference back to their parent driver.
type Lifecycle interface {
	GetPluginInfo() PluginInfo
	ProbePlugin(ctx context.Context) (bool, error)
	GetCapabilities(ctx context.Context) (*DriverCapabilities, error)
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
	GetSandboxPlugin(ctx context.Context) (SandboxPlugin, error)
}
