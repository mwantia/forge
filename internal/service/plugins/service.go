package plugins

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service"
	"github.com/mwantia/forge/internal/service/metrics"
	"github.com/mwantia/forge/internal/service/server"
	"github.com/mwantia/forge/internal/service/template"
)

type PluginsService struct {
	service.UnimplementedService

	mu      sync.RWMutex
	drivers map[string]*PluginDriver

	tmpl    template.TemplateRenderer `fabric:"inject"`
	metrics metrics.MetricsRegistar   `fabric:"inject"`
	router  server.HttpRouter         `fabric:"inject"`
	configs []PluginConfig            `fabric:"config:plugin"`
	logger  hclog.Logger              `fabric:"logger:plugins"`
}

func init() {
	if err := container.Register[*PluginsService](
		container.AsSingleton(),
		container.With[PluginsRegistry](),
		container.With[PluginsServer](),
	); err != nil {
		panic(err)
	}
}

func (s *PluginsService) Init(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.metrics.Register(PluginsLoaded, PluginsServeTotal, PluginsServeDuration); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	// /v1/plugins
	group := s.router.AuthGroup("/plugins")
	{
		group.GET("/", s.handleListPlugins())
		group.GET("/:name", s.handleGetPlugin())
		group.GET("/:name/capabilities", s.handleGetPluginCapabilities())
	}

	return nil
}

var _ service.Service = (*PluginsService)(nil)
