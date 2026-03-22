package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/metrics"
	pluginloader "github.com/mwantia/forge/internal/plugin"
	"github.com/mwantia/forge/internal/server"
)

type Agent struct {
	mutex    sync.RWMutex
	wait     sync.WaitGroup
	cleanups []func() error

	log    hclog.Logger
	cfg    config.AgentConfig
	loader *pluginloader.Loader
}

func NewAgent(cfg config.AgentConfig) *Agent {
	log := hclog.Default().Named("agent")
	return &Agent{
		cleanups: make([]func() error, 0),
		log:      log,
		cfg:      cfg,
		loader:   pluginloader.NewLoader(log),
	}
}

func (a *Agent) Serve(once bool, ctx context.Context) error {
	// Load configured plugins
	a.log.Debug("Loading configured plugins...")
	pluginConfigs := make(map[string]map[string]any)
	for _, pc := range a.cfg.Plugins {
		if pc.Config != nil && pc.Config.Body != nil {
			cfgMap, err := pluginloader.ParseHclBody(pc.Config.Body)
			if err != nil {
				a.log.Warn("Failed to parse config", "name", pc.Name, "error", err)
				continue
			}
			pluginConfigs[pc.Name] = cfgMap
		}
	}

	results := a.loader.LoadAll(ctx, a.cfg.PluginDir, pluginConfigs)
	for _, result := range results {
		if result.Error != nil {
			a.log.Error("Plugin failed to load", "name", result.Name, "error", result.Error)
		} else {
			a.log.Debug("Plugin loaded successfully", "name", result.Name)
		}
	}

	a.mutex.Lock()

	if a.cfg.Server != nil {
		a.log.Debug("Server config set - Starting server runner...")

		server, err := server.NewServer(a.cfg, a.log)
		if err != nil {
			return fmt.Errorf("error during server creation: %w", err)
		}

		if err := a.serveRunner(ctx, server); err != nil {
			return err
		}
	}

	if a.cfg.Metrics != nil {
		a.log.Debug("Metrics config set - Starting metrics runner...")

		metrics, err := metrics.NewMetrics(a.cfg, a.log)
		if err != nil {
			return fmt.Errorf("error during metrics creation: %w", err)
		}

		if err := a.serveRunner(ctx, metrics); err != nil {
			return err
		}
	}

	a.mutex.Unlock()
	if !once {
		<-ctx.Done()
	}

	a.log.Debug("Shutting down agent...")

	if err := a.Cleanup(); err != nil {
		return fmt.Errorf("failed to complete agent cleanup: %w", err)
	}

	a.wait.Wait()
	return nil
}

func (a *Agent) Cleanup() error {
	a.loader.Close()

	errs := &Errors{}
	for _, cleanup := range a.cleanups {
		if err := cleanup(); err != nil {
			errs.Add(fmt.Errorf("error during cleanup: %w", err))
		}
	}

	return errs.Errors()
}
