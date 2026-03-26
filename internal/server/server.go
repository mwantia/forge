package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/internal/server/api"
	"github.com/mwantia/forge/internal/session"
)

type Server struct {
	engine *gin.Engine
	srv    *http.Server

	logger   hclog.Logger             `fabric:"logger:server"`
	config   *config.AgentConfig      `fabric:"config"`
	registry *registry.PluginRegistry `fabric:"inject"`
}

func (s *Server) Setup() (func() error, error) {
	gin.SetMode(gin.ReleaseMode)
	s.engine = gin.New()
	s.srv = &http.Server{
		Addr:    s.config.Server.Address,
		Handler: s.engine,
	}

	mgr := session.NewManager(s.logger, s.config.DataDir, s.registry)

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

	// Sessions
	authed.GET("sessions", api.ListSessions(mgr))
	authed.POST("sessions", api.CreateSession(mgr))
	authed.GET("sessions/:id", api.GetSession(mgr))
	authed.DELETE("sessions/:id", api.DeleteSession(mgr))
	authed.GET("sessions/:id/messages", api.ListMessages(mgr))
	authed.POST("sessions/:id/messages", api.AddMessage(mgr))

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
