package stub

import (
	"context"

	"github.com/mwantia/forge/pkg/plugins"
)

// StubMemoryPlugin implements MemoryPlugin.
type StubMemoryPlugin struct {
	driver *StubDriver
}

func (p *StubMemoryPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *StubMemoryPlugin) GetPluginInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Type:    plugins.PluginTypeMemory,
		Name:    "stub-memory",
		Author:  "forge",
		Version: "0.1.0",
	}
}

func (p *StubMemoryPlugin) Store(ctx context.Context, req plugins.StoreRequest) (*plugins.StoreResponse, error) {
	return &plugins.StoreResponse{
		ID: "stub-memory-id",
	}, nil
}

func (p *StubMemoryPlugin) Retrieve(ctx context.Context, req plugins.RetrieveRequest) (*plugins.RetrieveResponse, error) {
	return &plugins.RetrieveResponse{
		Results: []plugins.MemoryResult{
			{
				ID:      "stub-memory-id",
				Content: "This is a stub memory result.",
				Score:   1.0,
			},
		},
	}, nil
}
