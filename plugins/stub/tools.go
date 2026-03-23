package stub

import (
	"context"

	"github.com/mwantia/forge/pkg/plugins"
)

// StubToolsPlugin implements ToolsPlugin.
type StubToolsPlugin struct {
	driver *StubDriver
}

func (p *StubToolsPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *StubToolsPlugin) List(ctx context.Context) (*plugins.ListToolsResponse, error) {
	return &plugins.ListToolsResponse{
		Tools: []plugins.ToolDefinition{
			{
				Name:        "stub_tool",
				Description: "A stub tool for testing",
				Parameters: map[string]any{
					"param1": map[string]any{
						"type":        "string",
						"description": "A test parameter",
					},
				},
			},
		},
	}, nil
}

func (p *StubToolsPlugin) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	return &plugins.ExecuteResponse{
		Result:  "stub_tool executed successfully",
		IsError: false,
	}, nil
}
