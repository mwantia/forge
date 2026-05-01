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

	// /v1/pipeline
	group := s.router.AuthGroup("/pipeline")
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

	return nil
}

func (s *PipelineService) Cleanup(context.Context) error {
	return nil
}

var _ service.Service = (*PipelineService)(nil)
