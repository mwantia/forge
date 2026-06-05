package ui

import (
	"context"

	apppipeline "github.com/mwantia/forge/internal/application/pipeline"
	appsession "github.com/mwantia/forge/internal/application/session"
	domsession "github.com/mwantia/forge/internal/domain/session"
)

// sessionReader is the narrow surface UIService handlers need.
// *appsession.SessionService satisfies this interface.
type sessionReader interface {
	domsession.SessionManager
	ListParentSessions(ctx context.Context, parentID string, archived bool, offset, limit int) ([]*appsession.SessionMetadata, error)
	CreateSession(ctx context.Context, model, name, title, description, parent, toolsVerbosity string, plugins []string) (*appsession.SessionMetadata, error)
	DeleteSession(ctx context.Context, idOrName string) error
}

// pipelineCommitter aliases the pipeline package interface to avoid re-declaring it.
type pipelineCommitter = apppipeline.PipelineCommitter
