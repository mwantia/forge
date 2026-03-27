package mcp

type MCPConfig struct {
	Servers map[string]MCPServerConfig `mapstructure:"server"`
}

type MCPServerConfig struct {
	Transport string            `mapstructure:"transport"` // "sse" or "stdio"
	URL       string            `mapstructure:"url"`       // SSE only
	Command   string            `mapstructure:"command"`   // stdio only
	Args      []string          `mapstructure:"args"`      // stdio, optional
	Headers   map[string]string `mapstructure:"headers"`   // SSE, optional
}
