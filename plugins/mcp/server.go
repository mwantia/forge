package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mwantia/forge/pkg/plugins"
)

type mcpServer struct {
	name   string
	cfg    MCPServerConfig
	log    hclog.Logger
	client *mcpclient.Client
	tools  []mcp.Tool
}

func newMCPServer(name string, cfg MCPServerConfig, log hclog.Logger) *mcpServer {
	return &mcpServer{
		name: name,
		cfg:  cfg,
		log:  log.Named(name),
	}
}

func (s *mcpServer) connect(ctx context.Context) error {
	var (
		c   *mcpclient.Client
		err error
	)

	switch s.cfg.Transport {
	case "sse":
		if s.cfg.URL == "" {
			return fmt.Errorf("url is required for sse transport")
		}
		c, err = mcpclient.NewSSEMCPClient(s.cfg.URL, mcpclient.WithHeaders(s.cfg.Headers))
		if err != nil {
			return fmt.Errorf("failed to create SSE client: %w", err)
		}

	case "stdio":
		if s.cfg.Command == "" {
			return fmt.Errorf("command is required for stdio transport")
		}
		c, err = mcpclient.NewStdioMCPClient(s.cfg.Command, nil, s.cfg.Args...)
		if err != nil {
			return fmt.Errorf("failed to create stdio client: %w", err)
		}

	default:
		return fmt.Errorf("unsupported transport %q: must be \"sse\" or \"stdio\"", s.cfg.Transport)
	}

	if err := c.Start(ctx); err != nil {
		return fmt.Errorf("failed to start MCP transport: %w", err)
	}

	if _, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "forge",
				Version: "0.1.0",
			},
		},
	}); err != nil {
		_ = c.Close()
		return fmt.Errorf("failed to initialize MCP session: %w", err)
	}

	resp, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		_ = c.Close()
		return fmt.Errorf("failed to list tools: %w", err)
	}

	s.client = c
	s.tools = resp.Tools
	return nil
}

func (s *mcpServer) close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *mcpServer) callTool(ctx context.Context, name string, args map[string]any) (*plugins.ExecuteResponse, error) {
	result, err := s.client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("MCP call failed: %w", err)
	}

	text := mcpContentToText(result.Content)
	return &plugins.ExecuteResponse{
		Result:  text,
		IsError: result.IsError,
	}, nil
}

// mcpContentToText joins all text content items into a single string.
// Non-text items (images, audio, etc.) are noted as placeholders.
func mcpContentToText(contents []mcp.Content) string {
	parts := make([]string, 0, len(contents))
	for _, c := range contents {
		if tc, ok := mcp.AsTextContent(c); ok {
			parts = append(parts, tc.Text)
		} else {
			b, _ := json.Marshal(c)
			parts = append(parts, string(b))
		}
	}
	return strings.Join(parts, "\n")
}

// inputSchemaToParams converts an MCP ToolInputSchema to the map[string]any
// format expected by forge's ToolDefinition.Parameters.
func inputSchemaToParams(schema mcp.ToolInputSchema) map[string]any {
	b, _ := json.Marshal(schema)
	var result map[string]any
	_ = json.Unmarshal(b, &result)
	return result
}
