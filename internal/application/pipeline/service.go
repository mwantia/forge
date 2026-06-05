package pipeline

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	approot "github.com/mwantia/forge/internal/application"
	appsession "github.com/mwantia/forge/internal/application/session"
	dompipeline "github.com/mwantia/forge/internal/domain/pipeline"
	domapprovals "github.com/mwantia/forge/internal/domain/approvals"
	domprovider "github.com/mwantia/forge/internal/domain/provider"
	domresource "github.com/mwantia/forge/internal/domain/resource"
	domtool "github.com/mwantia/forge/internal/domain/tool"
	inframetrics "github.com/mwantia/forge/internal/infrastructure/metrics"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
)

const (
	DefaultMaxToolIterations = 25
	ServiceNamespace         = "pipeline"
)

type PipelineService struct {
	approot.UnimplementedService

	tmpl    infratemplate.TemplateRenderer `fabric:"inject"`
	metrics inframetrics.MetricsRegistar   `fabric:"inject"`
	router  infraserver.HttpRouter         `fabric:"inject"`
	config  PipelineConfig                 `fabric:"config=pipeline"`
	logger  hclog.Logger                   `fabric:"logger=pipeline"`

	sessions  appsession.SessionManager       `fabric:"inject"`
	tools     domtool.ToolsRegistar           `fabric:"inject"`
	provider  domprovider.ProviderRegistar    `fabric:"inject"`
	resources domresource.ResourceRegistar    `fabric:"inject"`
	approvals domapprovals.ApprovalRegistar   `fabric:"inject"`
}

func init() {
	container.MustRegister[*PipelineService](
		container.AsSingleton(),
		container.With[PipelineExecutor](),
		container.With[dompipeline.BackgroundDispatcher](),
	)
}

func (*PipelineService) PreInit(context.Context) error {
	return nil
}

func (s *PipelineService) PostInit(ctx context.Context) error {
	if err := s.metrics.Register(PipelineMessagesTotal, PipelineToolCallsTotal); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	if s.config.MaxToolIterations <= 0 {
		s.config.MaxToolIterations = DefaultMaxToolIterations
	}

	metadata := domtool.NamespaceMetadata{
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

	// /v1/sessions/:session_id/system — owned by pipeline (needs prompt assembly).
	sessions := s.router.AuthGroup("/sessions")
	{
		sessions.POST("/:session_id/system/reset", s.handleResetSystemSnapshot())
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

var _ approot.Service = (*PipelineService)(nil)
