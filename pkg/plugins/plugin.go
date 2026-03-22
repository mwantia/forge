package plugins

// BasePlugin is the common interface all plugins share.
// Plugins can reference their parent driver via GetLifecycle.
type BasePlugin interface {
	GetLifecycle() Lifecycle
	GetPluginInfo() *PluginInfo
}

type PluginConfig struct {
	ConfigMap map[string]any `json:"-"`
}
