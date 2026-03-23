package provider

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/provider/proto"
	"google.golang.org/grpc"
)

// Client implements plugins.ProviderPlugin over gRPC.
type Client struct {
	client proto.ProviderServiceClient
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{client: proto.NewProviderServiceClient(conn)}
}

func (c *Client) GetLifecycle() plugins.Lifecycle { return nil }

func (c *Client) Generate(ctx context.Context, req plugins.GenerateRequest) (*plugins.GenerateResponse, error) {
	protoReq := &proto.GenerateReq{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   int32(req.MaxTokens),
	}
	for _, m := range req.Messages {
		protoReq.Messages = append(protoReq.Messages, &proto.MessageProto{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	for _, t := range req.Tools {
		params := make(map[string]string)
		for k, v := range t.Parameters {
			params[k] = fmt.Sprintf("%v", v)
		}
		protoReq.Tools = append(protoReq.Tools, &proto.ToolProto{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}

	resp, err := c.client.Generate(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	result := &plugins.GenerateResponse{
		ID:      resp.Id,
		Content: resp.Content,
		Role:    resp.Role,
		Model:   resp.Model,
	}
	if resp.Usage != nil {
		result.Usage = &plugins.Usage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
		}
	}
	return result, nil
}
