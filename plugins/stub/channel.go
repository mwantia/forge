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

func (p *StubChannelPlugin) Send(ctx context.Context, channel, content string, metadata map[string]any) (string, error) {
	return "stub-message-id", nil
}

func (p *StubChannelPlugin) Receive(ctx context.Context) (<-chan plugins.ChannelMessage, error) {
	ch := make(chan plugins.ChannelMessage, 1)
	go func() {
		ch <- plugins.ChannelMessage{
			ID:      "stub-event-id",
			Channel: "stub-channel-id",
			Author:  "stub-author-id",
			Content: "This is a stub message event.",
		}
		close(ch)
	}()
	return ch, nil
}
