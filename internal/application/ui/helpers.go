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

// renderMessages converts a slice of messages into RenderedMessages with
// per-block display token counts computed from the chain.
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
	computeDisplayTokens(out)
	return out
}

// computeDisplayTokens walks the rendered message chain and fills DisplayTokens
// for each block using delta arithmetic on consecutive assistant InputTokens.
//
// Assistant messages: OutputTokens (+ tool-result delta when tool msgs precede).
// User messages: net context delta since the previous assistant turn.
// Tool/system messages: left at 0.
func computeDisplayTokens(msgs []*tmplsessions.RenderedMessage) {
	// Collect assistant messages that carry usage, in chain order.
	type aInfo struct{ idx, input, output int }
	var assts []aInfo
	for i, m := range msgs {
		if m.Role == "assistant" && m.Usage != nil && m.Usage.InputTokens > 0 {
			assts = append(assts, aInfo{i, m.Usage.InputTokens, m.Usage.OutputTokens})
		}
	}
	if len(assts) == 0 {
		return
	}

	for ai, a := range assts {
		prevIdx := -1
		var prevInput, prevOutput int
		if ai > 0 {
			p := assts[ai-1]
			prevIdx, prevInput, prevOutput = p.idx, p.input, p.output
		}

		// blockDelta is the net context growth from non-assistant messages since
		// the last turn. Clamp to zero: a negative value means the provider
		// truncated history (context window overflow) and is not meaningful.
		blockDelta := max(0, a.input-prevInput-prevOutput)

		// Detect whether tool messages sit between the previous assistant and this one.
		hasTool := false
		for j := prevIdx + 1; j < a.idx; j++ {
			if msgs[j].Role == "tool" {
				hasTool = true
				break
			}
		}

		if hasTool {
			// Tool results are visually grouped with this assistant block.
			msgs[a.idx].DisplayTokens = blockDelta + a.output
		} else {
			msgs[a.idx].DisplayTokens = a.output
			// Attribute the delta to the preceding user message.
			if blockDelta > 0 {
				for j := prevIdx + 1; j < a.idx; j++ {
					if msgs[j].Role == "user" {
						msgs[j].DisplayTokens = blockDelta
						break
					}
				}
			}
		}
	}
}
