package resource

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service"
	"github.com/mwantia/forge/internal/service/metrics"
	"github.com/mwantia/forge/internal/service/plugins"
	"github.com/mwantia/forge/internal/service/provider"
	"github.com/mwantia/forge/internal/service/server"
	"github.com/mwantia/forge/internal/service/storage"
	"github.com/mwantia/forge/internal/service/tools"
)

const backendFile = "file"

type ResourceService struct {
	service.UnimplementedService

	mu      sync.RWMutex
	store   resourceStore
	backend string // "file" | <plugin name>

	pluginsReg plugins.PluginsRegistry  `fabric:"inject"`
	provider   provider.ProviderRegistar `fabric:"inject"`
	metrics    metrics.MetricsRegistar  `fabric:"inject"`
	router     server.HttpRouter        `fabric:"inject"`
	storage    storage.StorageBackend   `fabric:"inject"`
	tools      tools.ToolsRegistar      `fabric:"inject"`
	config     ResourceConfig           `fabric:"config:resource"`
	logger     hclog.Logger             `fabric:"logger:resource"`

	embedProvider string
	embedModel    string
}

func init() {
	if err := container.Register[*ResourceService](
		container.AsSingleton(),
		container.With[ResourceRegistar](),
	); err != nil {
		panic(err)
	}
}

func (s *ResourceService) Init(ctx context.Context) error {
	if err := s.metrics.Register(ResourceOperationsTotal, ResourceOperationDuration); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	// Default to the built-in file-backed store. Serve may swap this out
	// for a plugin-backed store if the config selects one.
	s.store = &fileResourceStore{storage: s.storage}
	s.backend = backendFile

	if s.config.EmbedModel != "" {
		p, m, err := s.provider.ResolveEmbedModel(ctx, s.config.EmbedModel)
		if err != nil {
			return fmt.Errorf("resource embed_model: %w", err)
		}
		s.embedProvider = p
		s.embedModel = m
		s.logger.Debug("Resolved resource embed model", "alias", s.config.EmbedModel, "provider", p, "model", m)
	}

	const namespace = "resource"
	if err := s.tools.RegisterNamespaceMetadata(namespace, tools.NamespaceMetadata{
		Description: "Built-in long-term resource store: persist and semantically retrieve context across sessions.",
		Builtin:     true,
		System: `Built-in resources persist context across turns and sessions, indexed for semantic retrieval. Store facts the user wants remembered (preferences, project context, recurring constraints) — not transient turn details. Retrieve before answering when the question references prior work that may not be in the current message history. Path defaults to the caller session (/sessions/<id>); use /global for agent-wide facts or any explicit path to share across sessions.`,
	}); err != nil {
		return fmt.Errorf("failed to register namespace metadata for %q: %w", namespace, err)
	}
	for _, definition := range ToolsDefinitions {
		capturedDef := definition
		exec := func(ctx context.Context, req sdkplugins.ExecuteRequest) (*sdkplugins.ExecuteResponse, error) {
			req.Tool = capturedDef.Name
			return s.ExecuteTool(ctx, req)
		}
		if err := s.tools.RegisterTool(namespace, definition, exec); err != nil {
			return fmt.Errorf("failed to register tool %q for namespace %q: %w", definition.Name, namespace, err)
		}
	}

	// /v1/resources
	// Method convention (avoids Gin wildcard conflicts):
	//   GET    /*path          → list (or get with ?id=)
	//   PUT    /*path          → store
	//   POST   /*path          → recall (RecallQuery JSON body)
	//   DELETE /*path          → forget (?id= required)
	group := s.router.AuthGroup("/resources")
	{
		group.GET("", s.handleStatus())
		group.GET("/*path", s.handleListOrGet())
		group.PUT("/*path", s.handleStoreResource())
		group.POST("/*path", s.handleRecallResources())
		group.DELETE("/*path", s.handleForgetResource())
	}

	return nil
}

func (s *ResourceService) Serve(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, driver := range s.pluginsReg.ListDrivers() {
		if driver.Capabilities == nil || driver.Capabilities.Resource == nil {
			continue
		}
		if s.config.Plugin != "" && driver.Info.Name != s.config.Plugin {
			continue
		}

		p, err := driver.Driver.GetResourcePlugin(ctx)
		if err != nil {
			s.logger.Warn("Failed to get resource plugin", "driver", driver.Info.Name, "error", err)
			continue
		}
		if p == nil {
			continue
		}

		s.store = &pluginResourceStore{plugin: p}
		s.backend = driver.Info.Name
		s.logger.Info("Bound resource plugin", "name", s.backend)
		return nil
	}

	if s.config.Plugin != "" {
		return fmt.Errorf("resource plugin %q not found or lacks resource capability", s.config.Plugin)
	}
	s.logger.Debug("No resource plugin bound; using built-in file store")
	return nil
}

func (s *ResourceService) Cleanup(context.Context) error {
	return nil
}

var _ service.Service = (*ResourceService)(nil)
