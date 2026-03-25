package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/mwantia/forge/pkg/errors"
	"github.com/mwantia/forge/pkg/plugins"
)

const PluginName = "workspace"

func init() {
	plugins.Register(PluginName, NewWorkspaceDriver)
}

// WorkspaceDriver implements plugins.Driver for filesystem access.
type WorkspaceDriver struct {
	plugins.UnimplementedToolsPlugin
	log    hclog.Logger
	config *WorkspaceConfig
}

type WorkspaceConfig struct {
	Home      string   `mapstructure:"home"`
	Tools     []string `mapstructure:"tools"`
	Allowlist []string `mapstructure:"allowlist"`
}

func NewWorkspaceDriver(log hclog.Logger) plugins.Driver {
	return &WorkspaceDriver{
		log: log.Named(PluginName),
	}
}

func (d *WorkspaceDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:    PluginName,
		Author:  "forge",
		Version: "0.1.0",
	}
}

func (d *WorkspaceDriver) ProbePlugin(ctx context.Context) (bool, error) {
	return true, nil
}

func (d *WorkspaceDriver) GetCapabilities(ctx context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeTools},
		Tools: &plugins.ToolsCapabilities{
			SupportsAsyncExecution: false,
		},
	}, nil
}

func (d *WorkspaceDriver) OpenDriver(ctx context.Context) error {
	return nil
}

func (d *WorkspaceDriver) CloseDriver(ctx context.Context) error {
	return nil
}

func (d *WorkspaceDriver) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
	if err := mapstructure.Decode(config.ConfigMap, &d.config); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	if d.config.Home == "" {
		d.config.Home = "."
	}

	// Expand ~ to user home directory
	if strings.HasPrefix(d.config.Home, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		d.config.Home = filepath.Join(home, d.config.Home[1:])
	}

	// Resolve to absolute path
	abs, err := filepath.Abs(d.config.Home)
	if err != nil {
		return fmt.Errorf("failed to resolve home path: %w", err)
	}
	d.config.Home = abs

	info, err := os.Stat(d.config.Home)
	if err != nil {
		return fmt.Errorf("failed to access home directory '%s': %w", d.config.Home, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("home '%s' is not a directory", d.config.Home)
	}

	d.log.Info("Workspace configured", "home", d.config.Home, "tools", d.config.Tools, "allowlist", len(d.config.Allowlist))
	return nil
}

func (d *WorkspaceDriver) GetProviderPlugin(ctx context.Context) (plugins.ProviderPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *WorkspaceDriver) GetMemoryPlugin(ctx context.Context) (plugins.MemoryPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *WorkspaceDriver) GetChannelPlugin(ctx context.Context) (plugins.ChannelPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *WorkspaceDriver) GetToolsPlugin(ctx context.Context) (plugins.ToolsPlugin, error) {
	return d, nil
}
