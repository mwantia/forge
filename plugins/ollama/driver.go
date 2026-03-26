package ollama

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

const PluginName = "ollama"

func init() {
	plugins.Register(PluginName, NewOllamaDriver)
}

// OllamaDriver implements plugins.Driver for the Ollama LLM provider.
type OllamaDriver struct {
	log          hclog.Logger
	config       *OllamaConfig
	client       *http.Client // used for unary requests (probe, embed, model ops)
	streamClient *http.Client // no timeout — context controls cancellation for streams
}

// NewOllamaDriver creates a new Ollama driver that supports provider plugin type.
func NewOllamaDriver(log hclog.Logger) plugins.Driver {
	cfg := DefaultConfig()
	return &OllamaDriver{
		log:          log.Named(PluginName),
		config:       cfg,
		client:       &http.Client{},
		streamClient: &http.Client{},
	}
}

func (d *OllamaDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:    PluginName,
		Author:  "forge",
		Version: "0.1.0",
	}
}

func (d *OllamaDriver) ProbePlugin(ctx context.Context) (bool, error) {
	if d.config == nil || d.config.Address == "" {
		return false, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.config.Address+"/api/tags", nil)
	if err != nil {
		return false, fmt.Errorf("failed to create probe request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		d.log.Debug("Ollama probe failed", "error", err)
		return false, nil
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (d *OllamaDriver) GetCapabilities(ctx context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeProvider},
		Provider: &plugins.ProviderCapabilities{
			SupportsStreaming: true,
			SupportsVision:    false,
		},
	}, nil
}

func (d *OllamaDriver) OpenDriver(ctx context.Context) error {
	return nil
}

func (d *OllamaDriver) CloseDriver(ctx context.Context) error {
	return nil
}

func (d *OllamaDriver) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
	cfg := DefaultConfig()

	if err := mapstructure.Decode(config.ConfigMap, cfg); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	d.config = cfg

	timeout := parseDuration(cfg.Timeout, 60*time.Second)
	d.client = &http.Client{Timeout: timeout}
	d.streamClient = &http.Client{}

	d.log.Info("Configured Ollama driver",
		"address", cfg.Address,
		"timeout", timeout,
	)

	for name, m := range cfg.Models {
		d.log.Debug("Registered model aliases",
			"alias", name,
			"base_model", m.BaseModel,
			"reasoning", m.Reasoning,
			"system", fmt.Sprintf("len:%d", len(m.System)),
		)
	}

	return nil
}

func (d *OllamaDriver) GetProviderPlugin(ctx context.Context) (plugins.ProviderPlugin, error) {
	return &OllamaProviderPlugin{driver: d}, nil
}

func (d *OllamaDriver) GetMemoryPlugin(ctx context.Context) (plugins.MemoryPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *OllamaDriver) GetChannelPlugin(ctx context.Context) (plugins.ChannelPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *OllamaDriver) GetToolsPlugin(ctx context.Context) (plugins.ToolsPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
