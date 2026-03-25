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
)

type Server struct {
	engine *gin.Engine
	srv    *http.Server

	logger   hclog.Logger             `fabric:"logger:server"`
	config   *config.ServerConfig     `fabric:"config:server"`
	registry *registry.PluginRegistry `fabric:"inject"`
}

func (s *Server) Setup() (func() error, error) {
	gin.SetMode(gin.ReleaseMode)
	s.engine = gin.New()
	s.srv = &http.Server{
		Addr:    s.config.Address,
		Handler: s.engine,
	}

	s.engine.Use(s.LoggerHandler(), s.Recovery())

	v1 := s.engine.Group("/v1")
	v1.GET("health", api.Health())

	return func() error {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		s.logger.Debug("Performing server shutdown...")
		return s.srv.Shutdown(shutdown)
	}, nil
}

func (s *Server) Serve(ctx context.Context) error {
	s.logger.Info("Serving http server", "address", s.config.Address)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
