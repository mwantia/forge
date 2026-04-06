package channel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/internal/session"
	"github.com/mwantia/forge/internal/storage"
)

// ChannelDispatcher is a Runner that subscribes to every loaded channel plugin
// and routes inbound messages into Forge sessions. Slash-command messages are
// handled directly (create / select / list / info / clear) while regular messages
// are dispatched through the session pipeline and the response sent back.
type ChannelDispatcher struct {
	log      hclog.Logger             `fabric:"logger:channel-dispatcher"`
	registry *registry.PluginRegistry `fabric:"inject"`
	sessions *session.SessionManager  `fabric:"inject"`
	backend  storage.Backend          `fabric:"inject"`
	stores   map[string]*BindingStore // plugin name → binding store (set in Setup)
}

// Setup initialises a binding store for every loaded channel plugin so the
// HTTP API can access bindings immediately (before Serve runs).
func (d *ChannelDispatcher) Setup() (func() error, error) {
	for name := range d.registry.GetAllChannelPlugins(context.Background()) {
		store := newBindingStore(d.backend, name)
		if err := store.Load(); err != nil {
			d.log.Warn("Failed to load channel bindings; starting fresh", "plugin", name, "error", err)
		}
		d.stores[strings.ToLower(name)] = store
	}
	return func() error { return nil }, nil
}

// Serve subscribes to all loaded channel plugins and routes messages until ctx is cancelled.
func (d *ChannelDispatcher) Serve(ctx context.Context) error {
	channelPlugins := d.registry.GetAllChannelPlugins(ctx)
	if len(channelPlugins) == 0 {
		d.log.Info("No channel plugins loaded; dispatcher idle")
		<-ctx.Done()
		return nil
	}

	for name, plugin := range channelPlugins {
		store := d.stores[strings.ToLower(name)]
		go d.consumePlugin(ctx, name, plugin, store)
		d.log.Info("Channel dispatcher started", "plugin", name)
	}

	<-ctx.Done()
	return nil
}

// --- Public binding API (used by HTTP handlers) ---

func (d *ChannelDispatcher) GetStore(plugin string) (*BindingStore, bool) {
	s, ok := d.stores[strings.ToLower(plugin)]
	return s, ok
}

func (d *ChannelDispatcher) ListPlugins() []string {
	names := make([]string, 0, len(d.stores))
	for name := range d.stores {
		names = append(names, name)
	}
	return names
}

// --- Internal ---

func (d *ChannelDispatcher) consumePlugin(ctx context.Context, pluginName string, plugin plugins.ChannelPlugin, store *BindingStore) {
	log := d.log.With("plugin", pluginName)

	ch, err := plugin.Receive(ctx)
	if err != nil {
		log.Error("Failed to start Receive stream", "error", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				log.Info("Receive channel closed")
				return
			}
			d.handleMessage(ctx, log, plugin, store, msg)
		}
	}
}

func (d *ChannelDispatcher) handleMessage(ctx context.Context, log hclog.Logger, plugin plugins.ChannelPlugin, store *BindingStore, msg plugins.ChannelMessage) {
	switch cmd, _ := metaStr(msg.Metadata, "cmd"); cmd {
	case "session_create":
		d.handleSessionCreate(ctx, log, plugin, store, msg)
	case "session_select":
		d.handleSessionSelect(ctx, log, plugin, store, msg)
	case "session_list":
		d.handleSessionList(ctx, log, plugin, msg)
	case "session_info":
		d.handleSessionInfo(ctx, log, plugin, store, msg)
	case "session_clear":
		d.handleSessionClear(ctx, log, plugin, store, msg)
	default:
		d.handleRegularMessage(ctx, log, plugin, store, msg)
	}
}

// --- Command handlers ---

func (d *ChannelDispatcher) handleSessionCreate(ctx context.Context, log hclog.Logger, plugin plugins.ChannelPlugin, store *BindingStore, msg plugins.ChannelMessage) {
	name, _ := metaStr(msg.Metadata, "session_name")
	if name == "" {
		d.reply(ctx, log, plugin, msg, "Session name is required. Usage: `/session create name:<name> model:<provider/model>`")
		return
	}
	model, _ := metaStr(msg.Metadata, "model")
	if model == "" {
		d.reply(ctx, log, plugin, msg, "Model is required. Usage: `/session create name:<name> model:<provider/model>`")
		return
	}

	sess, err := d.sessions.Create(session.CreateOptions{
		Name:  name,
		Model: model,
	})
	if err != nil {
		log.Error("Failed to create session", "name", name, "error", err)
		d.reply(ctx, log, plugin, msg, fmt.Sprintf("Failed to create session: %v", err))
		return
	}

	if err := store.Set(msg.Channel, &ChannelBinding{
		SessionID:   sess.ID,
		SessionName: sess.Name,
		BoundAt:     time.Now(),
	}); err != nil {
		log.Warn("Failed to persist channel binding", "error", err)
	}

	d.reply(ctx, log, plugin, msg, fmt.Sprintf("Session **%s** created and bound to this channel. Model: `%s`.", sess.Name, model))
}

func (d *ChannelDispatcher) handleSessionSelect(ctx context.Context, log hclog.Logger, plugin plugins.ChannelPlugin, store *BindingStore, msg plugins.ChannelMessage) {
	name, _ := metaStr(msg.Metadata, "session_name")
	if name == "" {
		d.reply(ctx, log, plugin, msg, "Session name is required. Usage: `/session select name:<name>`")
		return
	}

	sess, err := d.sessions.Get(name)
	if err != nil {
		d.reply(ctx, log, plugin, msg, fmt.Sprintf("Session **%s** not found.", name))
		return
	}

	if err := store.Set(msg.Channel, &ChannelBinding{
		SessionID:   sess.ID,
		SessionName: sess.Name,
		BoundAt:     time.Now(),
	}); err != nil {
		log.Warn("Failed to persist channel binding", "error", err)
	}

	d.reply(ctx, log, plugin, msg, fmt.Sprintf("Session **%s** is now bound to this channel.", sess.Name))
}

func (d *ChannelDispatcher) handleSessionList(ctx context.Context, log hclog.Logger, plugin plugins.ChannelPlugin, msg plugins.ChannelMessage) {
	sessions, err := d.sessions.List(session.ListOptions{})
	if err != nil {
		log.Error("Failed to list sessions", "error", err)
		d.reply(ctx, log, plugin, msg, "Failed to list sessions.")
		return
	}
	if len(sessions) == 0 {
		d.reply(ctx, log, plugin, msg, "No sessions found. Use `/session create` to get started.")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "**%d session(s):**\n", len(sessions))
	for _, s := range sessions {
		if s.Title != "" {
			fmt.Fprintf(&sb, "• **%s** — model: `%s`, title: %s\n", s.Name, s.Model, s.Title)
		} else {
			fmt.Fprintf(&sb, "• **%s** — model: `%s`\n", s.Name, s.Model)
		}
	}
	d.reply(ctx, log, plugin, msg, sb.String())
}

func (d *ChannelDispatcher) handleSessionInfo(ctx context.Context, log hclog.Logger, plugin plugins.ChannelPlugin, store *BindingStore, msg plugins.ChannelMessage) {
	binding, ok := store.Get(msg.Channel)
	if !ok {
		d.reply(ctx, log, plugin, msg, "No session is bound to this channel. Use `/session create` or `/session select`.")
		return
	}

	sess, err := d.sessions.Get(binding.SessionID)
	if err != nil {
		d.reply(ctx, log, plugin, msg, fmt.Sprintf("Bound session **%s** could not be loaded: %v", binding.SessionName, err))
		return
	}

	d.reply(ctx, log, plugin, msg, fmt.Sprintf(
		"**Current session:** %s\n**Model:** `%s`\n**Messages:** %d\n**Bound at:** %s",
		sess.Name, sess.Model, sess.MessageCount, binding.BoundAt.Format(time.RFC1123),
	))
}

func (d *ChannelDispatcher) handleSessionClear(ctx context.Context, log hclog.Logger, plugin plugins.ChannelPlugin, store *BindingStore, msg plugins.ChannelMessage) {
	binding, ok := store.Get(msg.Channel)
	if !ok {
		d.reply(ctx, log, plugin, msg, "No session is currently bound to this channel.")
		return
	}
	name := binding.SessionName
	if err := store.Delete(msg.Channel); err != nil {
		log.Warn("Failed to remove channel binding", "error", err)
	}
	d.reply(ctx, log, plugin, msg, fmt.Sprintf("Session **%s** unbound from this channel.", name))
}

func (d *ChannelDispatcher) handleRegularMessage(ctx context.Context, log hclog.Logger, plugin plugins.ChannelPlugin, store *BindingStore, msg plugins.ChannelMessage) {
	binding, ok := store.Get(msg.Channel)
	if !ok {
		if err := d.send(ctx, plugin, msg.Channel,
			"No session is configured for this channel. Use `/session create` or `/session select` to get started.",
			buildReplyMeta(msg),
		); err != nil {
			log.Warn("Failed to send no-session notice", "error", err)
		}
		return
	}

	stream, err := d.sessions.Dispatch(ctx, binding.SessionID, msg.Content)
	if err != nil {
		log.Error("Failed to dispatch to session", "session", binding.SessionID, "error", err)
		d.reply(ctx, log, plugin, msg, fmt.Sprintf("Failed to process message: %v", err))
		return
	}

	result, err := plugins.CollectStream(stream)
	if err != nil {
		log.Error("Failed to collect pipeline stream", "error", err)
		d.reply(ctx, log, plugin, msg, "An error occurred while generating a response.")
		return
	}

	if result.Content == "" {
		return
	}

	replyMeta := buildReplyMeta(msg)
	if guildID, _ := metaStr(msg.Metadata, "guild_id"); guildID != "" {
		tname := msg.Content
		if len(tname) > 50 {
			tname = tname[:50]
		}
		replyMeta["thread_name"] = tname
	}

	if err := d.send(ctx, plugin, msg.Channel, result.Content, replyMeta); err != nil {
		log.Error("Failed to send response", "error", err)
	}
}

// --- Helpers ---

func (d *ChannelDispatcher) reply(ctx context.Context, log hclog.Logger, plugin plugins.ChannelPlugin, msg plugins.ChannelMessage, text string) {
	if err := d.send(ctx, plugin, msg.Channel, text, buildReplyMeta(msg)); err != nil {
		log.Warn("Failed to send reply", "error", err)
	}
}

func (d *ChannelDispatcher) send(ctx context.Context, plugin plugins.ChannelPlugin, channelID, content string, metadata map[string]any) error {
	_, err := plugin.Send(ctx, channelID, content, metadata)
	return err
}

func buildReplyMeta(msg plugins.ChannelMessage) map[string]any {
	meta := make(map[string]any)
	if token, ok := metaStr(msg.Metadata, "interaction_token"); ok && token != "" {
		meta["interaction_token"] = token
		if appID, ok := metaStr(msg.Metadata, "application_id"); ok {
			meta["application_id"] = appID
		}
	} else {
		meta["reply_to_id"] = msg.ID
	}
	return meta
}

func metaStr(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
