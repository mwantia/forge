package pipeline

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
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
	ServiceNamespace         = "pipeline"
)

type PipelineService struct {
	service.UnimplementedService

	tmpl    template.TemplateRenderer `fabric:"inject"`
	metrics metrics.MetricsRegistar   `fabric:"inject"`
	router  server.HttpRouter         `fabric:"inject"`
	config  PipelineConfig            `fabric:"config:pipeline"`
	logger  hclog.Logger              `fabric:"logger:pipeline"`

	sessions  session.SessionManager    `fabric:"inject"`
	tools     tools.ToolsRegistar       `fabric:"inject"`
	provider  provider.ProviderRegistar `fabric:"inject"`
	resources resource.ResourceRegistar `fabric:"inject"`
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

	metadata := tools.NamespaceMetadata{
		Description: "Pipeline dispatch tools for driving sub-sessions synchronously.",
		Builtin:     true,
		System:      `Use pipeline tools to send a message to a sub-session and collect its full response before continuing. Each call is a complete nested LLM run — use sparingly and frame messages tightly to minimise token cost.`,
	}
	if err := s.tools.RegisterNamespaceMetadata(ServiceNamespace, metadata); err != nil {
		return fmt.Errorf("failed to register namespace metadata for %q: %w", ServiceNamespace, err)
	}

	for _, definition := range ToolsDefinitions {
		captured := definition
		exec := func(ctx context.Context, req sdkplugins.ExecuteRequest) (*sdkplugins.ExecuteResponse, error) {
			req.Tool = captured.Name
			return s.ExecuteTool(ctx, req)
		}
		if err := s.tools.RegisterTool(ServiceNamespace, definition, exec); err != nil {
			return fmt.Errorf("failed to register tool %q for namespace %q: %w", definition.Name, ServiceNamespace, err)
		}
	}

	// /v1/pipeline
	group := s.router.AuthGroup("/" + ServiceNamespace)
	{
		group.POST("/commit", s.handleCommit())
		group.POST("/preview", s.handlePreview())

		group.GET("/contexts/:hash", s.handleGetContext())
		group.GET("/contexts/:hash/materialized", s.handleMaterializeContext())
		group.POST("/contexts/:hash/replay", s.handleReplayContext())
	}

	return nil
}

func (s *PipelineService) Cleanup(context.Context) error {
	return nil
}

var _ service.Service = (*PipelineService)(nil)
