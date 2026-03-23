package driver

import (
	"context"
	"fmt"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mwantia/forge/pkg/plugins"
	channelgrpc "github.com/mwantia/forge/pkg/plugins/grpc/channel"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/driver/proto"
	memorygrpc "github.com/mwantia/forge/pkg/plugins/grpc/memory"
	providergrpc "github.com/mwantia/forge/pkg/plugins/grpc/provider"
	toolsgrpc "github.com/mwantia/forge/pkg/plugins/grpc/tools"
	"google.golang.org/grpc"
)

// Client implements plugins.Driver over gRPC.
type Client struct {
	client proto.DriverServiceClient
	broker *goplugin.GRPCBroker
	conn   *grpc.ClientConn
}

func NewClient(conn *grpc.ClientConn, broker *goplugin.GRPCBroker) *Client {
	return &Client{
		client: proto.NewDriverServiceClient(conn),
		broker: broker,
		conn:   conn,
	}
}

func (c *Client) GetPluginInfo() plugins.PluginInfo {
	resp, err := c.client.GetPluginInfo(context.Background(), &proto.GetPluginInfoRequest{})
	if err != nil {
		return plugins.PluginInfo{}
	}
	return plugins.PluginInfo{
		Name:    resp.Info.Name,
		Author:  resp.Info.Author,
		Version: resp.Info.Version,
	}
}

func (c *Client) ProbePlugin(ctx context.Context) (bool, error) {
	resp, err := c.client.ProbePlugin(ctx, &proto.ProbeRequest{})
	if err != nil {
		return false, err
	}
	return resp.Ok, nil
}

func (c *Client) GetCapabilities(ctx context.Context) (*plugins.DriverCapabilities, error) {
	resp, err := c.client.GetCapabilities(ctx, &proto.CapabilitiesRequest{})
	if err != nil {
		return nil, err
	}
	return capsFromProto(resp.Capabilities), nil
}

func (c *Client) OpenDriver(ctx context.Context) error {
	_, err := c.client.OpenDriver(ctx, &proto.OpenRequest{})
	return err
}

func (c *Client) CloseDriver(ctx context.Context) error {
	_, err := c.client.CloseDriver(ctx, &proto.CloseRequest{})
	return err
}

func (c *Client) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
	req := &proto.ConfigRequest{Config: make(map[string]string)}
	for k, v := range config.ConfigMap {
		switch val := v.(type) {
		case string:
			req.Config[k] = val
		case bool:
			if val {
				req.Config[k] = "true"
			} else {
				req.Config[k] = "false"
			}
		case int, int64:
			req.Config[k] = fmt.Sprintf("%d", val)
		case float64:
			req.Config[k] = fmt.Sprintf("%f", val)
		default:
			req.Config[k] = fmt.Sprintf("%v", val)
		}
	}
	_, err := c.client.ConfigDriver(ctx, req)
	return err
}

func (c *Client) GetProviderPlugin(ctx context.Context) (plugins.ProviderPlugin, error) {
	resp, err := c.client.GetProviderPlugin(ctx, &proto.GetPluginRequest{})
	if err != nil {
		return nil, err
	}
	if !resp.Available {
		return nil, nil
	}
	return providergrpc.NewClient(c.conn), nil
}

func (c *Client) GetMemoryPlugin(ctx context.Context) (plugins.MemoryPlugin, error) {
	resp, err := c.client.GetMemoryPlugin(ctx, &proto.GetPluginRequest{})
	if err != nil {
		return nil, err
	}
	if !resp.Available {
		return nil, nil
	}
	return memorygrpc.NewClient(c.conn), nil
}

func (c *Client) GetChannelPlugin(ctx context.Context) (plugins.ChannelPlugin, error) {
	resp, err := c.client.GetChannelPlugin(ctx, &proto.GetPluginRequest{})
	if err != nil {
		return nil, err
	}
	if !resp.Available {
		return nil, nil
	}
	return channelgrpc.NewClient(c.conn), nil
}

func (c *Client) GetToolsPlugin(ctx context.Context) (plugins.ToolsPlugin, error) {
	resp, err := c.client.GetToolsPlugin(ctx, &proto.GetPluginRequest{})
	if err != nil {
		return nil, err
	}
	if !resp.Available {
		return nil, nil
	}
	return toolsgrpc.NewClient(c.conn), nil
}
