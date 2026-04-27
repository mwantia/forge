package sandbox

import (
	"context"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service"
)

type SandboxService struct {
	service.Service

	mu sync.RWMutex

	logger hclog.Logger `fabric:"logger:sandbox"`
}

func init() {
	if err := container.Register[*SandboxService](
		container.AsSingleton(),
	); err != nil {
		panic(err)
	}
}

func (r *SandboxService) Init(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return nil
}

func (r *SandboxService) Cleanup(context.Context) error {
	return nil
}

func (r *SandboxService) Serve(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return nil
}

var _ service.Service = (*SandboxService)(nil)
