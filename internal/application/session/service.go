package session

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	approot "github.com/mwantia/forge/internal/application"
	domresource "github.com/mwantia/forge/internal/domain/resource"
	domtool "github.com/mwantia/forge/internal/domain/tool"
	inframetrics "github.com/mwantia/forge/internal/infrastructure/metrics"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
	infrastorage "github.com/mwantia/forge/internal/infrastructure/storage"
)

const ServiceNamespace = "sessions"

type SessionService struct {
	approot.UnimplementedService

	mu    sync.RWMutex
	store *DagStore

	metrics   inframetrics.MetricsRegistar `fabric:"inject"`
	router    infraserver.HttpRouter       `fabric:"inject"`
	storage   infrastorage.StorageBackend  `fabric:"inject"`
	tools     domtool.ToolsRegistar        `fabric:"inject"`
	resources domresource.ResourceRegistar `fabric:"inject"`
	logger    hclog.Logger                 `fabric:"logger=session"`
}

func init() {
	container.MustRegister[*SessionService](
		container.AsSingleton(),
		container.With[SessionManager](),
	)
}

func (*SessionService) PreInit(context.Context) error {
	return nil
}

func (s *SessionService) PostInit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.metrics.Register(SessionsTotal); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	s.store = NewDagStore(s.storage)

	metadata := domtool.NamespaceMetadata{
		Description: "Built-in session bookkeeping: title/description, sub-session creation, message history.",
		Builtin:     true,
		System: `
Built-in session tools manage the conversation's own metadata and any sub-sessions you spawn. Update title/description when the topic crystallises so the user can navigate session lists.
Read message history only when context truly requires it — the active conversation is already in your context window.
`,
	}
	if err := s.tools.RegisterNamespaceMetadata(ServiceNamespace, metadata); err != nil {
		return fmt.Errorf("failed to register namespace metadata for %q: %w", ServiceNamespace, err)
	}

	for _, definition := range ToolsDefinitions {
		captured := definition
		exec := func(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
			req.Tool = captured.Name
			return s.ExecuteTool(ctx, req)
		}
		if err := s.tools.RegisterTool(ServiceNamespace, definition, exec); err != nil {
			return fmt.Errorf("failed to register tool %q for namespace %q: %w", definition.Name, ServiceNamespace, err)
		}
	}

	// /v1/sessions
	group := s.router.AuthGroup("/" + ServiceNamespace)
	{
		group.GET("", s.handleListSessions())
		group.POST("", s.handleCreateSession())
		// /v1/sessions/:session_id
		group.GET("/:session_id", s.handleGetSession())
		group.PATCH("/:session_id", s.handleUpdateSession())
		group.DELETE("/:session_id", s.handleDeleteSession())
		// /v1/sessions/:session_id/messages
		group.GET("/:session_id/messages", s.handleListMessages())
		// /v1/sessions/:session_id/messages/:msg_id
		group.GET("/:session_id/messages/:msg_id", s.handleGetMessage())
		// /v1/sessions/:session_id/messages/compact|summarize
		group.PATCH("/:session_id/messages/compact", s.handleCompactMessages())
		group.PATCH("/:session_id/messages/summarize", s.handleSummarizeMessages())
		// /v1/sessions/:session_id/refs
		group.GET("/:session_id/refs", s.handleListRefs())
		group.POST("/:session_id/refs", s.handleCreateRef())
		group.PATCH("/:session_id/refs/:ref", s.handleUpdateRef())
		group.DELETE("/:session_id/refs/:ref", s.handleDeleteRef())
		group.POST("/:session_id/refs/:ref/revert", s.handleRevertRef())
		// /v1/sessions/:session_id/messages/:msg_id/diff?to=<hash_b>
		group.GET("/:session_id/messages/:msg_id/diff", s.handleMessageDiff())
		// /v1/sessions/:session_id/archive|clone
		group.POST("/:session_id/archive", s.handleArchiveSession())
		group.POST("/:session_id/clone", s.handleCloneSession())
	}

	return nil
}

func (s *SessionService) Cleanup(context.Context) error {
	return nil
}

var _ approot.Service = (*SessionService)(nil)
