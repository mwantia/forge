package memory

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/memory/proto"
)

// Server implements MemoryServiceServer, bridging gRPC to the MemoryPlugin interface.
type Server struct {
	proto.UnimplementedMemoryServiceServer
	impl plugins.Driver
}

func NewServer(impl plugins.Driver) *Server {
	return &Server{impl: impl}
}

func (s *Server) Store(ctx context.Context, req *proto.StoreReq) (*proto.StoreResp, error) {
	plugin, err := s.impl.GetMemoryPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("memory plugin not available")
	}

	metadata := make(map[string]any)
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	resource, err := plugin.StoreResource(ctx, req.Namespace, req.Content, metadata)
	if err != nil {
		return nil, err
	}
	return &proto.StoreResp{Id: resource.ID}, nil
}

func (s *Server) Retrieve(ctx context.Context, req *proto.RetrieveReq) (*proto.RetrieveResp, error) {
	plugin, err := s.impl.GetMemoryPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("memory plugin not available")
	}

	filter := make(map[string]any)
	for k, v := range req.Filter {
		filter[k] = v
	}

	resources, err := plugin.RetrieveResource(ctx, req.Namespace, req.Query, int(req.Limit), filter)
	if err != nil {
		return nil, err
	}

	protoResp := &proto.RetrieveResp{}
	for _, r := range resources {
		metadata := make(map[string]string)
		for k, v := range r.Metadata {
			metadata[k] = fmt.Sprintf("%v", v)
		}
		protoResp.Results = append(protoResp.Results, &proto.MemoryResultProto{
			Id:       r.ID,
			Content:  r.Content,
			Score:    r.Score,
			Metadata: metadata,
		})
	}
	return protoResp, nil
}
