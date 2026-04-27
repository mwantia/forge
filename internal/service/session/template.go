package session

import (
	"github.com/mwantia/forge/internal/service/template"
	"github.com/zclconf/go-cty/cty"
)

// SessionVars returns a TemplateOption that registers session-scoped variables
// for system-prompt rendering. Exposed under the "session.*" namespace:
//
//	${session.id}           — session ID
//	${session.name}         — session display name
//	${session.title}        — optional title
//	${session.description}  — optional description
//	${session.parent}       — parent session ID (empty for root)
//	${session.model}        — model reference (e.g. "ollama/llama3.2")
//	${session.created_at}   — RFC3339 timestamp
//	${session.updated_at}   — RFC3339 timestamp
func SessionVars(meta *SessionMetadata) template.TemplateOption {
	return func(t *template.Template) error {
		values := map[string]cty.Value{
			"session.id":          cty.StringVal(meta.ID),
			"session.name":        cty.StringVal(meta.Name),
			"session.title":       cty.StringVal(meta.Title),
			"session.description": cty.StringVal(meta.Description),
			"session.parent":      cty.StringVal(meta.Parent),
			"session.model":       cty.StringVal(meta.Model),
			"session.created_at":  cty.StringVal(meta.CreatedAt.Format("2006-01-02T15:04:05Z07:00")),
			"session.updated_at":  cty.StringVal(meta.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")),
		}
		for k, v := range values {
			if err := template.WithValue(k, v)(t); err != nil {
				return err
			}
		}
		return nil
	}
}
