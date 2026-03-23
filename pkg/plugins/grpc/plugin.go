package grpc

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mwantia/forge/pkg/plugins"
	channelgrpc "github.com/mwantia/forge/pkg/plugins/grpc/channel"
	drivergrpc "github.com/mwantia/forge/pkg/plugins/grpc/driver"
	driverproto "github.com/mwantia/forge/pkg/plugins/grpc/driver/proto"
	memorygrpc "github.com/mwantia/forge/pkg/plugins/grpc/memory"
	providergrpc "github.com/mwantia/forge/pkg/plugins/grpc/provider"
	providerproto "github.com/mwantia/forge/pkg/plugins/grpc/provider/proto"
	memoryproto "github.com/mwantia/forge/pkg/plugins/grpc/memory/proto"
	channelproto "github.com/mwantia/forge/pkg/plugins/grpc/channel/proto"
	toolsgrpc "github.com/mwantia/forge/pkg/plugins/grpc/tools"
	toolsproto "github.com/mwantia/forge/pkg/plugins/grpc/tools/proto"
	"google.golang.org/grpc"
)

// Handshake is the plugin handshake configuration.
// Plugins and hosts must use the same values to communicate.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  2,
	MagicCookieKey:   "FORGE_PLUGIN",
	MagicCookieValue: "forge",
}

// Plugins is the map of supported plugin types for use with go-plugin.
var Plugins = map[string]goplugin.Plugin{
	"driver": &DriverPlugin{},
}

// DriverPlugin is the hashicorp/go-plugin wrapper for the Driver interface.
type DriverPlugin struct {
	goplugin.Plugin
	Impl plugins.Driver
}

func (p *DriverPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	driverproto.RegisterDriverServiceServer(s, drivergrpc.NewServer(p.Impl, broker))
	providerproto.RegisterProviderServiceServer(s, providergrpc.NewServer(p.Impl))
	memoryproto.RegisterMemoryServiceServer(s, memorygrpc.NewServer(p.Impl))
	channelproto.RegisterChannelServiceServer(s, channelgrpc.NewServer(p.Impl))
	toolsproto.RegisterToolsServiceServer(s, toolsgrpc.NewServer(p.Impl))
	return nil
}

func (p *DriverPlugin) GRPCClient(ctx context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return drivergrpc.NewClient(c, broker), nil
}
