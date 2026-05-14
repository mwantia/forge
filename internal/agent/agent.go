package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge-sdk/pkg/errors"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/service/event"
	"github.com/mwantia/forge/internal/service/metrics"
	"github.com/mwantia/forge/internal/service/pipeline"
	"github.com/mwantia/forge/internal/service/plugins"
	"github.com/mwantia/forge/internal/service/provider"
	"github.com/mwantia/forge/internal/service/resource"
	"github.com/mwantia/forge/internal/service/server"
	"github.com/mwantia/forge/internal/service/session"
	"github.com/mwantia/forge/internal/service/system"
	"github.com/mwantia/forge/internal/service/tools"
)

type Agent struct {
	mutex sync.RWMutex
	wait  sync.WaitGroup

	logger hclog.Logger        `fabric:"logger:agent"`
	config *config.AgentConfig `fabric:"config"`

	plugins   plugins.PluginsServer     `fabric:"inject"`
	srv       *server.ServerService     `fabric:"inject"`
	met       *metrics.MetricsService   `fabric:"inject"`
	pipe      *pipeline.PipelineService `fabric:"inject"`
	sess      *session.SessionService   `fabric:"inject"`
	res       *resource.ResourceService `fabric:"inject"`
	providers *provider.ProviderService `fabric:"inject"`
	toolsSvc  *tools.ToolsService       `fabric:"inject"`
	events    *event.EventService       `fabric:"inject"`
	sysSvc    *system.SystemService     `fabric:"inject"`
}

func init() {
	if err := container.Register[*Agent](container.AsSingleton()); err != nil {
		panic(err)
	}
}

func (a *Agent) Serve(once bool, ctx context.Context) error {
	a.logger.Debug("Loading configured plugins...")
	if err := a.plugins.ServePluginsFrom(ctx, a.config.PluginDir); err != nil {
		a.logger.Error("Plugins failed to load", "errors", err.Error())
	}

	a.logger.Debug("Loading provider plugins...")
	if err := a.providers.Serve(ctx); err != nil {
		return fmt.Errorf("failed to load providers: %w", err)
	}

	a.logger.Debug("Loading tools plugins...")
	if err := a.toolsSvc.Serve(ctx); err != nil {
		return fmt.Errorf("failed to load tools: %w", err)
	}

	a.logger.Debug("Binding session backend...")
	if err := a.sess.Serve(ctx); err != nil {
		return fmt.Errorf("failed to bind session backend: %w", err)
	}

	a.logger.Debug("Binding resource backend...")
	if err := a.res.Serve(ctx); err != nil {
		return fmt.Errorf("failed to bind resource backend: %w", err)
	}

	a.logger.Debug("Binding event service...")
	if err := a.events.Serve(ctx); err != nil {
		return fmt.Errorf("failed to bind event service: %w", err)
	}

	a.mutex.Lock()
	a.logger.Debug("Starting server runner...")
	a.wait.Go(func() {
		if err := a.srv.Serve(ctx); err != nil {
			a.logger.Error("error serving http server", "error", err)
		}
	})

	a.logger.Debug("Starting metrics runner...")
	a.wait.Go(func() {
		if err := a.met.Serve(ctx); err != nil {
			a.logger.Error("error serving metrics server", "error", err)
		}
	})

	a.logger.Debug("Starting pipeline runner...")
	a.wait.Go(func() {
		if err := a.pipe.Serve(ctx); err != nil {
			a.logger.Error("error serving pipeline server", "error", err)
		}
	})

	a.mutex.Unlock()

	if !once {
		<-ctx.Done()
	}

	a.logger.Debug("Shutting down agent...")

	globalTimeout, err := time.ParseDuration(a.config.ShutdownTimeout)
	if err != nil {
		a.logger.Warn("Invalid duration defined for 'shutdown_timeout': %v", err)
		globalTimeout = 30 * time.Second
	}

	// Use a global background context with timeout for fan-out cleanup
	globalCtx, globalCancel := context.WithTimeout(context.Background(), globalTimeout)
	defer globalCancel()

	errs := &errors.Errors{}

	var mu sync.Mutex
	var wg sync.WaitGroup
	// Dedicated service list for all cleanup calls
	services := []interface{ Cleanup(context.Context) error }{
		a.pipe,
		a.events,
		a.sysSvc,
		a.sess,
		a.res,
		a.toolsSvc,
		a.providers,
		a.srv,
		a.met,
		a.plugins, // last: kills subprocesses after all gRPC callers are done
	}

	for _, svc := range services {
		wg.Add(1)
		// Performing fan-out cleanup with global timeout and per-service timeout of 5sec.
		go func(interface{ Cleanup(context.Context) error }) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(globalCtx, 5*time.Second)
			defer cancel()

			if err := svc.Cleanup(ctx); err != nil {
				a.logger.Error("Failed to perform service cleanup for type '%T': %v", svc, err)

				mu.Lock()
				errs.Add(err)
				mu.Unlock()
			}
		}(svc)
	}

	wg.Wait()
	a.wait.Wait()

	return errs.Errors()
}
