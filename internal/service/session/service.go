package session

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
	"github.com/mwantia/forge/internal/service/server"
	"github.com/mwantia/forge/internal/service/storage"
	"github.com/mwantia/forge/internal/service/tools"
)

type SessionService struct {
	service.UnimplementedService

	mu      sync.RWMutex
	store   sessionBackend
	backend string

	metrics    metrics.MetricsRegistar `fabric:"inject"`
	pluginsReg plugins.PluginsRegistry `fabric:"inject"`
	router     server.HttpRouter       `fabric:"inject"`
	storage    storage.StorageBackend  `fabric:"inject"`
	tools      tools.ToolsRegistar     `fabric:"inject"`
	config     SessionConfig           `fabric:"config:session"`
	logger     hclog.Logger            `fabric:"logger:session"`
}

func init() {
	if err := container.Register[*SessionService](
		container.AsSingleton(),
		container.With[SessionManager](),
	); err != nil {
		panic(err)
	}
}

func (s *SessionService) Init(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.metrics.Register(SessionsTotal); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	// Default to the file-backed store. Serve may swap to plugin-backed.
	s.store = &fileSessionStore{storage: s.storage}
	s.backend = BackendFile

	const namespace = "sessions"
	if err := s.tools.RegisterNamespaceMetadata(namespace, tools.NamespaceMetadata{
		Description: "Built-in session bookkeeping: title/description, sub-session creation, message history.",
		Builtin:     true,
		System: `Built-in session tools manage the conversation's own metadata and any sub-sessions you spawn. Update title/description when the topic crystallises so the user can navigate session lists. Read message history only when context truly requires it — the active conversation is already in your context window.`,
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

	// /v1/sessions
	group := s.router.AuthGroup("/sessions")
	{
		group.GET("", s.handleListSessions())
		group.POST("", s.handleCreateSession())
		// /v1/sessions/:session_id
		group.GET("/:session_id", s.handleGetSession())
		group.DELETE("/:session_id", s.handleDeleteSession())
		// /v1/sessions/:session_id/messages
		group.GET("/:session_id/messages", s.handleListMessages())
		// /v1/sessions/:session_id/messages/:msg_id
		group.GET("/:session_id/messages/:msg_id", s.handleGetMessage())
		// /v1/sessions/:session_id/messages/compact|summarize
		group.PATCH("/:session_id/messages/compact", s.handleCompactMessages())
		group.PATCH("/:session_id/messages/summarize", s.handleSummarizeMessages())
	}

	return nil
}

func (s *SessionService) Serve(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.config.Backend {
	case "", BackendFile:
		return nil
	case BackendPlugin:
		if s.config.Plugin == "" {
			return fmt.Errorf("session backend=plugin requires plugin name")
		}
		for _, driver := range s.pluginsReg.ListDrivers() {
			if driver.Info.Name != s.config.Plugin {
				continue
			}
			p, err := driver.Driver.GetSessionsPlugin(ctx)
			if err != nil {
				return fmt.Errorf("failed to dispense sessions plugin %q: %w", s.config.Plugin, err)
			}
			if p == nil {
				return fmt.Errorf("plugin %q does not implement SessionsPlugin", s.config.Plugin)
			}
			s.store = &pluginSessionStore{plugin: p}
			s.backend = s.config.Plugin
			s.logger.Info("Bound sessions plugin", "name", s.backend)
			return nil
		}
		return fmt.Errorf("session plugin %q not found", s.config.Plugin)
	default:
		return fmt.Errorf("unknown session backend %q", s.config.Backend)
	}
}

func (s *SessionService) Cleanup(context.Context) error {
	return nil
}

var _ service.Service = (*SessionService)(nil)
