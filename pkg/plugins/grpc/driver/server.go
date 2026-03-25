package driver

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/driver/proto"
)

// Server implements DriverServiceServer, bridging gRPC calls to the Driver interface.
type Server struct {
	proto.UnimplementedDriverServiceServer
	impl   plugins.Driver
	broker *goplugin.GRPCBroker
}

func NewServer(impl plugins.Driver, broker *goplugin.GRPCBroker) *Server {
	return &Server{impl: impl, broker: broker}
}

func (s *Server) GetPluginInfo(ctx context.Context, req *proto.GetPluginInfoRequest) (*proto.GetPluginInfoResponse, error) {
	info := s.impl.GetPluginInfo()
	return &proto.GetPluginInfoResponse{Info: &proto.PluginInfo{
		Name:    info.Name,
		Author:  info.Author,
		Version: info.Version,
	}}, nil
}

func (s *Server) ProbePlugin(ctx context.Context, req *proto.ProbeRequest) (*proto.ProbeResponse, error) {
	ok, err := s.impl.ProbePlugin(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.ProbeResponse{Ok: ok}, nil
}

func (s *Server) GetCapabilities(ctx context.Context, req *proto.CapabilitiesRequest) (*proto.CapabilitiesResponse, error) {
	caps, err := s.impl.GetCapabilities(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.CapabilitiesResponse{Capabilities: capsToProto(caps)}, nil
}

func (s *Server) OpenDriver(ctx context.Context, req *proto.OpenRequest) (*proto.OpenResponse, error) {
	if err := s.impl.OpenDriver(ctx); err != nil {
		return nil, err
	}
	return &proto.OpenResponse{}, nil
}

func (s *Server) CloseDriver(ctx context.Context, req *proto.CloseRequest) (*proto.CloseResponse, error) {
	if err := s.impl.CloseDriver(ctx); err != nil {
		return nil, err
	}
	return &proto.CloseResponse{}, nil
}

func (s *Server) ConfigDriver(ctx context.Context, req *proto.ConfigRequest) (*proto.ConfigResponse, error) {
	config := plugins.PluginConfig{ConfigMap: req.Config.AsMap()}
	if err := s.impl.ConfigDriver(ctx, config); err != nil {
		return nil, err
	}
	return &proto.ConfigResponse{}, nil
}

func (s *Server) GetProviderPlugin(ctx context.Context, req *proto.GetPluginRequest) (*proto.GetPluginResponse, error) {
	p, err := s.impl.GetProviderPlugin(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.GetPluginResponse{Available: p != nil}, nil
}

func (s *Server) GetMemoryPlugin(ctx context.Context, req *proto.GetPluginRequest) (*proto.GetPluginResponse, error) {
	p, err := s.impl.GetMemoryPlugin(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.GetPluginResponse{Available: p != nil}, nil
}

func (s *Server) GetChannelPlugin(ctx context.Context, req *proto.GetPluginRequest) (*proto.GetPluginResponse, error) {
	p, err := s.impl.GetChannelPlugin(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.GetPluginResponse{Available: p != nil}, nil
}

func (s *Server) GetToolsPlugin(ctx context.Context, req *proto.GetPluginRequest) (*proto.GetPluginResponse, error) {
	p, err := s.impl.GetToolsPlugin(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.GetPluginResponse{Available: p != nil}, nil
}

// capsToProto converts a Go DriverCapabilities to its proto representation.
func capsToProto(caps *plugins.DriverCapabilities) *proto.DriverCapabilities {
	if caps == nil {
		return nil
	}
	p := &proto.DriverCapabilities{Types: caps.Types}
	if caps.Provider != nil {
		p.Provider = &proto.ProviderCapabilities{
			SupportsStreaming: caps.Provider.SupportsStreaming,
			SupportsVision:    caps.Provider.SupportsVision,
		}
	}
	if caps.Memory != nil {
		p.Memory = &proto.MemoryCapabilities{
			SupportsVectorSearch: caps.Memory.SupportsVectorSearch,
			MaxContextSize:       int32(caps.Memory.MaxContextSize),
		}
	}
	if caps.Channel != nil {
		p.Channel = &proto.ChannelCapabilities{
			SupportsDirectMessages: caps.Channel.SupportsDirectMessages,
			SupportsThreads:        caps.Channel.SupportsThreads,
		}
	}
	if caps.Tools != nil {
		p.Tools = &proto.ToolsCapabilities{
			SupportsAsyncExecution: caps.Tools.SupportsAsyncExecution,
		}
	}
	return p
}

// capsFromProto converts a proto DriverCapabilities to the Go type.
func capsFromProto(p *proto.DriverCapabilities) *plugins.DriverCapabilities {
	if p == nil {
		return nil
	}
	caps := &plugins.DriverCapabilities{Types: p.Types}
	if p.Provider != nil {
		caps.Provider = &plugins.ProviderCapabilities{
			SupportsStreaming: p.Provider.SupportsStreaming,
			SupportsVision:    p.Provider.SupportsVision,
		}
	}
	if p.Memory != nil {
		caps.Memory = &plugins.MemoryCapabilities{
			SupportsVectorSearch: p.Memory.SupportsVectorSearch,
			MaxContextSize:       int(p.Memory.MaxContextSize),
		}
	}
	if p.Channel != nil {
		caps.Channel = &plugins.ChannelCapabilities{
			SupportsDirectMessages: p.Channel.SupportsDirectMessages,
			SupportsThreads:        p.Channel.SupportsThreads,
		}
	}
	if p.Tools != nil {
		caps.Tools = &plugins.ToolsCapabilities{
			SupportsAsyncExecution: p.Tools.SupportsAsyncExecution,
		}
	}
	return caps
}
