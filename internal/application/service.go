package application

import (
	"context"

	"github.com/mwantia/fabric/v2/pkg/container"
)

type Service interface {
	container.CleanupLifecycle
	container.PreLifecycle
	container.PostLifecycle

	Serve(context.Context) error
}
