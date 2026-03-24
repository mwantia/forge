package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/tools/proto"
	"google.golang.org/protobuf/types/known/structpb"
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
		paramsStruct, err := toStruct(t.Parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to encode parameters for tool %q: %w", t.Name, err)
		}
		protoResp.Tools = append(protoResp.Tools, &proto.ToolDefProto{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  paramsStruct,
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

	var args map[string]any
	if req.Arguments != nil {
		args = req.Arguments.AsMap()
	}

	resp, err := plugin.Execute(ctx, plugins.ExecuteRequest{
		Tool:      req.Tool,
		Arguments: args,
		CallID:    req.CallId,
	})
	if err != nil {
		return nil, err
	}

	resultValue, err := toValue(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to encode result: %w", err)
	}

	return &proto.ExecuteResp{
		Result:  resultValue,
		IsError: resp.IsError,
	}, nil
}

// toStruct converts a map[string]any to *structpb.Struct via a JSON round-trip so
// that native Go types like []string are normalised to JSON-compatible equivalents.
func toStruct(m map[string]any) (*structpb.Struct, error) {
	if len(m) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var normalized map[string]any
	if err := json.Unmarshal(b, &normalized); err != nil {
		return nil, err
	}
	return structpb.NewStruct(normalized)
}

// toValue converts any value to *structpb.Value via a JSON round-trip.
func toValue(v any) (*structpb.Value, error) {
	if v == nil {
		return structpb.NewNullValue(), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var normalized any
	if err := json.Unmarshal(b, &normalized); err != nil {
		return nil, err
	}
	return structpb.NewValue(normalized)
}
