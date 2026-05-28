package application

import (
	"context"

	// Ensure the hclog tag processor is registered before any application
	// service init() validates its fabric:"logger:*" struct tags.
	_ "github.com/mwantia/forge/internal/log"
)

type UnimplementedService struct {
	Service
}

func (*UnimplementedService) Cleanup(context.Context) error {
	return nil
}

func (*UnimplementedService) PreInit(ctx context.Context) error {
	return nil
}

func (*UnimplementedService) PostInit(ctx context.Context) error {
	return nil
}

func (*UnimplementedService) Serve(ctx context.Context) error {
	return nil
}

var _ Service = (*UnimplementedService)(nil)
