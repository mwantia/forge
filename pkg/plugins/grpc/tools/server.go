package tools

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/tools/proto"
)

// Server implements ToolsServiceServer, bridging gRPC to the ToolsPlugin interface.
type Server struct {
	proto.UnimplementedToolsServiceServer
	impl plugins.Driver
}

func NewServer(impl plugins.Driver) *Server {
	return &Server{impl: impl}
}

func (s *Server) List(ctx context.Context, req *proto.ListToolsReq) (*proto.ListToolsResp, error) {
	plugin, err := s.impl.GetToolsPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("tools plugin not available")
	}

	resp, err := plugin.List(ctx)
	if err != nil {
		return nil, err
	}

	protoResp := &proto.ListToolsResp{}
	for _, t := range resp.Tools {
		params := make(map[string]string)
		for k, v := range t.Parameters {
			params[k] = fmt.Sprintf("%v", v)
		}
		protoResp.Tools = append(protoResp.Tools, &proto.ToolDefProto{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return protoResp, nil
}

func (s *Server) Execute(ctx context.Context, req *proto.ExecuteReq) (*proto.ExecuteResp, error) {
	plugin, err := s.impl.GetToolsPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("tools plugin not available")
	}

	args := make(map[string]any)
	for k, v := range req.Arguments {
		args[k] = v
	}

	resp, err := plugin.Execute(ctx, plugins.ExecuteRequest{
		Tool:      req.Tool,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	if resultMap, ok := resp.Result.(map[string]any); ok {
		for k, v := range resultMap {
			result[k] = fmt.Sprintf("%v", v)
		}
	}

	return &proto.ExecuteResp{
		Result:  result,
		IsError: resp.IsError,
	}, nil
}
