package memory

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/memory/proto"
	"google.golang.org/grpc"
)

// Client implements plugins.MemoryPlugin over gRPC.
// Unimplemented capabilities (sessions, AddMessage) fall back to UnimplementedMemoryPlugin.
type Client struct {
	plugins.UnimplementedMemoryPlugin
	client proto.MemoryServiceClient
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{client: proto.NewMemoryServiceClient(conn)}
}

func (c *Client) StoreResource(ctx context.Context, sessionID, content string, metadata map[string]any) (*plugins.MemoryResource, error) {
	protoReq := &proto.StoreReq{
		Content:   content,
		Namespace: sessionID,
		Metadata:  make(map[string]string),
	}
	for k, v := range metadata {
		protoReq.Metadata[k] = fmt.Sprintf("%v", v)
	}

	resp, err := c.client.Store(ctx, protoReq)
	if err != nil {
		return nil, err
	}
	return &plugins.MemoryResource{ID: resp.Id, Content: content}, nil
}

func (c *Client) RetrieveResource(ctx context.Context, sessionID, query string, limit int, filter map[string]any) ([]*plugins.MemoryResource, error) {
	protoReq := &proto.RetrieveReq{
		Query:     query,
		Limit:     int32(limit),
		Namespace: sessionID,
		Filter:    make(map[string]string),
	}
	for k, v := range filter {
		protoReq.Filter[k] = fmt.Sprintf("%v", v)
	}

	resp, err := c.client.Retrieve(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	var results []*plugins.MemoryResource
	for _, r := range resp.Results {
		metadata := make(map[string]any)
		for k, v := range r.Metadata {
			metadata[k] = v
		}
		results = append(results, &plugins.MemoryResource{
			ID:       r.Id,
			Content:  r.Content,
			Score:    r.Score,
			Metadata: metadata,
		})
	}
	return results, nil
}
