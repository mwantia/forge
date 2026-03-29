package sandbox

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/sandbox/proto"
)

// Server implements SandboxServiceServer, bridging gRPC calls to the SandboxPlugin interface.
type Server struct {
	proto.UnimplementedSandboxServiceServer
	impl plugins.Driver
}

func NewServer(impl plugins.Driver) *Server {
	return &Server{impl: impl}
}

func (s *Server) getPlugin(ctx context.Context) (plugins.SandboxPlugin, error) {
	plugin, err := s.impl.GetSandboxPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("sandbox plugin not available")
	}
	return plugin, nil
}

func (s *Server) CreateSandbox(ctx context.Context, req *proto.CreateSandboxRequest) (*proto.CreateSandboxResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
	}

	spec := plugins.SandboxSpec{}
	if req.Spec != nil {
		spec.Name = req.Spec.Name
		spec.WorkDir = req.Spec.WorkDir
		spec.Env = req.Spec.Env
		for _, r := range req.Spec.AllowedHostPaths {
			spec.AllowedHostPaths = append(spec.AllowedHostPaths, plugins.SandboxPathRule{
				Path:     r.Path,
				Writable: r.Writable,
			})
		}
	}

	handle, err := plugin.CreateSandbox(ctx, spec)
	if err != nil {
		return nil, err
	}

	return &proto.CreateSandboxResponse{Id: handle.ID, Driver: handle.Driver}, nil
}

func (s *Server) DestroySandbox(ctx context.Context, req *proto.DestroySandboxRequest) (*proto.DestroySandboxResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if err := plugin.DestroySandbox(ctx, req.Id); err != nil {
		return nil, err
	}
	return &proto.DestroySandboxResponse{}, nil
}

func (s *Server) CopyIn(ctx context.Context, req *proto.CopyInRequest) (*proto.CopyInResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if err := plugin.CopyIn(ctx, req.Id, req.HostSrc, req.SandboxDst); err != nil {
		return nil, err
	}
	return &proto.CopyInResponse{}, nil
}

func (s *Server) CopyOut(ctx context.Context, req *proto.CopyOutRequest) (*proto.CopyOutResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if err := plugin.CopyOut(ctx, req.Id, req.SandboxSrc, req.HostDst); err != nil {
		return nil, err
	}
	return &proto.CopyOutResponse{}, nil
}

func (s *Server) Execute(req *proto.ExecuteRequest, stream proto.SandboxService_ExecuteServer) error {
	ctx := stream.Context()
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return err
	}

	ch, err := plugin.Execute(ctx, plugins.SandboxExecRequest{
		SandboxID:      req.SandboxId,
		Command:        req.Command,
		Args:           req.Args,
		Env:            req.Env,
		TimeoutSeconds: int(req.TimeoutSeconds),
	})
	if err != nil {
		return err
	}

	for chunk := range ch {
		if err := stream.Send(&proto.ExecuteChunk{
			Stream:   chunk.Stream,
			Data:     chunk.Data,
			ExitCode: int32(chunk.ExitCode),
			Done:     chunk.Done,
			IsError:  chunk.IsError,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Stat(ctx context.Context, req *proto.StatRequest) (*proto.StatResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
	}
	result, err := plugin.Stat(ctx, req.Id, req.Path)
	if err != nil {
		return nil, err
	}
	return &proto.StatResponse{
		Path:    result.Path,
		Exists:  result.Exists,
		IsDir:   result.IsDir,
		Size:    result.Size,
		Mode:    result.Mode,
		ModTime: result.ModTime,
	}, nil
}

func (s *Server) ReadFile(ctx context.Context, req *proto.ReadFileRequest) (*proto.ReadFileResponse, error) {
	plugin, err := s.getPlugin(ctx)
	if err != nil {
		return nil, err
	}
	data, err := plugin.ReadFile(ctx, req.Id, req.Path)
	if err != nil {
		return nil, err
	}
	return &proto.ReadFileResponse{Data: data}, nil
}
