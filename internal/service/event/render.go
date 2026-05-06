package event

import (
	"fmt"
	"strings"
	"time"

	"github.com/mwantia/forge/internal/service/template"
	"github.com/zclconf/go-cty/cty"
)

// renderPrompt evaluates the Go template string with event-scoped variables:
//
//	{{ .payload }}           — decoded JSON body of the event (fields accessible via dot)
//	{{ .event.id }}          — ID of the event definition
//	{{ .event.fired_at }}    — RFC3339 timestamp when the event fired
//	{{ .event.http.method }} — HTTP method used to fire ("GET" or "POST")
func RenderPrompt(renderer template.TemplateRenderer, cfg *EventConfig, ev EventInfo) (string, error) {
	tmpl, err := renderer.Clone(
		template.WithJsonValue("payload", ev.payload),

		template.WithValue("event.id", cty.StringVal(cfg.ID)),
		template.WithValue("event.desc", cty.StringVal(cfg.Description)),
		template.WithValue("event.session", cty.StringVal(cfg.Session)),
		template.WithValue("event.branch", cty.StringVal(cfg.Branch)),
		template.WithValue("event.model", cty.StringVal(cfg.Model)),
		template.WithValue("event.fired_at", cty.StringVal(ev.firedAt.UTC().Format(time.RFC3339))),
		template.WithValue("event.http.method", cty.StringVal("POST")), // Currently fix
	)
	if err != nil {
		return "", fmt.Errorf("failed to clone renderer template: %w", err)
	}

	body, err := tmpl.RenderBody(cfg.Prompt)
	if err != nil {
		return "", fmt.Errorf("failed to render body template: %w", err)
	}
	return body, nil
}

// assembleUserMessage renders each entry through the prompt template (or uses
// the raw payload when no prompt is configured) and joins them with numbered
// headers for batch dispatches.
func assembleUserMessage(renderer template.TemplateRenderer, entries []EventInfo, cfg *EventConfig) (string, error) {
	n := len(entries)
	parts := make([]string, 0, n)

	for i, ev := range entries {
		var msg string

		if cfg.Prompt != "" {
			rendered, err := RenderPrompt(renderer, cfg, ev)
			if err != nil {
				return "", fmt.Errorf("event %q entry %d: %w", cfg.ID, i+1, err)
			}
			msg = rendered
		} else {
			msg = string(ev.payload)
		}

		if n > 1 {
			header := fmt.Sprintf("[Event %d/%d — %s]", i+1, n, ev.firedAt.UTC().Format(time.RFC3339))
			parts = append(parts, header+"\n"+msg)
		} else {
			parts = append(parts, msg)
		}
	}

	return strings.Join(parts, "\n\n"), nil
}
