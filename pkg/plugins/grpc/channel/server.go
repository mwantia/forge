package channel

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/channel/proto"
)

// Server implements ChannelServiceServer, bridging gRPC to the ChannelPlugin interface.
type Server struct {
	proto.UnimplementedChannelServiceServer
	impl plugins.Driver
}

func NewServer(impl plugins.Driver) *Server {
	return &Server{impl: impl}
}

func (s *Server) Send(ctx context.Context, req *proto.SendRequest) (*proto.SendResponse, error) {
	plugin, err := s.impl.GetChannelPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("channel plugin not available")
	}

	metadata := make(map[string]any)
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	id, err := plugin.Send(ctx, req.ChannelId, req.Content, metadata)
	if err != nil {
		return nil, err
	}
	return &proto.SendResponse{
		MessageId: id,
	}, nil
}

func (s *Server) Receive(req *proto.ReceiveRequest, srv proto.ChannelService_ReceiveServer) error {
	plugin, err := s.impl.GetChannelPlugin(srv.Context())
	if err != nil {
		return err
	}
	if plugin == nil {
		return fmt.Errorf("channel plugin not available")
	}

	ch, err := plugin.Receive(srv.Context())
	if err != nil {
		return err
	}

	for evt := range ch {
		metadata := make(map[string]string)
		for k, v := range evt.Metadata {
			metadata[k] = fmt.Sprintf("%v", v)
		}
		if err := srv.Send(&proto.MessageEvent{
			Id:        evt.ID,
			ChannelId: evt.Channel,
			AuthorId:  evt.Author,
			Content:   evt.Content,
			Metadata:  metadata,
		}); err != nil {
			return err
		}
	}
	return nil
}
