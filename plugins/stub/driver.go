package stub

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/pkg/plugins"
	"github.com/mwantia/forge/pkg/plugins/proto"
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

// NewStubDriver creates a new stub driver that supports all plugin types.
func NewStubDriver(log hclog.Logger) plugins.Driver {
	return &StubDriver{
		log: log.Named(PluginName),
	}
}

// Lifecycle methods
func (d *StubDriver) Name() string {
	return PluginName
}

func (d *StubDriver) ProbePlugin(ctx context.Context) (bool, error) {
	return true, nil
}

func (d *StubDriver) GetCapabilities(ctx context.Context) (*proto.DriverCapabilities, error) {
	return &proto.DriverCapabilities{
		Types: []string{
			plugins.PluginTypeProvider,
			plugins.PluginTypeMemory,
			plugins.PluginTypeChannel,
			plugins.PluginTypeTools,
		},
		Provider: &proto.ProviderCaps{
			SupportsStreaming: true,
			SupportsVision:    false,
		},
		Memory: &proto.MemoryCaps{
			SupportsVectorSearch: false,
			MaxContextSize:       4096,
		},
		Channel: &proto.ChannelCaps{
			SupportsDirectMessages: true,
			SupportsThreads:        true,
		},
		Tools: &proto.ToolsCaps{
			SupportsAsyncExecution: false,
		},
	}, nil
}

// Lifecycle management
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

// Plugin type accessors
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
