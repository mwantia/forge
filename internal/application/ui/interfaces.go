package ui

import (
	"context"

	"github.com/mwantia/forge-sdk/pkg/plugin/provider"
	apppipeline "github.com/mwantia/forge/internal/application/pipeline"
	appsession "github.com/mwantia/forge/internal/application/session"
	domprovider "github.com/mwantia/forge/internal/domain/provider"
	domsession "github.com/mwantia/forge/internal/domain/session"
	domtool "github.com/mwantia/forge/internal/domain/tool"
)

// sessionReader is the narrow surface UIService handlers need.
// *appsession.SessionService satisfies this interface.
type sessionReader interface {
	domsession.SessionManager
	ListParentSessions(ctx context.Context, parentID string, archived bool, offset, limit int) ([]*appsession.SessionMetadata, error)
	QuerySessions(ctx context.Context, q appsession.SessionQuery) ([]*appsession.SessionMetadata, error)
	CreateSession(ctx context.Context, model, name, title, description, parent string, plugins []appsession.PluginConfig) (*appsession.SessionMetadata, error)
	DeleteSession(ctx context.Context, idOrName string) error
	ArchiveSession(ctx context.Context, sessionID, refName, name string) (*appsession.ArchiveResult, error)
}

// pluginNamespace is the minimal view of a registered namespace exposed to UI handlers.
type pluginNamespace struct {
	Name    string
	Builtin bool
}

// namespaceLister is the narrow surface UIService needs from ToolsRegistar.
type namespaceLister interface {
	ListNamespaces() []domtool.NamespaceInfo
}

// modelLister is the narrow surface UIService needs from ProviderRegistar.
type modelLister interface {
	ListModelsByType(ctx context.Context, kind string) ([]*domprovider.ProviderModelTemplate, error)
	GetModel(ctx context.Context, providerName, modelName string) (*provider.Model, error)
}

// pipelineCommitter aliases the pipeline package interface to avoid re-declaring it.
type pipelineCommitter = apppipeline.PipelineCommitter

// pipelineRenderer aliases the pipeline package interface to avoid re-declaring it.
type pipelineRenderer = apppipeline.PipelineRenderer
