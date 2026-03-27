package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/mwantia/forge/pkg/errors"
	"github.com/mwantia/forge/pkg/plugins"
)

const PluginName = "mcp"

func init() {
	plugins.Register(PluginName, NewMCPDriver)
}

type MCPDriver struct {
	plugins.UnimplementedToolsPlugin
	log     hclog.Logger
	config  *MCPConfig
	servers map[string]*mcpServer
	mu      sync.RWMutex
}

func NewMCPDriver(log hclog.Logger) plugins.Driver {
	return &MCPDriver{
		log: log.Named(PluginName),
	}
}

func (d *MCPDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:    PluginName,
		Author:  "forge",
		Version: "0.1.0",
	}
}

func (d *MCPDriver) ProbePlugin(_ context.Context) (bool, error) {
	return true, nil
}

func (d *MCPDriver) GetCapabilities(_ context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeTools},
		Tools: &plugins.ToolsCapabilities{
			SupportsAsyncExecution: false,
		},
	}, nil
}

func (d *MCPDriver) ConfigDriver(_ context.Context, config plugins.PluginConfig) error {
	cfg := &MCPConfig{}
	if err := mapstructure.Decode(config.ConfigMap, cfg); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}
	d.config = cfg
	return nil
}

func (d *MCPDriver) OpenDriver(ctx context.Context) error {
	if d.config == nil {
		return nil
	}

	d.mu.Lock()
	d.servers = make(map[string]*mcpServer, len(d.config.Servers))
	d.mu.Unlock()

	for name, cfg := range d.config.Servers {
		srv := newMCPServer(name, cfg, d.log)
		if err := srv.connect(ctx); err != nil {
			d.log.Warn("Failed to connect to MCP server", "server", name, "error", err)
			continue
		}
		d.mu.Lock()
		d.servers[name] = srv
		d.mu.Unlock()
		d.log.Info("Connected to MCP server", "server", name, "tools", len(srv.tools))
	}

	return nil
}

func (d *MCPDriver) CloseDriver(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for name, srv := range d.servers {
		if err := srv.close(); err != nil {
			d.log.Warn("Error closing MCP server connection", "server", name, "error", err)
		}
	}
	d.servers = nil
	return nil
}

func (d *MCPDriver) GetProviderPlugin(_ context.Context) (plugins.ProviderPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *MCPDriver) GetMemoryPlugin(_ context.Context) (plugins.MemoryPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *MCPDriver) GetChannelPlugin(_ context.Context) (plugins.ChannelPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *MCPDriver) GetToolsPlugin(_ context.Context) (plugins.ToolsPlugin, error) {
	return d, nil
}
