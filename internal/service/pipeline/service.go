package pipeline

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service"
	"github.com/mwantia/forge/internal/service/metrics"
	"github.com/mwantia/forge/internal/service/provider"
	"github.com/mwantia/forge/internal/service/resource"
	"github.com/mwantia/forge/internal/service/server"
	"github.com/mwantia/forge/internal/service/session"
	"github.com/mwantia/forge/internal/service/template"
	"github.com/mwantia/forge/internal/service/tools"
)

const (
	DefaultMaxToolIterations = 25
)

type PipelineService struct {
	service.UnimplementedService

	tmpl    template.TemplateRenderer `fabric:"inject"`
	metrics metrics.MetricsRegistar   `fabric:"inject"`
	router  server.HttpRouter         `fabric:"inject"`
	config  PipelineConfig            `fabric:"config:pipeline"`
	logger  hclog.Logger              `fabric:"logger:pipeline"`

	sessions  session.SessionManager     `fabric:"inject"`
	tools     tools.ToolsRegistar        `fabric:"inject"`
	provider  provider.ProviderRegistar  `fabric:"inject"`
	resources resource.ResourceRegistar  `fabric:"inject"`
}

func init() {
	if err := container.Register[*PipelineService](
		container.AsSingleton(),
		container.With[PipelineExecutor](),
		container.With[BackgroundDispatcher](),
	); err != nil {
		panic(err)
	}
}

func (s *PipelineService) Init(ctx context.Context) error {
	if err := s.metrics.Register(PipelineMessagesTotal, PipelineToolCallsTotal); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	if s.config.MaxToolIterations <= 0 {
		s.config.MaxToolIterations = DefaultMaxToolIterations
	}

	// /v1/sessions
	group := s.router.AuthGroup("/sessions")
	{
		group.POST("/dispatch", s.handleDispatch())
		group.POST("/preview", s.handlePreview())
	}

	// /v1/contexts — debug/observability surface for the dispatched
	// PromptContext blobs that the pipeline records each turn.
	contexts := s.router.AuthGroup("/contexts")
	{
		contexts.GET("/:hash", s.handleGetContext())
		contexts.GET("/:hash/materialized", s.handleMaterializeContext())
		contexts.POST("/:hash/replay", s.handleReplayContext())
	}

	// /v1/sessions/:session_id/system — system prompt snapshot management.
	sessions := s.router.AuthGroup("/sessions")
	{
		sessions.GET("/:session_id/system", s.handleSystemShow())
		sessions.PATCH("/:session_id/system", s.handleSystemEdit())
		sessions.POST("/:session_id/system/regen", s.handleSystemRegen())
	}

	return nil
}

func (s *PipelineService) Cleanup(context.Context) error {
	return nil
}

var _ service.Service = (*PipelineService)(nil)
