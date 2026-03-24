package channel

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/channel/proto"
	"google.golang.org/grpc"
)

// Client implements plugins.ChannelPlugin over gRPC.
type Client struct {
	client proto.ChannelServiceClient
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{client: proto.NewChannelServiceClient(conn)}
}

func (c *Client) GetLifecycle() plugins.Lifecycle { return nil }

func (c *Client) Send(ctx context.Context, channel, content string, metadata map[string]any) (string, error) {
	req := &proto.SendReq{
		ChannelId: channel,
		Content:   content,
		Metadata:  make(map[string]string),
	}
	for k, v := range req.Metadata {
		req.Metadata[k] = fmt.Sprintf("%v", v)
	}
	resp, err := c.client.Send(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.MessageId, nil
}

func (c *Client) Receive(ctx context.Context) (<-chan plugins.ChannelMessage, error) {
	stream, err := c.client.Receive(ctx, &proto.ReceiveReq{})
	if err != nil {
		return nil, err
	}

	ch := make(chan plugins.ChannelMessage, 1)
	go func() {
		defer close(ch)
		for {
			evt, err := stream.Recv()
			if err != nil {
				return
			}
			metadata := make(map[string]any)
			for k, v := range evt.Metadata {
				metadata[k] = v
			}
			ch <- plugins.ChannelMessage{
				ID:       evt.Id,
				Channel:  evt.ChannelId,
				Author:   evt.AuthorId,
				Content:  evt.Content,
				Metadata: metadata,
			}
		}
	}()
	return ch, nil
}
