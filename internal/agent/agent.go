package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge-sdk/pkg/errors"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/service/metrics"
	"github.com/mwantia/forge/internal/service/pipeline"
	"github.com/mwantia/forge/internal/service/plugins"
	"github.com/mwantia/forge/internal/service/provider"
	"github.com/mwantia/forge/internal/service/resource"
	"github.com/mwantia/forge/internal/service/server"
	"github.com/mwantia/forge/internal/service/session"
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

	if err := a.Cleanup(ctx); err != nil {
		return fmt.Errorf("failed to complete agent cleanup: %w", err)
	}

	a.wait.Wait()
	return nil
}

func (a *Agent) Cleanup(ctx context.Context) error {
	errs := &errors.Errors{}
	if err := a.plugins.Cleanup(ctx); err != nil {
		errs.Add(fmt.Errorf("plugins cleanup: %w", err))
	}
	if err := a.srv.Cleanup(ctx); err != nil {
		errs.Add(fmt.Errorf("server cleanup: %w", err))
	}
	if err := a.met.Cleanup(ctx); err != nil {
		errs.Add(fmt.Errorf("metrics cleanup: %w", err))
	}
	return errs.Errors()
}
