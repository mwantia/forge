package tmplsessions

import appsession "github.com/mwantia/forge/internal/application/session"

// RenderedMessage pairs a raw DAG message with its template-rendered content.
// Raw holds the original template source; Rendered holds the evaluated output.
// The UI shows Rendered by default and lets the user toggle to Raw per bubble.
type RenderedMessage struct {
	*appsession.Message
	Rendered string
}
