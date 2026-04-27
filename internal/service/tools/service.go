package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service"
	"github.com/mwantia/forge/internal/service/metrics"
	"github.com/mwantia/forge/internal/service/plugins"
	"github.com/mwantia/forge/internal/service/server"
)

type ToolsService struct {
	service.UnimplementedService

	mu         sync.RWMutex
	namespaces map[string][]*ToolsNamespace
	nsMeta     map[string]NamespaceMetadata

	plugins plugins.PluginsRegistry `fabric:"inject"`
	metrics metrics.MetricsRegistar `fabric:"inject"`
	router  server.HttpRouter       `fabric:"inject"`
	logger  hclog.Logger            `fabric:"logger:tools"`
}

type ToolsNamespace struct {
	FullName       string
	ToolDefinition sdkplugins.ToolDefinition
	Execution      ToolsExecution
}

func init() {
	if err := container.Register[*ToolsService](
		container.AsSingleton(),
		container.With[ToolsRegistar](),
	); err != nil {
		panic(err)
	}
}

func (s *ToolsService) Init(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.namespaces = make(map[string][]*ToolsNamespace)
	s.nsMeta = make(map[string]NamespaceMetadata)

	if err := s.metrics.Register(ToolsTotal, ToolsExecutionsTotal, ToolsExecutionDuration); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	group := s.router.AuthGroup("/tools")
	{
		group.GET("/", s.handleListTools())
		group.GET("/:namespace", s.handleListToolsByNamespace())
		group.GET("/:namespace/:name", s.handleGetTool())
		group.POST("/:namespace/:name/execute", s.handleExecuteTool())
		group.POST("/:namespace/:name/execute/:callid", s.handleExecuteToolWithCallID())
	}

	return nil
}

func (s *ToolsService) Serve(ctx context.Context) error {
	for _, driver := range s.plugins.ListDrivers() {
		if driver.Capabilities == nil || driver.Capabilities.Tools == nil {
			continue
		}

		tp, err := driver.Driver.GetToolsPlugin(ctx)
		if err != nil {
			s.logger.Warn("Failed to get tools plugin", "driver", driver.Info.Name, "error", err)
			continue
		}

		resp, err := tp.ListTools(ctx, sdkplugins.ListToolsFilter{})
		if err != nil {
			s.logger.Warn("Failed to list tools", "driver", driver.Info.Name, "error", err)
			continue
		}

		namespace := driver.Info.Name

		info := driver.Driver.GetPluginInfo()
		if err := s.RegisterNamespaceMetadata(namespace, NamespaceMetadata{
			Description: info.Description,
			Version:     info.Version,
			Plugin:      tp,
		}); err != nil {
			s.logger.Warn("Failed to register namespace metadata", "namespace", namespace, "error", err)
		}

		for _, def := range resp.Tools {
			captured := tp
			capturedDef := def
			exec := func(ctx context.Context, req sdkplugins.ExecuteRequest) (*sdkplugins.ExecuteResponse, error) {
				req.Tool = capturedDef.Name
				return captured.Execute(ctx, req)
			}
			if err := s.RegisterTool(namespace, def, exec); err != nil {
				s.logger.Warn("Failed to register tool", "namespace", namespace, "tool", def.Name, "error", err)
			}
		}

		s.logger.Debug("Loaded tools plugin", "name", namespace, "count", len(resp.Tools))
	}

	return nil
}

var _ service.Service = (*ToolsService)(nil)
