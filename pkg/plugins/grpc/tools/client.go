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
		params := make(map[string]any)
		for k, v := range t.Parameters {
			params[k] = v
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
	protoReq := &proto.ExecuteReq{
		Tool:      req.Tool,
		Arguments: make(map[string]string),
	}
	for k, v := range req.Arguments {
		protoReq.Arguments[k] = fmt.Sprintf("%v", v)
	}

	resp, err := c.client.Execute(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	resultMap := make(map[string]any)
	for k, v := range resp.Result {
		resultMap[k] = v
	}

	return &plugins.ExecuteResponse{
		Result:  resultMap,
		IsError: resp.IsError,
	}, nil
}
