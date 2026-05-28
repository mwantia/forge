package system

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	approot "github.com/mwantia/forge/internal/application"
	domplugin "github.com/mwantia/forge/internal/domain/plugin"
	domtool "github.com/mwantia/forge/internal/domain/tool"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
	infrastorage "github.com/mwantia/forge/internal/infrastructure/storage"
)

type SystemService struct {
	approot.UnimplementedService

	router  infraserver.HttpRouter      `fabric:"inject"`
	storage infrastorage.StorageBackend `fabric:"inject"`
	plugins domplugin.PluginsRegistry   `fabric:"inject"`
	tools   domtool.ToolsRegistar       `fabric:"inject"`
	logger  hclog.Logger                `fabric:"logger=system"`
}

func init() {
	container.MustRegister[*SystemService](container.AsSingleton())
}

func (*SystemService) PreInit(context.Context) error {
	return nil
}

func (s *SystemService) PostInit(ctx context.Context) error {
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
