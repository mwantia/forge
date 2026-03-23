package provider

import (
	"context"
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

	goReq := plugins.GenerateRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   int(req.MaxTokens),
	}
	for _, m := range req.Messages {
		goReq.Messages = append(goReq.Messages, plugins.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	for _, t := range req.Tools {
		params := make(map[string]interface{})
		for k, v := range t.Parameters {
			params[k] = v
		}
		goReq.Tools = append(goReq.Tools, plugins.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}

	resp, err := plugin.Generate(ctx, goReq)
	if err != nil {
		return nil, err
	}

	protoResp := &proto.GenerateResp{
		Id:      resp.ID,
		Content: resp.Content,
		Role:    resp.Role,
		Model:   resp.Model,
	}
	if resp.Usage != nil {
		protoResp.Usage = &proto.UsageProto{
			InputTokens:  int32(resp.Usage.InputTokens),
			OutputTokens: int32(resp.Usage.OutputTokens),
		}
	}
	return protoResp, nil
}
