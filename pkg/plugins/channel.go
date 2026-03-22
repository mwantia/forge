package plugins

import "context"

// ChannelPlugin acts as communication gateway for endpoints like Discord.
type ChannelPlugin interface {
	BasePlugin
	// Additional channel methods will be added here
	Send(ctx context.Context, req SendRequest) (*SendResponse, error)
	Receive(ctx context.Context) (<-chan MessageEvent, error)
}

type SendRequest struct {
	ChannelID string         `json:"channel_id"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type SendResponse struct {
	MessageID string `json:"message_id"`
}

type MessageEvent struct {
	ID        string         `json:"id"`
	ChannelID string         `json:"channel_id"`
	AuthorID  string         `json:"author_id"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}