package provider

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	approot "github.com/mwantia/forge/internal/application"
	domplugin "github.com/mwantia/forge/internal/domain/plugin"
	inframetrics "github.com/mwantia/forge/internal/infrastructure/metrics"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
)

type ProviderService struct {
	approot.UnimplementedService

	mu        sync.RWMutex
	providers map[string]sdkplugins.ProviderPlugin

	plugins domplugin.PluginsRegistry    `fabric:"inject"`
	metrics inframetrics.MetricsRegistar `fabric:"inject"`
	router  infraserver.HttpRouter       `fabric:"inject"`
	configs ProviderConfig               `fabric:"config=provider"`
	logger  hclog.Logger                 `fabric:"logger=provider"`
}

func init() {
	container.MustRegister[*ProviderService](
		container.AsSingleton(),
		container.With[ProviderRegistar](),
	)
}

func (*ProviderService) PreInit(context.Context) error {
	return nil
}

func (s *ProviderService) PostInit(ctx context.Context) error {
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
		group.POST("/embed", s.handleEmbed())
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

var _ approot.Service = (*ProviderService)(nil)
