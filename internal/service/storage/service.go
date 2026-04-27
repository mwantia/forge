package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/service"
	"github.com/mwantia/forge/internal/service/metrics"
	"github.com/mwantia/forge/internal/service/storage/file"
	"github.com/mwantia/forge/internal/service/template"
)

type StorageService struct {
	service.Service
	StorageBackend

	mu      sync.RWMutex
	backend StorageBackend

	tmpl    template.TemplateRenderer `fabric:"inject"`
	metrics metrics.MetricsRegistar   `fabric:"inject"`
	config  StorageConfig             `fabric:"config:storage"`
	logger  hclog.Logger              `fabric:"logger:storage"`
}

func init() {
	if err := container.Register[*StorageService](
		container.AsSingleton(),
		container.With[StorageBackend](),
	); err != nil {
		panic(err)
	}
}

func (s *StorageService) Init(ctx context.Context) error {
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
	default:
		return fmt.Errorf("unknown storage backend %q", s.config.Type)
	}

	if serv, ok := backend.(service.Service); ok {
		if err := serv.Init(ctx); err != nil {
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

	if serv, ok := s.backend.(service.Service); ok {
		return serv.Cleanup(ctx)
	}

	return nil
}

func (s *StorageService) Serve(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if serv, ok := s.backend.(service.Service); ok {
		return serv.Serve(ctx)
	}

	return nil
}

var _ service.Service = (*StorageService)(nil)
