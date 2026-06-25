package tool

import (
	"context"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/plugin/tool"
)

// ToolsExecution is the function signature for tool call handlers.
type ToolsExecution = func(ctx context.Context, request tool.ExecuteToolRequest) (*tool.ExecuteToolResponse, error)

// NamespaceMetadata is the per-namespace data captured at plugin load time.
type NamespaceMetadata struct {
	Description string
	Version     string
	Plugin      tool.ToolsPlugin
	// Builtin marks namespaces owned by the agent process itself (memory,
	// sessions). Builtins always sit in a fixed cache-stable section of the
	// assembled system prompt, ahead of dynamically loaded plugins.
	Builtin bool
	// System is a static plugin-level system prompt fragment. Only consulted
	// when Plugin is nil — plugin-backed namespaces source their fragment
	// from ToolsPlugin.System(ctx) at request time.
	System string
}

// NamespaceInfo is the public, sorted view returned by ListNamespaces.
type NamespaceInfo struct {
	Namespace   string
	Description string
	Version     string
	Plugin      tool.ToolsPlugin
	Builtin     bool
	System      string
	Tools       []tool.ToolDefinition
}

// SplitToolCallName splits a fully-qualified "namespace__name" tool call
// identifier into its two components. Returns false when the format is invalid.
func SplitToolCallName(s string) (namespace, name string, ok bool) {
	parts := strings.SplitN(s, "__", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	return strings.ToLower(parts[0]), strings.ToLower(parts[1]), true
}
