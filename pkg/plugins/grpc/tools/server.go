package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

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

func (s *Server) getPlugin(ctx context.Context) (plugins.ToolsPlugin, error) {
	plugin, err := s.impl.GetToolsPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("tools plugin not available")
	}
	return plugin, nil
}

func (s *Server) ListTools(ctx context.Context, req *proto.ListToolsRequest) (*proto.ListToolsResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
	}

	filter := plugins.ListToolsFilter{
		Tags:       req.Tags,
		Deprecated: req.Deprecated,
		Prefix:     req.Prefix,
	}

	resp, err := plugin.ListTools(ctx, filter)
	if err != nil {
		return nil, err
	}

	protoResp := &proto.ListToolsResponse{}
	for _, t := range resp.Tools {
		p, err := toolDefinitionToProto(t)
		if err != nil {
			return nil, fmt.Errorf("failed to encode tool %q: %w", t.Name, err)
		}
		protoResp.Tools = append(protoResp.Tools, p)
	}
	return protoResp, nil
}

func (s *Server) GetTool(ctx context.Context, req *proto.GetToolRequest) (*proto.GetToolResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
	}

	def, err := plugin.GetTool(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	p, err := toolDefinitionToProto(*def)
	if err != nil {
		return nil, err
	}
	return &proto.GetToolResponse{Tool: p}, nil
}

func (s *Server) Execute(ctx context.Context, req *proto.ExecuteRequest) (*proto.ExecuteResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
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

	metadataStruct, err := toStruct(resp.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to encode metadata: %w", err)
	}

	return &proto.ExecuteResponse{
		Result:   resultValue,
		IsError:  resp.IsError,
		Metadata: metadataStruct,
	}, nil
}

func (s *Server) ExecuteStream(req *proto.ExecuteRequest, stream proto.ToolsService_ExecuteStreamServer) error {
	ctx := stream.Context()
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return err
	}

	var args map[string]any
	if req.Arguments != nil {
		args = req.Arguments.AsMap()
	}

	ch, err := plugin.ExecuteStream(ctx, plugins.ExecuteRequest{
		Tool:      req.Tool,
		Arguments: args,
		CallID:    req.CallId,
	})
	if err != nil {
		return err
	}

	for chunk := range ch {
		delta, err := toValue(chunk.Delta)
		if err != nil {
			return fmt.Errorf("failed to encode chunk delta: %w", err)
		}
		if err := stream.Send(&proto.ExecuteChunk{
			CallId:  chunk.CallID,
			Delta:   delta,
			Done:    chunk.Done,
			IsError: chunk.IsError,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Cancel(ctx context.Context, req *proto.CancelRequest) (*proto.CancelResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
	}

	if err := plugin.Cancel(ctx, req.CallId); err != nil {
		return &proto.CancelResponse{Ok: false}, nil
	}
	return &proto.CancelResponse{Ok: true}, nil
}

func (s *Server) Validate(ctx context.Context, req *proto.ValidateRequest) (*proto.ValidateResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
	}

	var args map[string]any
	if req.Arguments != nil {
		args = req.Arguments.AsMap()
	}

	resp, err := plugin.Validate(ctx, plugins.ExecuteRequest{
		Tool:      req.Tool,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	return &proto.ValidateResponse{
		Valid:  resp.Valid,
		Errors: resp.Errors,
	}, nil
}

// toolDefinitionToProto converts a plugins.ToolDefinition to *proto.ToolDefProto.
func toolDefinitionToProto(t plugins.ToolDefinition) (*proto.ToolDefinitionProto, error) {
	paramsStruct, err := toStruct(t.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to encode parameters: %w", err)
	}
	p := &proto.ToolDefinitionProto{
		Name:               t.Name,
		Description:        t.Description,
		Parameters:         paramsStruct,
		Tags:               t.Tags,
		Version:            t.Version,
		Deprecated:         t.Deprecated,
		DeprecationMessage: t.DeprecationMessage,
		Annotations: &proto.ToolAnnotations{
			ReadOnly:             t.Annotations.ReadOnly,
			Destructive:          t.Annotations.Destructive,
			Idempotent:           t.Annotations.Idempotent,
			RequiresConfirmation: t.Annotations.RequiresConfirmation,
			CostHint:             t.Annotations.CostHint,
		},
	}
	return p, nil
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

// isEOF returns true if err signals end-of-stream.
func isEOF(err error) bool {
	return err == io.EOF
}
