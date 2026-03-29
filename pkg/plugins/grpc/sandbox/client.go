package sandbox

import (
	"context"
	"io"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/sandbox/proto"
	"google.golang.org/grpc"
)

// Client implements plugins.SandboxPlugin over gRPC.
type Client struct {
	client proto.SandboxServiceClient
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{client: proto.NewSandboxServiceClient(conn)}
}

func (c *Client) GetLifecycle() plugins.Lifecycle { return nil }

func (c *Client) CreateSandbox(ctx context.Context, spec plugins.SandboxSpec) (*plugins.SandboxHandle, error) {
	rules := make([]*proto.SandboxPathRule, 0, len(spec.AllowedHostPaths))
	for _, r := range spec.AllowedHostPaths {
		rules = append(rules, &proto.SandboxPathRule{
			Path:     r.Path,
			Writable: r.Writable,
		})
	}

	resp, err := c.client.CreateSandbox(ctx, &proto.CreateSandboxRequest{
		Spec: &proto.SandboxSpec{
			Name:             spec.Name,
			AllowedHostPaths: rules,
			WorkDir:          spec.WorkDir,
			Env:              spec.Env,
		},
	})
	if err != nil {
		return nil, err
	}

	return &plugins.SandboxHandle{ID: resp.Id, Driver: resp.Driver}, nil
}

func (c *Client) DestroySandbox(ctx context.Context, id string) error {
	_, err := c.client.DestroySandbox(ctx, &proto.DestroySandboxRequest{Id: id})
	return err
}

func (c *Client) CopyIn(ctx context.Context, id, hostSrc, sandboxDst string) error {
	_, err := c.client.CopyIn(ctx, &proto.CopyInRequest{
		Id:         id,
		HostSrc:    hostSrc,
		SandboxDst: sandboxDst,
	})
	return err
}

func (c *Client) CopyOut(ctx context.Context, id, sandboxSrc, hostDst string) error {
	_, err := c.client.CopyOut(ctx, &proto.CopyOutRequest{
		Id:         id,
		SandboxSrc: sandboxSrc,
		HostDst:    hostDst,
	})
	return err
}

func (c *Client) Execute(ctx context.Context, req plugins.SandboxExecRequest) (<-chan plugins.SandboxExecChunk, error) {
	stream, err := c.client.Execute(ctx, &proto.ExecuteRequest{
		SandboxId:      req.SandboxID,
		Command:        req.Command,
		Args:           req.Args,
		Env:            req.Env,
		TimeoutSeconds: int32(req.TimeoutSeconds),
	})
	if err != nil {
		return nil, err
	}

	ch := make(chan plugins.SandboxExecChunk)
	go func() {
		defer close(ch)
		for {
			chunk, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					ch <- plugins.SandboxExecChunk{IsError: true, Data: err.Error(), Done: true}
				}
				return
			}
			ch <- plugins.SandboxExecChunk{
				Stream:   chunk.Stream,
				Data:     chunk.Data,
				ExitCode: int(chunk.ExitCode),
				Done:     chunk.Done,
				IsError:  chunk.IsError,
			}
			if chunk.Done {
				return
			}
		}
	}()

	return ch, nil
}

func (c *Client) Stat(ctx context.Context, id, path string) (*plugins.SandboxStatResult, error) {
	resp, err := c.client.Stat(ctx, &proto.StatRequest{Id: id, Path: path})
	if err != nil {
		return nil, err
	}
	return &plugins.SandboxStatResult{
		Path:    resp.Path,
		Exists:  resp.Exists,
		IsDir:   resp.IsDir,
		Size:    resp.Size,
		Mode:    resp.Mode,
		ModTime: resp.ModTime,
	}, nil
}

func (c *Client) ReadFile(ctx context.Context, id, path string) ([]byte, error) {
	resp, err := c.client.ReadFile(ctx, &proto.ReadFileRequest{Id: id, Path: path})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}
