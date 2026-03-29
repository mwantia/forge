package searxng

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/mwantia/forge/pkg/errors"
	"github.com/mwantia/forge/pkg/plugins"
)

const PluginName = "searxng"

const PluginDescription = "SearXNG metasearch engine for privacy-respecting web search"

func init() {
	plugins.Register(PluginName, PluginDescription, NewSearXNGDriver)
}

type SearXNGDriver struct {
	plugins.UnimplementedToolsPlugin
	log    hclog.Logger
	config *SearXNGConfig
	client *http.Client
}

type SearXNGConfig struct {
	Address    string   `mapstructure:"address"`
	Timeout    int      `mapstructure:"timeout"`
	MaxResults int      `mapstructure:"max_results"`
	Tools      []string `mapstructure:"tools"`
}

func NewSearXNGDriver(log hclog.Logger) plugins.Driver {
	return &SearXNGDriver{
		log: log.Named(PluginName),
	}
}

func (d *SearXNGDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:        PluginName,
		Author:      "forge",
		Version:     "0.1.0",
		Description: PluginDescription,
	}
}

func (d *SearXNGDriver) ProbePlugin(ctx context.Context) (bool, error) {
	return true, nil
}

func (d *SearXNGDriver) GetCapabilities(ctx context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeTools},
		Tools: &plugins.ToolsCapabilities{
			SupportsAsyncExecution: false,
		},
	}, nil
}

func (d *SearXNGDriver) OpenDriver(ctx context.Context) error {
	return nil
}

func (d *SearXNGDriver) CloseDriver(ctx context.Context) error {
	return nil
}

func (d *SearXNGDriver) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
	cfg := &SearXNGConfig{}

	if err := mapstructure.Decode(config.ConfigMap, cfg); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	if cfg.Address == "" {
		cfg.Address = "http://localhost:8080"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30
	}
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = 10
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = []string{"web_search", "web_fetch"}
	}

	d.config = cfg
	d.client = &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}

	d.log.Info("SearXNG configured", "address", cfg.Address, "tools", cfg.Tools, "max_results", cfg.MaxResults)
	return nil
}

func (d *SearXNGDriver) GetProviderPlugin(ctx context.Context) (plugins.ProviderPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *SearXNGDriver) GetMemoryPlugin(ctx context.Context) (plugins.MemoryPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *SearXNGDriver) GetChannelPlugin(ctx context.Context) (plugins.ChannelPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *SearXNGDriver) GetToolsPlugin(ctx context.Context) (plugins.ToolsPlugin, error) {
	return d, nil
}

func (d *SearXNGDriver) GetSandboxPlugin(_ context.Context) (plugins.SandboxPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}
