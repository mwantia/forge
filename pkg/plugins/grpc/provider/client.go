package provider

import (
	"context"
	"fmt"
	"io"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/provider/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
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

// grpcChatStream wraps the gRPC streaming client into a plugins.ChatStream.
type grpcChatStream struct {
	stream proto.ProviderService_ChatClient
}

func (s *grpcChatStream) Recv() (*plugins.ChatChunk, error) {
	chunk, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	result := &plugins.ChatChunk{
		ID:    chunk.Id,
		Role:  chunk.Role,
		Delta: chunk.Delta,
		Done:  chunk.Done,
	}
	for _, tc := range chunk.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, plugins.ChatToolCall{
			ID:        tc.Id,
			Name:      tc.Name,
			Arguments: tc.Arguments.AsMap(),
		})
	}
	return result, nil
}

func (s *grpcChatStream) Close() error {
	return s.stream.CloseSend()
}

func (c *Client) Chat(ctx context.Context, messages []plugins.ChatMessage, tools []plugins.ToolCall, model *plugins.Model) (plugins.ChatStream, error) {
	req := &proto.ChatRequest{}
	if model != nil {
		req.Model = model.ModelName
		req.Temperature = model.Temperature
	}
	for _, m := range messages {
		pm := &proto.MessageProto{
			Role:    m.Role,
			Content: m.Content,
		}
		if m.ToolCalls != nil {
			for _, tc := range m.ToolCalls.ToolCalls {
				arguments, err := structpb.NewStruct(tc.Arguments)
				if err != nil {
					return nil, err
				}
				pm.ToolCalls = append(pm.ToolCalls, &proto.ToolCallProto{
					Id:        tc.ID,
					Name:      tc.Name,
					Arguments: arguments,
				})
			}
		}
		req.Messages = append(req.Messages, pm)
	}
	for _, t := range tools {
		params, err := structpb.NewStruct(t.Parameters)
		if err != nil {
			return nil, err
		}
		req.Tools = append(req.Tools, &proto.ToolDefProto{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}

	stream, err := c.client.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	return &grpcChatStream{stream: stream}, nil
}

func (c *Client) Embed(ctx context.Context, content string, model *plugins.Model) ([][]float32, error) {
	req := &proto.EmbedRequest{Content: content}
	if model != nil {
		req.Model = model.ModelName
	}
	resp, err := c.client.Embed(ctx, req)
	if err != nil {
		return nil, err
	}
	result := make([][]float32, len(resp.Embeddings))
	for i, e := range resp.Embeddings {
		result[i] = e.Values
	}
	return result, nil
}

func (c *Client) ListModels(ctx context.Context) ([]*plugins.Model, error) {
	resp, err := c.client.ListModels(ctx, &proto.ListModelsRequest{})
	if err != nil {
		return nil, err
	}
	models := make([]*plugins.Model, len(resp.Models))
	for i, m := range resp.Models {
		meta := make(map[string]any)
		for k, v := range m.Metadata {
			meta[k] = v
		}
		models[i] = &plugins.Model{
			ModelName: m.Name,
			Dimension: int(m.Dimension),
			Metadata:  meta,
		}
	}
	return models, nil
}

func (c *Client) CreateModel(ctx context.Context, modelName string, template *plugins.ModelTemplate) (*plugins.Model, error) {
	req := &proto.CreateModelRequest{Name: modelName}
	if template != nil {
		req.BaseModel = template.BaseModel
		req.PromptTemplate = template.PromptTemplate
		req.System = template.System
		req.Parameters = make(map[string]string)
		for k, v := range template.Parameters {
			req.Parameters[k] = fmt.Sprintf("%v", v)
		}
	}
	resp, err := c.client.CreateModel(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Model == nil {
		return nil, io.EOF
	}
	return &plugins.Model{ModelName: resp.Model.Name, Dimension: int(resp.Model.Dimension)}, nil
}

func (c *Client) GetModel(ctx context.Context, name string) (*plugins.Model, error) {
	resp, err := c.client.GetModel(ctx, &proto.GetModelRequest{Name: name})
	if err != nil {
		return nil, err
	}
	if resp.Model == nil {
		return nil, fmt.Errorf("model not found: %s", name)
	}
	return &plugins.Model{ModelName: resp.Model.Name, Dimension: int(resp.Model.Dimension)}, nil
}

func (c *Client) DeleteModel(ctx context.Context, name string) (bool, error) {
	resp, err := c.client.DeleteModel(ctx, &proto.DeleteModelRequest{Name: name})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}
