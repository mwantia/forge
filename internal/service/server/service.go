package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	_ "github.com/mwantia/forge/docs"
	"github.com/mwantia/forge/internal/service"
	svcmetrics "github.com/mwantia/forge/internal/service/metrics"
	"github.com/mwantia/forge/internal/service/server/api"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type ServerService struct {
	service.UnimplementedService

	mu     sync.RWMutex
	engine *gin.Engine
	srv    *http.Server

	public *gin.RouterGroup
	auth   *gin.RouterGroup

	metrics svcmetrics.MetricsRegistar `fabric:"inject"`
	config  ServerConfig               `fabric:"config:server"`
	logger  hclog.Logger               `fabric:"logger:server"`
}

func init() {
	if err := container.Register[*ServerService](
		container.AsSingleton(),
		container.With[HttpRouter](),
	); err != nil {
		panic(err)
	}
}

func (s *ServerService) Init(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config.Address == "" {
		s.config.Address = "127.0.0.1:9280"
	}

	if err := s.metrics.Register(ServerHttpRequestsTotal, ServerHttpRequestsDuration); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	s.engine = gin.New()
	s.srv = &http.Server{
		Addr:    s.config.Address,
		Handler: s.engine,
	}

	s.engine.Use(s.loggerHandler(), s.recovery())

	v1 := s.engine.Group("/v1")
	s.public = v1.Group("/")
	s.auth = v1.Group("/", s.authMiddleware())

	s.public.GET("/health", api.Health())

	if sw := s.config.Swagger; sw != nil {
		path := sw.Path
		if path == "" {
			path = "/swagger"
		}
		swaggerHandler := ginSwagger.WrapHandler(swaggerFiles.Handler,
			ginSwagger.URL(path+"/doc.json"),
		)
		s.engine.GET(path+"/*any", func(c *gin.Context) {
			if c.Param("any") == "/" {
				c.Redirect(http.StatusMovedPermanently, path+"/index.html")
				return
			}
			swaggerHandler(c)
		})
		s.logger.Info("Swagger UI enabled", "path", path)
	}

	return nil
}

func (s *ServerService) Cleanup(ctx context.Context) error {
	shutdown, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	s.logger.Debug("Performing server shutdown...")
	return s.srv.Shutdown(shutdown)
}

func (s *ServerService) Serve(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Serving http server", "address", s.config.Address)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

var _ service.Service = (*ServerService)(nil)
