package provider

import (
	"context"

	"github.com/mwantia/forge-sdk/pkg/plugin/provider"
)

// ProviderRegistar is the narrow surface for provider-backed model operations.
type ProviderRegistar interface {
	Chat(ctx context.Context, providerName, modelName string, messages []provider.ChatMessage, tools []provider.ToolCall) (provider.ChatStream, error)
	Embed(ctx context.Context, providerName, modelName, content string) ([][]float32, error)

	ListAllModels(ctx context.Context) ([]*ModelInfo, error)
	ListModels(ctx context.Context, providerName string) ([]*provider.Model, error)
	ListModelsByType(ctx context.Context, kind string) ([]*ProviderModelTemplate, error)
	GetModel(ctx context.Context, providerName, modelName string) (*provider.Model, error)
	ResolveEmbedModel(ctx context.Context, alias string) (providerName, modelName string, err error)
	CreateLocalModel(ctx context.Context, providerName string, tmpl *ProviderModelTemplate) (*provider.Model, error)
	DeleteModel(ctx context.Context, providerName, modelName string) (bool, error)
}
