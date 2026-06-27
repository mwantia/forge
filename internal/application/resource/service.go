package resource

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	"github.com/mwantia/forge-sdk/pkg/plugin/tool"
	approot "github.com/mwantia/forge/internal/application"
	domprovider "github.com/mwantia/forge/internal/domain/provider"
	domtool "github.com/mwantia/forge/internal/domain/tool"
	inframetrics "github.com/mwantia/forge/internal/infrastructure/metrics"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
	infrastorage "github.com/mwantia/forge/internal/infrastructure/storage"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
)

type ResourceService struct {
	approot.UnimplementedService

	defaultStore resourceStore // built-in DAG store, sole storage backend
	idx          *vectorIndex  // shared flat vector index for semantic recall

	provider domprovider.ProviderRegistar        `fabric:"inject"`
	metrics  inframetrics.MetricsRegistar        `fabric:"inject"`
	router   infraserver.HttpRouter              `fabric:"inject"`
	storage  infrastorage.StorageBackend         `fabric:"inject"`
	tools    domtool.ToolsRegistar               `fabric:"inject"`
	tmpl     infratemplate.TemplateRenderer      `fabric:"inject"`
	config   ResourceConfig                      `fabric:"config=resource"`
	logger   hclog.Logger                        `fabric:"logger=resource"`

	embedProvider string
	embedModel    string
	embedTemplate string

	uploadMaxBytes uint64
	uploadExts     []string
	uploadOptimize bool
}

func init() {
	container.MustRegister[*ResourceService](
		container.AsSingleton(),
		container.With[ResourceRegistar](),
	)
}

func (*ResourceService) PreInit(context.Context) error {
	return nil
}

func (s *ResourceService) PostInit(ctx context.Context) error {
	if err := s.metrics.Register(ResourceOperationsTotal, ResourceOperationDuration); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	// defaultStore is always the built-in file store. Serve() may add plugin
	// mounts on top, but requests arriving before Serve() completes are safe.
	s.defaultStore = newDagResourceStore(s.storage)
	s.idx = newVectorIndex()

	maxBytes, err := s.config.Upload.GetFilesize()
	if err != nil {
		return fmt.Errorf("resource upload.filesize: %w", err)
	}
	s.uploadMaxBytes = maxBytes
	s.uploadExts = s.config.Upload.GetExtensions()
	s.uploadOptimize = s.config.Upload.GetOptimize()

	if alias := s.config.Embed.GetModel(); alias != "" {
		p, m, err := s.provider.ResolveEmbedModel(ctx, alias)
		if err != nil {
			return fmt.Errorf("resource embed.model: %w", err)
		}
		s.embedProvider = p
		s.embedModel = m
		s.embedTemplate = s.config.Embed.GetTemplate()
		s.logger.Debug("Resolved resource embed model", "alias", alias, "provider", p, "model", m)
	}

	for _, definition := range ToolsDefinitions {
		capturedDef := definition
		exec := func(ctx context.Context, req tool.ExecuteToolRequest) (*tool.ExecuteToolResponse, error) {
			req.Tool = capturedDef.Name
			return s.ExecuteTool(ctx, req)
		}
		if err := s.tools.RegisterTool("builtin", definition, exec); err != nil {
			return fmt.Errorf("failed to register tool %q under builtin namespace: %w", definition.Name, err)
		}
	}

	// /v1/resources — flat bucket, no path prefix
	group := s.router.AuthGroup("/resources")
	{
		group.GET("", s.handleListResources())
		group.PUT("", s.handleStoreResource())
		group.POST("/recall", s.handleRecallResources())
		group.GET("/:id", s.handleGetResource())
		group.POST("/:id/commit", s.handleCommitResource())
		group.GET("/:id/history", s.handleHistory())
		group.GET("/:id/diff", s.handleDiff())
		group.GET("/:id/versions/:hash", s.handleGetVersion())
		group.POST("/:id/revert", s.handleRevert())
		group.PATCH("/:id", s.handlePatchResource())
		group.DELETE("/:id", s.handleForgetResource())
	}

	return nil
}

func (s *ResourceService) Serve(_ context.Context) error {
	return nil
}

func (s *ResourceService) Cleanup(context.Context) error {
	return nil
}

// applyEmbedTemplate renders s.embedTemplate with the given content bound to
// {{ .embed }}. All base template vars (runtime, env, …) remain accessible.
func (s *ResourceService) applyEmbedTemplate(content string) (string, error) {
	t, err := s.tmpl.Clone(infratemplate.WithAnyValue("embed", content))
	if err != nil {
		return "", fmt.Errorf("clone embed template: %w", err)
	}

	return t.RenderBody(s.embedTemplate)
}

var _ approot.Service = (*ResourceService)(nil)
