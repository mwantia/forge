package plugins

import goplugin "github.com/hashicorp/go-plugin"

const (
	// A provider plugin acts as LLM provider to "provide" access to endpoints like Ollama, Anthropic, etc.
	PluginTypeProvider string = "provider"
	// A memory plugin acts as memory management for endpoints like OpenViking.
	PluginTypeMemory string = "memory"
	// A channel plugin acts as communication gateway for endpoints like Discord.
	PluginTypeChannel string = "channel"
	// A tools plugin acts as bridge (or summary of embedded tools) for tool calling.
	PluginTypeTools string = "tools"
)

// Handshake is the plugin handshake configuration.
// Plugins and hosts must use the same values to communicate.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  2,
	MagicCookieKey:   "FORGE_PLUGIN",
	MagicCookieValue: "forge",
}

var Plugins = map[string]goplugin.Plugin{
	"driver": &DriverPlugin{},
}
