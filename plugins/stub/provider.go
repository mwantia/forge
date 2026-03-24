package stub

import (
	"context"

	"github.com/mwantia/forge/pkg/plugins"
)

// StubProviderPlugin implements ProviderPlugin.
type StubProviderPlugin struct {
	plugins.UnimplementedProviderPlugin
	driver *StubDriver
}

func (p *StubProviderPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *StubProviderPlugin) Chat(ctx context.Context, messages []plugins.ChatMessage, tools []plugins.ToolCall, model *plugins.Model) (*plugins.ChatResult, error) {
	name := ""
	if model != nil {
		name = model.ModelName
	}
	return &plugins.ChatResult{
		ID:      "stub-result",
		Role:    "assistant",
		Content: "This is a stub response from the provider plugin.",
		Metadata: map[string]any{
			"model": name,
		},
	}, nil
}

func (p *StubProviderPlugin) Embed(ctx context.Context, content string, model *plugins.Model) ([][]float32, error) {
	return [][]float32{{0.1, 0.2, 0.3}}, nil
}

func (p *StubProviderPlugin) ListModels(ctx context.Context) ([]*plugins.Model, error) {
	return []*plugins.Model{
		{ModelName: "stub-model"},
	}, nil
}

func (p *StubProviderPlugin) GetModel(ctx context.Context, name string) (*plugins.Model, error) {
	return &plugins.Model{ModelName: name}, nil
}
