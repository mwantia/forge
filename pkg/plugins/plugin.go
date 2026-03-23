package plugins

import "github.com/mwantia/forge/pkg/plugins/proto"

// BasePlugin is the common interface all plugins share.
// Plugins can reference their parent driver via GetLifecycle.
type BasePlugin interface {
	GetLifecycle() Lifecycle
	GetPluginInfo() *proto.PluginInfo
}

type PluginConfig struct {
	ConfigMap map[string]any `json:"-"`
}
