package plugins

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	approot "github.com/mwantia/forge/internal/application"
	inframetrics "github.com/mwantia/forge/internal/infrastructure/metrics"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
)

type PluginsService struct {
	approot.UnimplementedService

	mu      sync.RWMutex
	drivers map[string]*PluginDriver

	tmpl    infratemplate.TemplateRenderer `fabric:"inject"`
	metrics inframetrics.MetricsRegistar   `fabric:"inject"`
	router  infraserver.HttpRouter         `fabric:"inject"`
	configs []PluginConfig                 `fabric:"config=plugin"`
	logger  hclog.Logger                   `fabric:"logger=plugins"`
}

func init() {
	container.MustRegister[*PluginsService](
		container.AsSingleton(),
		container.With[PluginsRegistry](),
		container.With[PluginsServer](),
	)
}

func (*PluginsService) PreInit(context.Context) error {
	return nil
}

func (s *PluginsService) PostInit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.drivers = make(map[string]*PluginDriver)

	if err := s.metrics.Register(PluginsLoaded, PluginsServeTotal, PluginsServeDuration); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	// /v1/plugins
	group := s.router.AuthGroup("/plugins")
	{
		group.GET("/", s.handleListPlugins())
		group.GET("/:name", s.handleGetPlugin())
		group.GET("/:name/capabilities", s.handleGetPluginCapabilities())
		group.GET("/:name/health", s.handleGetPluginHealth())
	}

	return nil
}

var _ approot.Service = (*PluginsService)(nil)
