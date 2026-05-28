package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	approot "github.com/mwantia/forge/internal/application"
	"github.com/mwantia/forge/internal/config"
	inframetrics "github.com/mwantia/forge/internal/infrastructure/metrics"
	"github.com/mwantia/forge/internal/infrastructure/storage/file"
	"github.com/mwantia/forge/internal/infrastructure/storage/postgres"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
)

type StorageService struct {
	approot.Service
	StorageBackend

	mu      sync.RWMutex
	backend StorageBackend

	tmpl    infratemplate.TemplateRenderer `fabric:"inject"`
	metrics inframetrics.MetricsRegistar   `fabric:"inject"`
	config  StorageConfig                  `fabric:"config=storage"`
	logger  hclog.Logger                   `fabric:"logger=storage"`
}

func init() {
	container.MustRegister[*StorageService](
		container.AsSingleton(),
		container.With[StorageBackend](),
	)
}

func (*StorageService) PreInit(context.Context) error {
	return nil
}

func (s *StorageService) PostInit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger := s.logger.Named(s.config.Type)

	var backend StorageBackend
	switch strings.ToLower(s.config.Type) {
	case "file":
		cfg, err := config.Decode[FileConfig](s.config.Body, s.tmpl.Base())
		if err != nil {
			return fmt.Errorf("storage file config: %w", err)
		}
		backend = file.NewFileStorageBackend(logger, cfg.Path)
	case "postgres":
		cfg, err := config.Decode[PostgresConfig](s.config.Body, s.tmpl.Base())
		if err != nil {
			return fmt.Errorf("storage postgres config: %w", err)
		}
		backend = postgres.NewPostgresBackend(logger, cfg.DSN, cfg.MaxConns)
	default:
		return fmt.Errorf("unknown storage backend %q", s.config.Type)
	}

	if serv, ok := backend.(approot.Service); ok {
		if err := serv.PostInit(ctx); err != nil {
			return fmt.Errorf("failed to initialize backend: %w", err)
		}
	}

	if err := s.metrics.Register(OperationsTotal, OperationDuration, BytesTotal); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	s.backend = backend
	return nil
}

func (s *StorageService) Cleanup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if serv, ok := s.backend.(approot.Service); ok {
		return serv.Cleanup(ctx)
	}

	return nil
}

func (s *StorageService) Serve(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if serv, ok := s.backend.(approot.Service); ok {
		return serv.Serve(ctx)
	}

	return nil
}

var _ approot.Service = (*StorageService)(nil)
