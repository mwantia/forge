package system

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service"
	"github.com/mwantia/forge/internal/service/plugins"
	"github.com/mwantia/forge/internal/service/server"
	"github.com/mwantia/forge/internal/service/storage"
	"github.com/mwantia/forge/internal/service/tools"
)

type SystemService struct {
	service.UnimplementedService

	router  server.HttpRouter       `fabric:"inject"`
	storage storage.StorageBackend  `fabric:"inject"`
	plugins plugins.PluginsRegistry `fabric:"inject"`
	tools   tools.ToolsRegistar     `fabric:"inject"`
	logger  hclog.Logger            `fabric:"logger:system"`
}

func init() {
	if err := container.Register[*SystemService](
		container.AsSingleton(),
	); err != nil {
		panic(err)
	}
}

func (s *SystemService) Init(ctx context.Context) error {
	if err := s.registerForgeTools(); err != nil {
		return err
	}

	group := s.router.AuthGroup("/system")
	{
		group.GET("/monitor", s.handleMonitor())
		group.GET("/health", s.handleSystemHealth())
		group.POST("/gc", s.handleGC())

		dag := group.Group("/dag")
		{
			dag.GET("/objects", s.handleDagObjects())
			dag.GET("/objects/:hash", s.handleDagCat())
			dag.GET("/objects/:hash/type", s.handleDagType())
			dag.GET("/sessions/:id/log", s.handleDagLog())
			dag.GET("/diff", s.handleDagDiff())
			dag.POST("/verify", s.handleDagVerify())
			dag.POST("/gc", s.handleDagGC())
		}
	}

	return nil
}
