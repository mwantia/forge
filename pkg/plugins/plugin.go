package plugins

// PluginInfo describes plugin metadata at the driver level.
type PluginInfo struct {
	Name    string
	Author  string
	Version string
}

// BasePlugin is the common interface all sub-plugins share.
// Sub-plugins reference their parent driver via GetLifecycle.
type BasePlugin interface {
	GetLifecycle() Lifecycle
}

// PluginConfig holds driver configuration as a generic map.
type PluginConfig struct {
	ConfigMap map[string]any `json:"-"`
}
