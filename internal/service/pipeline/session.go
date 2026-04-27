package pipeline

import (
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session"
)

// Session is the runtime dispatch request handed to RunSessionPipeline.
// It carries the resolved metadata, the materialized chat history (system +
// prior turns + the current user message), the available tool catalog, and
// the per-request output policy.
//
// Persisted state lives in the session package; this struct is not stored.
type Session struct {
	SessionID string
	Metadata  *session.SessionMetadata
	Messages  []sdkplugins.ChatMessage
	ToolCalls []sdkplugins.ToolCall
	Plugins   []string
	NoStore   bool

	// Output is the resolved per-request chunking/pacing policy.
	Output resolvedOutput
}
