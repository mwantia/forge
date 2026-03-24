package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/hcl/v2"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/pkg/errors"
	"github.com/mwantia/forge/pkg/plugins"
	pluginsgrpc "github.com/mwantia/forge/pkg/plugins/grpc"
	"github.com/zclconf/go-cty/cty"
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
		cm, err := r.GetPluginConfigMap(cfg.Config)
		if err != nil {
			errs.Add(err)
			continue
		}

		if err := r.servePlugin(ctx, cfg.Name, dir, cm); err != nil {
			errs.Add(err)
			continue
		}
	}

	return errs.Errors()
}

func (r *PluginRegistry) servePlugin(ctx context.Context, name, dir string, cm map[string]any) error {
	path := filepath.Join(dir, name)
	args := make([]string, 0)
	if _, err := os.Stat(path); err != nil {
		path, err = os.Executable()
		if err != nil {
			return fmt.Errorf("failed to execute embedded plugin: %w", err)
		}
		args = []string{"plugin", name}
	}

	r.logger.Debug("Executing plugin", "path", path, "args", args)
	driver, client, err := r.runPlugin(ctx, r.logger.Named(name), cm, path, args...)
	if err != nil {
		return fmt.Errorf("failed to run plugin: %w", err)
	}

	caps, err := driver.GetCapabilities(ctx)
	if err != nil {
		return fmt.Errorf("failed to load driver capabilities: %w", err)
	}

	r.drivers[name] = &PluginDriver{
		Name:         name,
		Capabilities: caps,
		Driver:       driver,
		Cleanup:      func() { client.Kill() },
	}
	return nil
}

func (r *PluginRegistry) runPlugin(ctx context.Context, logger hclog.Logger, cm map[string]any, path string, args ...string) (plugins.Driver, *goplugin.Client, error) {
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

	if err := driver.OpenDriver(ctx); err != nil {
		client.Kill()
		return nil, client, fmt.Errorf("failed to open driver connections: %w", err)
	}

	if len(cm) > 0 {
		if err := driver.ConfigDriver(ctx, plugins.PluginConfig{ConfigMap: cm}); err != nil {
			client.Kill()
			return nil, client, fmt.Errorf("failed to configure driver: %w", err)
		}
	}

	return driver, client, nil
}

func (r *PluginRegistry) GetPluginConfigMap(cfg *config.PluginConfigBody) (map[string]any, error) {
	cm := make(map[string]any, 0)
	// Return empty map if undefined config
	if cfg == nil || cfg.Body == nil {
		return cm, nil
	}
	attrs, diags := cfg.Body.JustAttributes()
	if diags.HasErrors() {
		return cm, fmt.Errorf("invalid plugin config: %v", diags.Error())
	}

	for name, attr := range attrs {
		value, diags := attr.Expr.Value(&hcl.EvalContext{})
		if diags.HasErrors() {
			return cm, fmt.Errorf("invalid plugin config: %v", diags.Error())
		}

		cm[name] = ctyValueToGo(value)
	}

	return cm, nil
}

// ctyValueToGo converts a cty.Value to a plain Go value suitable for mapstructure decoding.
func ctyValueToGo(value cty.Value) any {
	ty := value.Type()

	switch {
	case ty == cty.String:
		return value.AsString()
	case ty == cty.Number:
		f, _ := value.AsBigFloat().Float64()
		return f
	case ty == cty.Bool:
		return value.True()
	case ty.IsListType() || ty.IsTupleType() || ty.IsSetType():
		var result []any
		for it := value.ElementIterator(); it.Next(); {
			_, v := it.Element()
			result = append(result, ctyValueToGo(v))
		}
		return result
	case ty.IsObjectType() || ty.IsMapType():
		result := make(map[string]any)
		for it := value.ElementIterator(); it.Next(); {
			k, v := it.Element()
			result[k.AsString()] = ctyValueToGo(v)
		}
		return result
	default:
		return value.GoString()
	}
}
