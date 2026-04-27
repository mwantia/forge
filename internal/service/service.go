package service

import (
	"context"

	"github.com/mwantia/fabric/pkg/container"
)

type Service interface {
	container.LifecycleService

	Serve(context.Context) error
}
