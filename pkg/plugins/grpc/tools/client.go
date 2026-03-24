package tools

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/tools/proto"
	"google.golang.org/grpc"
)

// Client implements plugins.ToolsPlugin over gRPC.
type Client struct {
	client proto.ToolsServiceClient
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{client: proto.NewToolsServiceClient(conn)}
}

func (c *Client) GetLifecycle() plugins.Lifecycle { return nil }

func (c *Client) List(ctx context.Context) (*plugins.ListToolsResponse, error) {
	resp, err := c.client.List(ctx, &proto.ListToolsReq{})
	if err != nil {
		return nil, err
	}

	result := &plugins.ListToolsResponse{}
	for _, t := range resp.Tools {
		var params map[string]any
		if t.Parameters != nil {
			params = t.Parameters.AsMap()
		}
		result.Tools = append(result.Tools, plugins.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return result, nil
}

func (c *Client) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	argsStruct, err := toStruct(req.Arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to encode arguments: %w", err)
	}

	protoReq := &proto.ExecuteReq{
		Tool:      req.Tool,
		Arguments: argsStruct,
		CallId:    req.CallID,
	}

	resp, err := c.client.Execute(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	var result any
	if resp.Result != nil {
		result = resp.Result.AsInterface()
	}

	return &plugins.ExecuteResponse{
		Result:  result,
		IsError: resp.IsError,
	}, nil
}
