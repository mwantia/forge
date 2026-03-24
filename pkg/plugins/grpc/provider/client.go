package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/provider/proto"
	"google.golang.org/grpc"
)

// Client implements plugins.ProviderPlugin over gRPC.
// Unimplemented capabilities fall back to UnimplementedProviderPlugin.
type Client struct {
	plugins.UnimplementedProviderPlugin
	client proto.ProviderServiceClient
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{client: proto.NewProviderServiceClient(conn)}
}

func (c *Client) Chat(ctx context.Context, messages []plugins.ChatMessage, tools []plugins.ToolCall, model *plugins.Model) (*plugins.ChatResult, error) {
	protoReq := &proto.GenerateReq{}
	if model != nil {
		protoReq.Model = model.ModelName
		protoReq.Temperature = model.Temperature
	}
	for _, m := range messages {
		protoReq.Messages = append(protoReq.Messages, &proto.MessageProto{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	for _, t := range tools {
		params := make(map[string]string)
		for k, v := range t.Parameters {
			b, err := json.Marshal(v)
			if err != nil {
				params[k] = fmt.Sprintf("%v", v)
			} else {
				params[k] = string(b)
			}
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

	return &plugins.ChatResult{
		ID:      resp.Id,
		Role:    resp.Role,
		Content: resp.Content,
	}, nil
}
