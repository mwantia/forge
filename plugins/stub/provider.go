package stub

import (
	"context"

	"github.com/mwantia/forge/pkg/plugins"
)

// StubProviderPlugin implements ProviderPlugin.
type StubProviderPlugin struct {
	driver *StubDriver
}

func (p *StubProviderPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *StubProviderPlugin) GetPluginInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Type:    plugins.PluginTypeProvider,
		Name:    "stub-provider",
		Author:  "forge",
		Version: "0.1.0",
	}
}

func (p *StubProviderPlugin) Generate(ctx context.Context, req plugins.GenerateRequest) (*plugins.GenerateResponse, error) {
	return &plugins.GenerateResponse{
		ID:      "stub-response",
		Content: "This is a stub response from the provider plugin.",
		Role:    "assistant",
		Model:   req.Model,
	}, nil
}
