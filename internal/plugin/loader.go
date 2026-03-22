package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/mwantia/forge/pkg/log"
	"github.com/mwantia/forge/pkg/plugins"
	"github.com/zclconf/go-cty/cty"
)

// LoadResult holds the result of loading a single plugin.
type LoadResult struct {
	Name     string
	Duration time.Duration
	Error    error
}

// Loader manages plugin loading and lifecycle.
type Loader struct {
	log     log.Logger
	clients map[string]*plugins.Client // Plugin clients for subprocess management
	drivers map[string]plugins.Driver  // Connected drivers
}

// NewLoader creates a new plugin loader.
func NewLoader() *Loader {
	return &Loader{
		log:     log.Named("plugin"),
		clients: make(map[string]*plugins.Client),
		drivers: make(map[string]plugins.Driver),
	}
}

// LoadAll loads plugins from the specified directory with given configurations.
// Each plugin is spawned as a separate process and communicates via gRPC.
func (l *Loader) LoadAll(ctx context.Context, pluginDir string, pluginConfigs map[string]map[string]any) []LoadResult {
	results := make([]LoadResult, 0)

	// Default plugin directory
	if pluginDir == "" {
		pluginDir = "./plugins"
	}

	for name, config := range pluginConfigs {
		driver, err := l.loadPlugin(ctx, pluginDir, name, config)
		results = append(results, LoadResult{
			Name:  name,
			Error: err,
		})
		if err != nil {
			l.log.Warn("Failed to load plugin", "name", name, "error", err)
			continue
		}
		l.drivers[name] = driver
	}

	l.log.Info("Loaded plugins", "count", len(l.drivers))
	return results
}

// loadPlugin spawns a plugin process and connects via gRPC.
func (l *Loader) loadPlugin(ctx context.Context, pluginDir, name string, config map[string]any) (plugins.Driver, error) {
	start := time.Now()

	// Try external plugin first
	pluginPath := filepath.Join(pluginDir, name)
	if _, err := os.Stat(pluginPath); err == nil {
		// External plugin exists
		l.log.Debug("Loading external plugin", "name", name, "path", pluginPath)
		return l.startPlugin(ctx, name, exec.Command(pluginPath), config, start)
	}

	// Fall back to embedded plugin
	forgePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get forge executable: %w", err)
	}

	l.log.Debug("Loading embedded plugin", "name", name)
	return l.startPlugin(ctx, name, exec.Command(forgePath, "plugin", name), config, start)
}

func (l *Loader) startPlugin(ctx context.Context, name string, cmd *exec.Cmd, config map[string]any, start time.Time) (plugins.Driver, error) {
	client := plugins.NewClientFromCmd(cmd, l.log.Named(name))

	driver, err := client.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start plugin '%s': %w", name, err)
	}
	l.clients[name] = client

	// Apply configuration if provided
	if len(config) > 0 {
		if err := driver.ConfigDriver(ctx, plugins.PluginConfig{ConfigMap: config}); err != nil {
			client.Stop()
			return nil, fmt.Errorf("failed to configure driver: %w", err)
		}
		l.log.Debug("Configured plugin", "name", name, "settings", len(config))
	}

	// Open driver
	if err := driver.OpenDriver(ctx); err != nil {
		client.Stop()
		return nil, fmt.Errorf("failed to open driver: %w", err)
	}

	// Probe the plugin (optional check)
	if ok, err := driver.ProbePlugin(ctx); err != nil {
		l.log.Warn("Plugin probe error", "name", name, "error", err)
	} else if !ok {
		l.log.Warn("Plugin probe returned false", "name", name)
	}

	l.log.Debug("Plugin loaded", "name", name, "startup", time.Since(start))
	return driver, nil
}

// GetDriver returns a loaded driver by name.
func (l *Loader) GetDriver(name string) plugins.Driver {
	return l.drivers[name]
}

// GetDrivers returns all loaded drivers.
func (l *Loader) GetDrivers() map[string]plugins.Driver {
	return l.drivers
}

// Close stops all loaded plugins.
func (l *Loader) Close() {
	for name, client := range l.clients {
		l.log.Debug("Stopping plugin", "name", name)
		client.Stop()
	}
}

// ParseHclBody converts hcl.Body to map[string]any.
func ParseHclBody(body hcl.Body) (map[string]any, error) {
	result := make(map[string]any)

	if body == nil {
		return result, nil
	}

	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse attributes: %v", diags)
	}

	for name, attr := range attrs {
		value, diags := attr.Expr.Value(&hcl.EvalContext{})
		if diags.HasErrors() {
			continue
		}

		switch {
		case value.Type() == cty.String:
			result[name] = value.AsString()
		case value.Type() == cty.Number:
			f, _ := value.AsBigFloat().Float64()
			result[name] = f
		case value.Type() == cty.Bool:
			result[name] = value.True()
		default:
			result[name] = value.GoString()
		}
	}

	return result, nil
}
