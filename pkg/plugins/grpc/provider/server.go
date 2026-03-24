package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/provider/proto"
)

// Server implements ProviderServiceServer, bridging gRPC to the ProviderPlugin interface.
type Server struct {
	proto.UnimplementedProviderServiceServer
	impl plugins.Driver
}

func NewServer(impl plugins.Driver) *Server {
	return &Server{impl: impl}
}

func (s *Server) Generate(ctx context.Context, req *proto.GenerateReq) (*proto.GenerateResp, error) {
	plugin, err := s.impl.GetProviderPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("provider plugin not available")
	}

	var messages []plugins.ChatMessage
	for _, m := range req.Messages {
		messages = append(messages, plugins.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	var tools []plugins.ToolCall
	for _, t := range req.Tools {
		params := make(map[string]any)
		for k, v := range t.Parameters {
			var decoded any
			if err := json.Unmarshal([]byte(v), &decoded); err != nil {
				params[k] = v
			} else {
				params[k] = decoded
			}
		}
		tools = append(tools, plugins.ToolCall{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}

	model := &plugins.Model{
		ModelName:   req.Model,
		Temperature: req.Temperature,
	}

	result, err := plugin.Chat(ctx, messages, tools, model)
	if err != nil {
		return nil, err
	}

	return &proto.GenerateResp{
		Id:      result.ID,
		Role:    result.Role,
		Content: result.Content,
		Model:   req.Model,
	}, nil
}
