package resource

import (
	"context"
	"fmt"
	"sort"
	"strings"
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

// mountedStore pairs a path prefix with the store that owns it.
type mountedStore struct {
	prefix string       // normalized: leading slash, no trailing slash
	store  resourceStore
	plugin string       // "file" or driver name, for status display
}

type ResourceService struct {
	service.UnimplementedService

	mu           sync.RWMutex
	mounts       []mountedStore // sorted longest-prefix-first
	defaultStore resourceStore  // built-in file store, always present

	pluginsReg plugins.PluginsRegistry   `fabric:"inject"`
	provider   provider.ProviderRegistar `fabric:"inject"`
	metrics    metrics.MetricsRegistar   `fabric:"inject"`
	router     server.HttpRouter         `fabric:"inject"`
	storage    storage.StorageBackend    `fabric:"inject"`
	tools      tools.ToolsRegistar       `fabric:"inject"`
	config     ResourceConfig            `fabric:"config:resource"`
	logger     hclog.Logger              `fabric:"logger:resource"`

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

	// defaultStore is always the built-in file store. Serve() may add plugin
	// mounts on top, but requests arriving before Serve() completes are safe.
	s.defaultStore = &fileResourceStore{storage: s.storage}

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

	// Collect all resource-capable plugin drivers.
	pluginMap := map[string]sdkplugins.ResourcePlugin{}
	for _, driver := range s.pluginsReg.ListDrivers() {
		if driver.Capabilities == nil || driver.Capabilities.Resource == nil {
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
		pluginMap[driver.Info.Name] = p
	}

	// Always keep defaultStore pointing at the file store so that Init-time
	// assignment stays coherent after Serve runs.
	s.defaultStore = &fileResourceStore{storage: s.storage}

	if len(s.config.Mounts) == 0 {
		s.logger.Debug("No resource mount blocks; all paths use built-in file store")
		return nil
	}

	seen := map[string]struct{}{}
	mounts := make([]mountedStore, 0, len(s.config.Mounts))

	for _, mc := range s.config.Mounts {
		// Normalize: ensure single leading slash, no trailing slash.
		normalized := "/" + strings.Trim(mc.Path, "/")

		if _, dup := seen[normalized]; dup {
			return fmt.Errorf("resource: duplicate mount prefix %q", normalized)
		}
		seen[normalized] = struct{}{}

		var store resourceStore
		var pluginLabel string

		if mc.Plugin == "" || mc.Plugin == "file" {
			store = &fileResourceStore{storage: s.storage}
			pluginLabel = "file"
		} else {
			p, ok := pluginMap[mc.Plugin]
			if !ok {
				return fmt.Errorf("resource mount %q: plugin %q not found or lacks resource capability", normalized, mc.Plugin)
			}
			store = &pluginResourceStore{plugin: p}
			pluginLabel = mc.Plugin
		}

		mounts = append(mounts, mountedStore{
			prefix: normalized,
			store:  store,
			plugin: pluginLabel,
		})
		s.logger.Info("Mounted resource store", "path", normalized, "plugin", pluginLabel)
	}

	// Longest prefix first so resolveStore picks the most specific match.
	sort.SliceStable(mounts, func(i, j int) bool {
		return len(mounts[i].prefix) > len(mounts[j].prefix)
	})

	s.mounts = mounts
	return nil
}

func (s *ResourceService) Cleanup(context.Context) error {
	return nil
}

var _ service.Service = (*ResourceService)(nil)
