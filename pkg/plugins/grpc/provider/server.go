package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

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

func (s *Server) Chat(req *proto.ChatReq, stream proto.ProviderService_ChatServer) error {
	ctx := stream.Context()

	plugin, err := s.impl.GetProviderPlugin(ctx)
	if err != nil {
		return err
	}
	if plugin == nil {
		return fmt.Errorf("provider plugin not available")
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

	chatStream, err := plugin.Chat(ctx, messages, tools, model)
	if err != nil {
		return err
	}
	defer chatStream.Close()

	for {
		chunk, err := chatStream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		protoChunk := &proto.ChatChunk{
			Id:    chunk.ID,
			Role:  chunk.Role,
			Delta: chunk.Delta,
			Done:  chunk.Done,
		}
		for _, tc := range chunk.ToolCalls {
			args := make(map[string]string)
			for k, v := range tc.Arguments {
				b, marshalErr := json.Marshal(v)
				if marshalErr != nil {
					args[k] = fmt.Sprintf("%v", v)
				} else {
					args[k] = string(b)
				}
			}
			protoChunk.ToolCalls = append(protoChunk.ToolCalls, &proto.ToolCallProto{
				Id:        tc.ID,
				Name:      tc.Name,
				Arguments: args,
			})
		}

		if err := stream.Send(protoChunk); err != nil {
			return err
		}
	}
}

func (s *Server) Embed(ctx context.Context, req *proto.EmbedReq) (*proto.EmbedResp, error) {
	plugin, err := s.impl.GetProviderPlugin(ctx)
	if err != nil {
		return nil, err
	}

	model := &plugins.Model{ModelName: req.Model}
	vectors, err := plugin.Embed(ctx, req.Content, model)
	if err != nil {
		return nil, err
	}

	resp := &proto.EmbedResp{}
	for _, vec := range vectors {
		resp.Embeddings = append(resp.Embeddings, &proto.EmbeddingProto{Values: vec})
	}
	return resp, nil
}

func (s *Server) ListModels(ctx context.Context, _ *proto.ListModelsReq) (*proto.ListModelsResp, error) {
	plugin, err := s.impl.GetProviderPlugin(ctx)
	if err != nil {
		return nil, err
	}

	models, err := plugin.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	resp := &proto.ListModelsResp{}
	for _, m := range models {
		pm := &proto.ModelProto{
			Name:      m.ModelName,
			Dimension: int32(m.Dimension),
			Metadata:  make(map[string]string),
		}
		for k, v := range m.Metadata {
			pm.Metadata[k] = fmt.Sprintf("%v", v)
		}
		resp.Models = append(resp.Models, pm)
	}
	return resp, nil
}

func (s *Server) CreateModel(ctx context.Context, req *proto.CreateModelReq) (*proto.CreateModelResp, error) {
	plugin, err := s.impl.GetProviderPlugin(ctx)
	if err != nil {
		return nil, err
	}

	params := make(map[string]any)
	for k, v := range req.Parameters {
		params[k] = v
	}
	template := &plugins.ModelTemplate{
		BaseModel:      req.BaseModel,
		PromptTemplate: req.PromptTemplate,
		System:         req.System,
		Parameters:     params,
	}

	model, err := plugin.CreateModel(ctx, req.Name, template)
	if err != nil {
		return nil, err
	}

	return &proto.CreateModelResp{
		Model: &proto.ModelProto{Name: model.ModelName, Dimension: int32(model.Dimension)},
	}, nil
}

func (s *Server) GetModel(ctx context.Context, req *proto.GetModelReq) (*proto.GetModelResp, error) {
	plugin, err := s.impl.GetProviderPlugin(ctx)
	if err != nil {
		return nil, err
	}

	model, err := plugin.GetModel(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return &proto.GetModelResp{
		Model: &proto.ModelProto{Name: model.ModelName, Dimension: int32(model.Dimension)},
	}, nil
}

func (s *Server) DeleteModel(ctx context.Context, req *proto.DeleteModelReq) (*proto.DeleteModelResp, error) {
	plugin, err := s.impl.GetProviderPlugin(ctx)
	if err != nil {
		return nil, err
	}

	ok, err := plugin.DeleteModel(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return &proto.DeleteModelResp{Success: ok}, nil
}
