package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/metrics"
	"github.com/mwantia/forge/internal/plugins"
	"github.com/mwantia/forge/internal/server"
	"github.com/mwantia/forge/pkg/errors"
)

type Agent struct {
	mutex    sync.RWMutex
	wait     sync.WaitGroup
	cleanups []func() error

	log      hclog.Logger
	cfg      config.AgentConfig
	registry *plugins.PluginRegistry
}

func NewAgent(cfg config.AgentConfig) *Agent {
	log := hclog.Default().Named("agent")
	return &Agent{
		cleanups: make([]func() error, 0),
		log:      log,
		cfg:      cfg,
		registry: plugins.NewRegistry(log),
	}
}

func (a *Agent) Serve(once bool, ctx context.Context) error {
	// Load configured plugins
	a.log.Debug("Loading configured plugins...")
	if err := a.registry.ServePlugins(ctx, a.cfg.PluginDir, a.cfg.Plugins); err != nil {
		a.log.Error("Plugins failed to load", "errors", err.Error())
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
	a.registry.CleanupDrivers()

	errs := &errors.Errors{}
	for _, cleanup := range a.cleanups {
		if err := cleanup(); err != nil {
			errs.Add(fmt.Errorf("error during cleanup: %w", err))
		}
	}

	return errs.Errors()
}
