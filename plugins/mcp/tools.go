package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mwantia/forge/pkg/plugins"
)

func (d *MCPDriver) ListTools(_ context.Context, _ plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var defs []plugins.ToolDefinition
	for serverName, srv := range d.servers {
		for _, tool := range srv.tools {
			defs = append(defs, plugins.ToolDefinition{
				Name:        serverName + "__" + tool.Name,
				Description: tool.Description,
				Parameters:  inputSchemaToParams(tool.InputSchema),
			})
		}
	}
	return &plugins.ListToolsResponse{Tools: defs}, nil
}

func (d *MCPDriver) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	serverName, toolName, ok := strings.Cut(name, "__")
	if !ok {
		return nil, fmt.Errorf("invalid tool name %q: expected serverName__toolName", name)
	}

	d.mu.RLock()
	srv, exists := d.servers[serverName]
	d.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("unknown MCP server %q", serverName)
	}

	for _, tool := range srv.tools {
		if tool.Name == toolName {
			return &plugins.ToolDefinition{
				Name:        name,
				Description: tool.Description,
				Parameters:  inputSchemaToParams(tool.InputSchema),
			}, nil
		}
	}
	return nil, fmt.Errorf("tool %q not found on server %q", toolName, serverName)
}

func (d *MCPDriver) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	serverName, toolName, ok := strings.Cut(req.Tool, "__")
	if !ok {
		return nil, fmt.Errorf("invalid tool name %q: expected serverName__toolName", req.Tool)
	}

	d.mu.RLock()
	srv, exists := d.servers[serverName]
	d.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("unknown MCP server %q", serverName)
	}

	return srv.callTool(ctx, toolName, req.Arguments)
}

func (d *MCPDriver) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	serverName, toolName, ok := strings.Cut(req.Tool, "__")
	if !ok {
		return &plugins.ValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("invalid tool name %q: expected serverName__toolName", req.Tool)},
		}, nil
	}

	d.mu.RLock()
	srv, exists := d.servers[serverName]
	d.mu.RUnlock()
	if !exists {
		return &plugins.ValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("unknown MCP server %q", serverName)},
		}, nil
	}

	for _, tool := range srv.tools {
		if tool.Name == toolName {
			return &plugins.ValidateResponse{Valid: true}, nil
		}
	}
	return &plugins.ValidateResponse{
		Valid:  false,
		Errors: []string{fmt.Sprintf("tool %q not found on server %q", toolName, serverName)},
	}, nil
}
