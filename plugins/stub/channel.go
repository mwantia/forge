package stub

import (
	"context"

	"github.com/mwantia/forge/pkg/plugins"
)

// StubChannelPlugin implements ChannelPlugin.
type StubChannelPlugin struct {
	driver *StubDriver
}

func (p *StubChannelPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *StubChannelPlugin) GetPluginInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Type:    plugins.PluginTypeChannel,
		Name:    "stub-channel",
		Author:  "forge",
		Version: "0.1.0",
	}
}

func (p *StubChannelPlugin) Send(ctx context.Context, req plugins.SendRequest) (*plugins.SendResponse, error) {
	return &plugins.SendResponse{
		MessageID: "stub-message-id",
	}, nil
}

func (p *StubChannelPlugin) Receive(ctx context.Context) (<-chan plugins.MessageEvent, error) {
	ch := make(chan plugins.MessageEvent, 1)
	go func() {
		ch <- plugins.MessageEvent{
			ID:        "stub-event-id",
			ChannelID: "stub-channel-id",
			AuthorID:  "stub-author-id",
			Content:   "This is a stub message event.",
		}
		close(ch)
	}()
	return ch, nil
}
