package plugins

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
