package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	pluginsgrpc "github.com/mwantia/forge-sdk/pkg/plugins/grpc"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/service/metrics"
)

type PluginsServer interface {
	ServePluginsFrom(ctx context.Context, dir string) error
	ServePlugin(ctx context.Context, dir string, info PluginDriverInfo) error
	Cleanup(ctx context.Context) error
}

type PluginDriver struct {
	Info         PluginDriverInfo
	Capabilities *plugins.DriverCapabilities
	Driver       plugins.Driver
	Cleanup      PluginDriverCleanup
}

func (s *PluginsService) Cleanup(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, driver := range s.drivers {
		s.logger.Debug("Killing plugin", "name", name)
		driver.Cleanup()
	}
	return nil
}

func (s *PluginsService) ServePluginsFrom(ctx context.Context, dir string) error {
	if dir == "" {
		// Default plugin directory
		dir = "./plugins"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, cfg := range s.configs {
		info, err := s.GetPluginDriverInfo(cfg)
		if err != nil {
			return fmt.Errorf("failed to load driver config for %q: %w", cfg.Name, err)
		}
		// Allow plugins to be disabled via config
		if info.Enabled {
			if err := s.ServePlugin(ctx, dir, info); err != nil {
				return fmt.Errorf("failed to serve plugin %q: %w", cfg.Name, err)
			}
		}
	}

	return nil
}

func (s *PluginsService) ServePlugin(ctx context.Context, dir string, info PluginDriverInfo) error {
	if info.Path == "" {
		path := filepath.Join(dir, info.Type)
		if _, err := os.Stat(path); err != nil {
			path, err = os.Executable()
			if err != nil {
				return fmt.Errorf("failed to execute embedded plugin: %w", err)
			}
			info.Args = []string{"plugin", info.Type}
		}
		info.Path = path
	}

	s.logger.Debug("Executing plugin", "path", info.Path, "args", info.Args, "env", len(info.Env))
	name := info.Type
	if info.Name != info.Type {
		name += "." + info.Name
	}

	start := time.Now()
	driver, client, err := s.runPlugin(ctx, s.logger.Named(name), info)
	PluginsServeDuration.WithLabelValues(info.Type).Observe(time.Since(start).Seconds())
	PluginsServeTotal.WithLabelValues(info.Type, metrics.ErrToStatusLabel(err)).Inc()
	if err != nil {
		return fmt.Errorf("failed to run plugin: %w", err)
	}

	caps, err := driver.GetCapabilities(ctx)
	if err != nil {
		return fmt.Errorf("failed to load driver capabilities: %w", err)
	}

	s.drivers[info.Name] = &PluginDriver{
		Info:         info,
		Capabilities: caps,
		Driver:       driver,
		Cleanup:      func() { client.Kill() },
	}

	PluginsLoaded.WithLabelValues(info.Type).Inc()
	return nil
}

func (s *PluginsService) GetPluginDriverInfo(cfg PluginConfig) (PluginDriverInfo, error) {
	info := PluginDriverInfo{
		Name:    cfg.Name,
		Type:    cfg.Type,
		Enabled: !cfg.Disabled,
		Timeout: time.Minute * 1, // See goplugin.ClientConfig.StartTimeout
		MinPort: 10000,           // See goplugin.ClientConfig.MinPort
		MaxPort: 25000,           // See goplugin.ClientConfig.MaxPort
		Env:     make(map[string]any),
		Config:  make(map[string]any),
	}
	// Overwrite empty name with type
	if info.Name == "" {
		info.Name = info.Type
	}

	tmpl := s.tmpl.Base()

	if cfg.Runtime != nil {
		info.Path = cfg.Runtime.Path
		info.Args = cfg.Runtime.Args

		if cfg.Runtime.Timeout != "" {
			timeout, err := time.ParseDuration(cfg.Runtime.Timeout)
			if err == nil || timeout > 0 {
				info.Timeout = timeout
			}
		}

		if cfg.Runtime.Port != nil {
			info.MinPort = cfg.Runtime.Port.Min
			info.MaxPort = cfg.Runtime.Port.Max
		}

		if cfg.Runtime.Env != nil {
			env, err := config.DecodeBody(cfg.Runtime.Env.Body, tmpl)
			if err != nil {
				return info, err
			}
			info.Env = env
		}
	}

	if cfg.Config != nil {
		cfgMap, err := config.DecodeBody(cfg.Config.Body, tmpl)
		if err != nil {
			return info, err
		}
		info.Config = cfgMap
	}

	return info, nil
}

func (s *PluginsService) runPlugin(ctx context.Context, logger hclog.Logger, info PluginDriverInfo) (plugins.Driver, *goplugin.Client, error) {
	config := &goplugin.ClientConfig{
		HandshakeConfig: pluginsgrpc.Handshake,
		Plugins:         pluginsgrpc.Plugins,
		AllowedProtocols: []goplugin.Protocol{
			goplugin.ProtocolGRPC,
		},
		StartTimeout: info.Timeout,
		MinPort:      info.MinPort,
		MaxPort:      info.MaxPort,
		Cmd:          exec.Command(info.Path, info.Args...),
		Logger:       logger,
		SkipHostEnv:  true,
	}

	if len(info.Env) > 0 {
		env := make([]string, 0)
		for k, v := range info.Env {
			env = append(env, strings.ToUpper(k)+"="+fmt.Sprintf("%s", v))
		}
		config.Cmd.Env = env
	}

	client := goplugin.NewClient(config)

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
