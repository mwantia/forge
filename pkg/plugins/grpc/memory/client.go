package memory

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/memory/proto"
	"google.golang.org/grpc"
)

// Client implements plugins.MemoryPlugin over gRPC.
type Client struct {
	client proto.MemoryServiceClient
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{client: proto.NewMemoryServiceClient(conn)}
}

func (c *Client) GetLifecycle() plugins.Lifecycle { return nil }

func (c *Client) Store(ctx context.Context, req plugins.StoreRequest) (*plugins.StoreResponse, error) {
	protoReq := &proto.StoreReq{
		Content:   req.Content,
		Namespace: req.Namespace,
		Metadata:  make(map[string]string),
	}
	for k, v := range req.Metadata {
		protoReq.Metadata[k] = fmt.Sprintf("%v", v)
	}

	resp, err := c.client.Store(ctx, protoReq)
	if err != nil {
		return nil, err
	}
	return &plugins.StoreResponse{ID: resp.Id}, nil
}

func (c *Client) Retrieve(ctx context.Context, req plugins.RetrieveRequest) (*plugins.RetrieveResponse, error) {
	protoReq := &proto.RetrieveReq{
		Query:     req.Query,
		Limit:     int32(req.Limit),
		Namespace: req.Namespace,
		Filter:    make(map[string]string),
	}
	for k, v := range req.Filter {
		protoReq.Filter[k] = fmt.Sprintf("%v", v)
	}

	resp, err := c.client.Retrieve(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	result := &plugins.RetrieveResponse{}
	for _, r := range resp.Results {
		metadata := make(map[string]any)
		for k, v := range r.Metadata {
			metadata[k] = v
		}
		result.Results = append(result.Results, plugins.MemoryResult{
			ID:       r.Id,
			Content:  r.Content,
			Score:    r.Score,
			Metadata: metadata,
		})
	}
	return result, nil
}
