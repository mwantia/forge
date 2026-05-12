package system

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service"
	"github.com/mwantia/forge/internal/service/server"
	"github.com/mwantia/forge/internal/service/storage"
)

type SystemService struct {
	service.UnimplementedService

	router  server.HttpRouter      `fabric:"inject"`
	storage storage.StorageBackend `fabric:"inject"`
	logger  hclog.Logger           `fabric:"logger:system"`
}

func init() {
	if err := container.Register[*SystemService](
		container.AsSingleton(),
	); err != nil {
		panic(err)
	}
}

func (s *SystemService) Init(ctx context.Context) error {
	group := s.router.AuthGroup("/system")
	group.GET("/monitor", s.handleMonitor())
	group.POST("/gc", s.handleGC())
	return nil
}
