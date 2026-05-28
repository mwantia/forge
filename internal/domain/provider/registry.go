package provider

import (
	"context"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
)

// ProviderRegistar is the narrow surface for provider-backed model operations.
type ProviderRegistar interface {
	Chat(ctx context.Context, providerName, modelName string, messages []sdkplugins.ChatMessage, tools []sdkplugins.ToolCall) (sdkplugins.ChatStream, error)
	Embed(ctx context.Context, providerName, modelName, content string) ([][]float32, error)

	ListAllModels(ctx context.Context) (map[string][]*sdkplugins.Model, []*ProviderModelTemplate, error)
	ListModels(ctx context.Context, providerName string) ([]*sdkplugins.Model, error)
	ListModelsByType(ctx context.Context, kind string) ([]*ProviderModelTemplate, error)
	GetModel(ctx context.Context, providerName, modelName string) (*sdkplugins.Model, error)
	ResolveEmbedModel(ctx context.Context, alias string) (providerName, modelName string, err error)
	CreateLocalModel(ctx context.Context, providerName string, tmpl *ProviderModelTemplate) (*sdkplugins.Model, error)
	DeleteModel(ctx context.Context, providerName, modelName string) (bool, error)
}
