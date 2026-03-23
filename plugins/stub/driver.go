package stub

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/pkg/plugins"
)

const PluginName = "stub"

func init() {
	plugins.Register(PluginName, NewStubDriver)
}

// StubDriver is a reference implementation of the Driver interface.
// It demonstrates how to implement a multi-type plugin.
type StubDriver struct {
	log hclog.Logger
}

func NewStubDriver(log hclog.Logger) plugins.Driver {
	return &StubDriver{log: log.Named(PluginName)}
}

func (d *StubDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:    PluginName,
		Author:  "forge",
		Version: "0.1.0",
	}
}

func (d *StubDriver) ProbePlugin(ctx context.Context) (bool, error) {
	return true, nil
}

func (d *StubDriver) GetCapabilities(ctx context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{
			plugins.PluginTypeProvider,
			plugins.PluginTypeMemory,
			plugins.PluginTypeChannel,
			plugins.PluginTypeTools,
		},
		Provider: &plugins.ProviderCaps{
			SupportsStreaming: true,
			SupportsVision:   false,
		},
		Memory: &plugins.MemoryCaps{
			SupportsVectorSearch: false,
			MaxContextSize:       4096,
		},
		Channel: &plugins.ChannelCaps{
			SupportsDirectMessages: true,
			SupportsThreads:        true,
		},
		Tools: &plugins.ToolsCaps{
			SupportsAsyncExecution: false,
		},
	}, nil
}

func (d *StubDriver) OpenDriver(ctx context.Context) error {
	d.log.Debug("Opening stub driver...")
	return nil
}

func (d *StubDriver) CloseDriver(ctx context.Context) error {
	d.log.Debug("Closing stub driver...")
	return nil
}

func (d *StubDriver) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
	d.log.Debug("Configuring stub driver...")
	return nil
}

func (d *StubDriver) GetProviderPlugin(ctx context.Context) (plugins.ProviderPlugin, error) {
	return &StubProviderPlugin{driver: d}, nil
}

func (d *StubDriver) GetMemoryPlugin(ctx context.Context) (plugins.MemoryPlugin, error) {
	return &StubMemoryPlugin{driver: d}, nil
}

func (d *StubDriver) GetChannelPlugin(ctx context.Context) (plugins.ChannelPlugin, error) {
	return &StubChannelPlugin{driver: d}, nil
}

func (d *StubDriver) GetToolsPlugin(ctx context.Context) (plugins.ToolsPlugin, error) {
	return &StubToolsPlugin{driver: d}, nil
}
