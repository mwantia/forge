package ui

import (
	"context"

	appsession "github.com/mwantia/forge/internal/application/session"
	tmplsessions "github.com/mwantia/forge/internal/application/ui/templates/sessions"
)

// pluginNamespacesFrom returns the names of all non-builtin namespaces from l.
func pluginNamespacesFrom(l namespaceLister) []string {
	if l == nil {
		return nil
	}

	ns := l.ListNamespaces()
	names := make([]string, 0, len(ns))

	for _, n := range ns {
		if !n.Builtin {
			names = append(names, n.Namespace)
		}
	}

	return names
}

// renderMessages converts a slice of messages into RenderedMessages.
// The optional renderer processes content before markdown conversion.
// Markdown is applied only to assistant messages; other roles stay as plain text.
func renderMessages(ctx context.Context, renderer pipelineRenderer, sessionID string, msgs []*appsession.Message) []*tmplsessions.RenderedMessage {
	out := make([]*tmplsessions.RenderedMessage, len(msgs))
	for i, msg := range msgs {
		content := msg.Content
		if renderer != nil {
			if r, err := renderer.RenderContent(ctx, sessionID, msg.Content); err == nil {
				content = r
			}
		}
		rendered := content
		if msg.Role == "assistant" {
			rendered = renderMarkdown(content)
		}
		out[i] = &tmplsessions.RenderedMessage{Message: msg, Rendered: rendered}
	}
	return out
}
