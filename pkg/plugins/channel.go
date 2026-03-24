package plugins

import (
	"context"

	"github.com/mwantia/forge/pkg/errors"
)

// ChannelPlugin acts as communication gateway for endpoints like Discord.
type ChannelPlugin interface {
	BasePlugin
	// Additional channel methods will be added here
	Send(ctx context.Context, channel, content string, metadata map[string]any) (string, error)
	Receive(ctx context.Context) (<-chan ChannelMessage, error)
}

type ChannelMessage struct {
	ID       string         `json:"id"`
	Channel  string         `json:"channel"`
	Author   string         `json:"author"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// UnimplementedChannelPlugin can be embedded to satisfy ChannelPlugin with default
// implementations that return errors.ErrPluginCapabilityNotSupported.
type UnimplementedChannelPlugin struct{}

func (UnimplementedChannelPlugin) GetLifecycle() Lifecycle { return nil }

func (UnimplementedChannelPlugin) Send(_ context.Context, _, _ string, _ map[string]any) (string, error) {
	return "", errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedChannelPlugin) Receive(_ context.Context) (<-chan ChannelMessage, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}
