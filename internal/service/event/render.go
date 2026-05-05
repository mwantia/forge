package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	svctemplate "github.com/mwantia/forge/internal/service/template"
	"github.com/zclconf/go-cty/cty"
)

// parsePayload tries to unmarshal body as JSON; falls back to raw string.
func parsePayload(body []byte) any {
	var v any
	if json.Unmarshal(body, &v) == nil {
		return v
	}
	return string(body)
}

// renderPrompt evaluates the HCL template string with event-scoped variables:
//
//	${payload}   — pretty-printed JSON (or raw string) body of the event
//	${event_id}  — ID of the event definition
//	${fired_at}  — RFC3339 timestamp when the event fired
//	${method}    — HTTP method used to fire ("GET" or "POST")
func renderPrompt(renderer svctemplate.TemplateRenderer, tmplSrc string, payload any, eventID string, firedAt time.Time, method string) (string, error) {
	t, err := renderer.Clone(
		svctemplate.WithAnyValue("payload", payload),
		svctemplate.WithValue("event_id", cty.StringVal(eventID)),
		svctemplate.WithValue("fired_at", cty.StringVal(firedAt.UTC().Format(time.RFC3339))),
		svctemplate.WithValue("method", cty.StringVal(method)),
	)
	if err != nil {
		return "", fmt.Errorf("clone event template: %w", err)
	}
	result, err := t.Render(tmplSrc)
	if err != nil {
		return "", fmt.Errorf("render event template: %w", err)
	}
	return result, nil
}

// assembleUserMessage renders each entry through the prompt template (or uses
// the raw payload when no prompt is configured) and joins them with numbered
// headers for batch dispatches.
func assembleUserMessage(renderer svctemplate.TemplateRenderer, entries []queueEntry, cfg *EventConfig) (string, error) {
	n := len(entries)
	parts := make([]string, 0, n)

	for i, e := range entries {
		var msg string

		if cfg.Prompt != "" {
			rendered, err := renderPrompt(renderer, cfg.Prompt, e.payload, cfg.ID, e.firedAt, "POST")
			if err != nil {
				return "", fmt.Errorf("event %q entry %d: %w", cfg.ID, i+1, err)
			}
			msg = rendered
		} else {
			// No prompt configured — emit the payload directly.
			switch v := e.payload.(type) {
			case string:
				msg = v
			default:
				b, _ := json.MarshalIndent(v, "", "  ")
				msg = string(b)
			}
		}

		if n > 1 {
			header := fmt.Sprintf("[Event %d/%d — %s]", i+1, n, e.firedAt.UTC().Format(time.RFC3339))
			parts = append(parts, header+"\n"+msg)
		} else {
			parts = append(parts, msg)
		}
	}

	return strings.Join(parts, "\n\n"), nil
}
