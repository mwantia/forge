package memory

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

type MemoryService struct {
	service.UnimplementedService

	mu      sync.RWMutex
	store   memoryStore
	backend string // "file" | <plugin name>

	pluginsReg plugins.PluginsRegistry  `fabric:"inject"`
	provider   provider.ProviderRegistar `fabric:"inject"`
	metrics    metrics.MetricsRegistar  `fabric:"inject"`
	router     server.HttpRouter        `fabric:"inject"`
	storage    storage.StorageBackend   `fabric:"inject"`
	tools      tools.ToolsRegistar      `fabric:"inject"`
	config     MemoryConfig             `fabric:"config:memory"`
	logger     hclog.Logger             `fabric:"logger:memory"`

	embedProvider string
	embedModel    string
}

func init() {
	if err := container.Register[*MemoryService](
		container.AsSingleton(),
		container.With[MemoryRegistar](),
	); err != nil {
		panic(err)
	}
}

func (s *MemoryService) Init(ctx context.Context) error {
	if err := s.metrics.Register(MemoryOperationsTotal, MemoryOperationDuration); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	// Default to the built-in file-backed store. Serve may swap this out
	// for a plugin-backed store if the config selects one.
	s.store = &fileMemoryStore{storage: s.storage}
	s.backend = backendFile

	if s.config.EmbedModel != "" {
		p, m, err := s.provider.ResolveEmbedModel(ctx, s.config.EmbedModel)
		if err != nil {
			return fmt.Errorf("memory embed_model: %w", err)
		}
		s.embedProvider = p
		s.embedModel = m
		s.logger.Debug("Resolved memory embed model", "alias", s.config.EmbedModel, "provider", p, "model", m)
	}

	const namespace = "memory"
	if err := s.tools.RegisterNamespaceMetadata(namespace, tools.NamespaceMetadata{
		Description: "Built-in long-term memory: store and semantically retrieve context across sessions.",
		Builtin:     true,
		System: `Built-in memory persists context across turns and sessions, indexed for semantic retrieval. Store facts the user wants remembered (preferences, project context, recurring constraints) — not transient turn details. Retrieve before answering when the question references prior work that may not be in the current message history. Namespace defaults to the caller session ID; pass it explicitly only when sharing memory across sessions is intended.`,
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

	// /v1/memory
	group := s.router.AuthGroup("/memory")
	{
		group.GET("", s.handleStatus())
		group.POST("/:namespace/resources", s.handleStoreResource())
		group.GET("/:namespace/resources", s.handleRetrieveResources())
	}

	return nil
}

func (s *MemoryService) Serve(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, driver := range s.pluginsReg.ListDrivers() {
		if driver.Capabilities == nil || driver.Capabilities.Memory == nil {
			continue
		}
		if s.config.Plugin != "" && driver.Info.Name != s.config.Plugin {
			continue
		}

		p, err := driver.Driver.GetMemoryPlugin(ctx)
		if err != nil {
			s.logger.Warn("Failed to get memory plugin", "driver", driver.Info.Name, "error", err)
			continue
		}
		if p == nil {
			continue
		}

		s.store = &pluginMemoryStore{plugin: p}
		s.backend = driver.Info.Name
		s.logger.Info("Bound memory plugin", "name", s.backend)
		return nil
	}

	if s.config.Plugin != "" {
		return fmt.Errorf("memory plugin %q not found or lacks memory capability", s.config.Plugin)
	}
	s.logger.Debug("No memory plugin bound; using built-in file store")
	return nil
}

func (s *MemoryService) Cleanup(context.Context) error {
	return nil
}

var _ service.Service = (*MemoryService)(nil)
