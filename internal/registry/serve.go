package registry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/pkg/errors"
	"github.com/mwantia/forge/pkg/plugins"
	pluginsgrpc "github.com/mwantia/forge/pkg/plugins/grpc"
)

func (r *PluginRegistry) ServePlugins(ctx context.Context, dir string, cfgs []*config.PluginConfig) error {
	if dir == "" {
		// Default plugin directory
		dir = "./plugins"
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	errs := errors.Errors{}

	for _, cfg := range cfgs {
		info, err := r.GetPluginDriverInfo(cfg)
		if err != nil {
			errs.Add(err)
			continue
		}

		if err := r.servePlugin(ctx, dir, info); err != nil {
			errs.Add(err)
			continue
		}
	}

	return errs.Errors()
}

func (r *PluginRegistry) servePlugin(ctx context.Context, dir string, info PluginDriverInfo) error {
	path := filepath.Join(dir, info.Type)
	args := make([]string, 0)
	if _, err := os.Stat(path); err != nil {
		path, err = os.Executable()
		if err != nil {
			return fmt.Errorf("failed to execute embedded plugin: %w", err)
		}
		args = []string{"plugin", info.Type}
	}

	r.logger.Debug("Executing plugin", "path", path, "args", args)
	name := info.Type
	if info.Name != info.Type {
		name += "." + info.Name
	}
	driver, client, err := r.runPlugin(ctx, r.logger.Named(name), info, path, args...)
	if err != nil {
		return fmt.Errorf("failed to run plugin: %w", err)
	}

	caps, err := driver.GetCapabilities(ctx)
	if err != nil {
		return fmt.Errorf("failed to load driver capabilities: %w", err)
	}

	r.drivers[info.Name] = &PluginDriver{
		Name:         info.Name,
		Capabilities: caps,
		Driver:       driver,
		Cleanup:      func() { client.Kill() },
	}
	return nil
}

func (r *PluginRegistry) runPlugin(ctx context.Context, logger hclog.Logger, info PluginDriverInfo, path string, args ...string) (plugins.Driver, *goplugin.Client, error) {
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig: pluginsgrpc.Handshake,
		Plugins:         pluginsgrpc.Plugins,
		AllowedProtocols: []goplugin.Protocol{
			goplugin.ProtocolGRPC,
		},
		Cmd:    exec.Command(path, args...),
		Logger: logger,
	})

	grpc, err := client.Client()
	if err != nil {
		// Check if the plugin process exited
		if exitErr := client.Exited(); exitErr {
			return nil, client, fmt.Errorf("plugin process exited unexpectedly: %w", err)
		}

		client.Kill()
		return nil, client, fmt.Errorf("failed to get gRPC client: %w", err)
	}

	raw, err := grpc.Dispense("driver")
	if err != nil {
		// Check if the plugin process exited
		if exitErr := client.Exited(); exitErr {
			return nil, client, fmt.Errorf("plugin process exited during dispense: %w", err)
		}

		client.Kill()
		return nil, client, fmt.Errorf("failed to dispense driver plugin: %w", err)
	}

	driver, ok := raw.(plugins.Driver)
	if !ok {
		client.Kill()
		return nil, client, fmt.Errorf("failed to cast grpc interface as driver")
	}

	if len(info.Config) > 0 {
		if err := driver.ConfigDriver(ctx, plugins.PluginConfig{ConfigMap: info.Config}); err != nil {
			client.Kill()
			return nil, client, fmt.Errorf("failed to configure driver: %w", err)
		}
	}

	if err := driver.OpenDriver(ctx); err != nil {
		client.Kill()
		return nil, client, fmt.Errorf("failed to open driver connections: %w", err)
	}

	return driver, client, nil
}

func (r *PluginRegistry) GetPluginDriverInfo(cfg *config.PluginConfig) (PluginDriverInfo, error) {
	info := PluginDriverInfo{
		Name: cfg.Name,
		Type: cfg.Type,
	}
	// Overwrite empty name with type
	if info.Name == "" {
		info.Name = info.Type
	}

	body, err := cfg.Config.DecodeBody()
	if err != nil {
		return info, err
	}

	info.Config = body
	return info, nil
}
