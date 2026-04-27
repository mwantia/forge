package provider

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service"
	"github.com/mwantia/forge/internal/service/metrics"
	"github.com/mwantia/forge/internal/service/plugins"
	"github.com/mwantia/forge/internal/service/server"
)

type ProviderService struct {
	service.UnimplementedService

	mu        sync.RWMutex
	providers map[string]sdkplugins.ProviderPlugin

	plugins plugins.PluginsRegistry `fabric:"inject"`
	metrics metrics.MetricsRegistar `fabric:"inject"`
	router  server.HttpRouter       `fabric:"inject"`
	configs ProviderConfig          `fabric:"config:provider"`
	logger  hclog.Logger            `fabric:"logger:provider"`
}

func init() {
	if err := container.Register[*ProviderService](
		container.AsSingleton(),
		container.With[ProviderRegistar](),
	); err != nil {
		panic(err)
	}
}

func (s *ProviderService) Init(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.providers = make(map[string]sdkplugins.ProviderPlugin)

	if err := s.metrics.Register(ProvidersTotal); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	group := s.router.AuthGroup("/provider")
	{
		group.GET("/", s.handleListProviders())
		group.GET("/models", s.handleListAllModels())
		group.GET("/:name", s.handleGetProvider())
		group.GET("/:name/models", s.handleListModels())
		group.GET("/:name/models/:model", s.handleGetModel())
	}
	return nil
}

func (s *ProviderService) Serve(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, driver := range s.plugins.ListDrivers() {
		if driver.Capabilities == nil || driver.Capabilities.Provider == nil {
			continue
		}

		p, err := driver.Driver.GetProviderPlugin(ctx)
		if err != nil {
			s.logger.Warn("Failed to get provider plugin", "driver", driver.Info.Name, "error", err)
			continue
		}

		s.providers[driver.Info.Name] = p
		ProvidersTotal.WithLabelValues().Inc()
		s.logger.Debug("Loaded provider", "name", driver.Info.Name)
	}

	return nil
}

var _ service.Service = (*ProviderService)(nil)
