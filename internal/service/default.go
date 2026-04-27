package service

import "context"

type UnimplementedService struct {
	Service
}

func (*UnimplementedService) Cleanup(context.Context) error {
	return nil
}

// Init implements [Service].
func (*UnimplementedService) Init(context.Context) error {
	return nil
}

// Serve implements [Service].
func (*UnimplementedService) Serve(ctx context.Context) error {
	return nil
}

var _ Service = (*UnimplementedService)(nil)
