package tmplsessions

import (
	"strings"

	appsession "github.com/mwantia/forge/internal/application/session"
)

// RenderedMessage pairs a raw DAG message with its template-rendered content.
// Raw holds the original template source; Rendered holds the evaluated output.
// The UI shows Rendered by default and lets the user toggle to Raw per bubble.
type RenderedMessage struct {
	*appsession.Message
	Rendered string
}

// pluginNames extracts plugin names from a PluginConfig slice and joins them
// with sep. Returns "" when the slice is empty.
func pluginNames(plugins []appsession.PluginConfig, sep string) string {
	names := make([]string, len(plugins))
	for i, p := range plugins {
		names[i] = p.Name
	}
	
	return strings.Join(names, sep)
}
