package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/internal/channel"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/internal/sandbox"
	"github.com/mwantia/forge/internal/server/api"
	"github.com/mwantia/forge/internal/session"
)

type Server struct {
	engine *gin.Engine
	srv    *http.Server

	logger     hclog.Logger               `fabric:"logger:server"`
	config     *config.AgentConfig        `fabric:"config"`
	registry   *registry.PluginRegistry   `fabric:"inject"`
	dispatcher *channel.ChannelDispatcher `fabric:"inject"`
	sessions   *session.SessionManager    `fabric:"inject"`
	sandboxes  *sandbox.SandboxManager    `fabric:"inject"`
}

func (s *Server) Setup() (func() error, error) {
	gin.SetMode(gin.ReleaseMode)
	s.engine = gin.New()
	s.srv = &http.Server{
		Addr:    s.config.Server.Address,
		Handler: s.engine,
	}

	// Initialise channel binding stores before registering routes.
	if s.dispatcher != nil {
		if _, err := s.dispatcher.Setup(); err != nil {
			return nil, err
		}
	}

	s.engine.Use(s.LoggerHandler(), s.Recovery())

	v1 := s.engine.Group("/v1")
	v1.GET("health", api.Health())

	authed := v1.Group("/", s.AuthMiddleware())

	// Plugins
	authed.GET("plugins", api.ListPlugins(s.registry))
	authed.GET("plugins/:name", api.GetPlugin(s.registry))

	// Models
	authed.GET("models", api.ListModels(s.registry))
	authed.GET("models/:provider", api.ListProviderModels(s.registry))
	authed.POST("models/:provider", api.CreateModel(s.registry))
	authed.DELETE("models/:provider/:model", api.DeleteModel(s.registry))

	// Embeddings
	authed.POST("embeddings", api.Embed(s.registry))

	// Tools
	authed.GET("tools", api.ListTools(s.registry))
	authed.GET("tools/:driver", api.ListDriverTools(s.registry))
	authed.GET("tools/:driver/:tool", api.GetDriverTool(s.registry))
	authed.POST("tools/:driver/:tool/validate", api.ValidateTool(s.registry))
	authed.POST("tools/:driver/:tool/execute", api.ExecuteTool(s.registry))
	authed.DELETE("tools/:driver/cancel/:call_id", api.CancelTool(s.registry))

	// Channel bindings
	if s.dispatcher != nil {
		authed.GET("channels/:plugin/bindings", api.ListChannelBindings(s.dispatcher))
		authed.POST("channels/:plugin/bind", api.BindChannel(s.dispatcher, s.sessions))
		authed.DELETE("channels/:plugin/bind/:channel_id", api.UnbindChannel(s.dispatcher))
	}

	// Sessions
	authed.GET("sessions", api.ListSessions(s.sessions))
	authed.POST("sessions", api.CreateSession(s.sessions))
	authed.GET("sessions/:id", api.GetSession(s.sessions))
	authed.DELETE("sessions/:id", api.DeleteSession(s.sessions))
	authed.GET("sessions/:id/tools", api.ListSessionTools(s.sessions))
	authed.GET("sessions/:id/messages", api.ListMessages(s.sessions))
	authed.POST("sessions/:id/messages", api.AddMessage(s.sessions))
	authed.GET("sessions/:id/messages/:message_id", api.GetMessage(s.sessions))
	authed.POST("sessions/:id/messages/compact", api.CompactMessages(s.sessions))
	authed.GET("sessions/:id/sandboxes", api.ListSessionSandboxes(s.sandboxes))

	// Sandboxes
	authed.GET("sandboxes", api.ListSandboxes(s.sandboxes))
	authed.POST("sandboxes", api.CreateSandbox(s.sandboxes))
	authed.GET("sandboxes/:id", api.GetSandbox(s.sandboxes))
	authed.DELETE("sandboxes/:id", api.DeleteSandbox(s.sandboxes))
	authed.POST("sandboxes/:id/exec", api.ExecSandbox(s.sandboxes))
	authed.POST("sandboxes/:id/copy-in", api.CopyInSandbox(s.sandboxes))
	authed.POST("sandboxes/:id/copy-out", api.CopyOutSandbox(s.sandboxes))
	authed.GET("sandboxes/:id/stat", api.StatSandbox(s.sandboxes))
	authed.GET("sandboxes/:id/read", api.ReadFileSandbox(s.sandboxes))

	return func() error {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		s.logger.Debug("Performing server shutdown...")
		return s.srv.Shutdown(shutdown)
	}, nil
}

func (s *Server) Serve(ctx context.Context) error {
	s.logger.Info("Serving http server", "address", s.config.Server.Address)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
