package service

import (
	"context"

	"github.com/mwantia/fabric/pkg/container"
)

type Service interface {
	container.Lifecycle
	container.PreLifecycle
	container.PostLifecycle

	Serve(context.Context) error
}
