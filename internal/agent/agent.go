package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/mwantia/fabric/pkg/container"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/metrics"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/internal/server"
	"github.com/mwantia/forge/pkg/errors"
)

type Agent struct {
	mutex    sync.RWMutex
	wait     sync.WaitGroup
	cleanups []func() error

	logger    hclog.Logger                `fabric:"logger:agent"`
	config    *config.AgentConfig         `fabric:"config"`
	container *container.ServiceContainer `fabric:"inject"`
	registry  *registry.PluginRegistry    `fabric:"inject"`
}

func (a *Agent) Serve(once bool, ctx context.Context) error {
	// Load configured plugins
	a.logger.Debug("Loading configured plugins...")
	if err := a.registry.ServePlugins(ctx, a.config.PluginDir, a.config.Plugins); err != nil {
		a.logger.Error("Plugins failed to load", "errors", err.Error())
	}

	a.cleanups = make([]func() error, 0)
	a.cleanups = append(a.cleanups, func() error {
		a.registry.CleanupDrivers()
		return nil
	})

	a.mutex.Lock()

	if a.config.Server != nil {
		a.logger.Debug("Server config set - Starting server runner...")
		server, err := container.Resolve[*server.Server](ctx, a.container)
		if err != nil {
			return fmt.Errorf("failed to create http server: %w", err)
		}

		if err := a.serveRunner(ctx, server); err != nil {
			return err
		}
	}

	if a.config.Metrics != nil {
		a.logger.Debug("Metrics config set - Starting metrics runner...")
		metrics, err := container.Resolve[*metrics.Metrics](ctx, a.container)
		if err != nil {
			return fmt.Errorf("failed to create metrics server: %w", err)
		}

		if err := a.serveRunner(ctx, metrics); err != nil {
			return err
		}
	}

	a.mutex.Unlock()
	if !once {
		<-ctx.Done()
	}

	a.logger.Debug("Shutting down agent...")

	if err := a.Cleanup(); err != nil {
		return fmt.Errorf("failed to complete agent cleanup: %w", err)
	}

	a.wait.Wait()
	return nil
}

func (a *Agent) Cleanup() error {
	errs := &errors.Errors{}
	for _, cleanup := range a.cleanups {
		if err := cleanup(); err != nil {
			errs.Add(fmt.Errorf("error during cleanup: %w", err))
		}
	}

	return errs.Errors()
}
