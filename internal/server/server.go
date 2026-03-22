package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/server/api"
)

type Server struct {
	log hclog.Logger
	cfg config.ServerConfig

	engine *gin.Engine
	srv    *http.Server
}

func NewServer(cfg config.AgentConfig, log hclog.Logger) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	srv := &http.Server{
		Addr:    cfg.Server.Address,
		Handler: engine,
	}

	return &Server{
		log:    log.Named("server"),
		cfg:    *cfg.Server,
		engine: engine,
		srv:    srv,
	}, nil
}

func (impl *Server) Setup() (func() error, error) {
	impl.engine.Use(impl.Logger(), impl.Recovery())

	v1 := impl.engine.Group("/v1")
	v1.GET("health", api.Health())

	return func() error {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		impl.log.Debug("Performing server shutdown...")
		return impl.srv.Shutdown(shutdown)
	}, nil
}

func (impl *Server) Serve(ctx context.Context) error {
	impl.log.Info("Serving http server", "address", impl.cfg.Address)
	if err := impl.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
