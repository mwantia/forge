package pipeline

import (
	"context"
	"strings"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	domtool "github.com/mwantia/forge/internal/domain/tool"
)

// buildToolsData assembles the live tool namespace tree injected into every
// system-prompt render as {{ .tools }}. Plugin System() calls are made once
// per turn; failures are silently skipped so a misbehaving plugin never
// breaks the prompt.
func buildToolsData(ctx context.Context, registar domtool.ToolsRegistar) map[string]any {
	namespaces := make(map[string]any)

	for _, ns := range registar.ListNamespaces() {
		system := ns.System
		if ns.Plugin != nil {
			if s, err := ns.Plugin.System(ctx); err == nil {
				system = s
			}
		}

		defs := make([]any, 0, len(ns.Tools))
		for _, t := range ns.Tools {
			defs = append(defs, map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"annotations": map[string]any{
					"system":                t.Annotations.System,
					"read_only":             t.Annotations.ReadOnly,
					"idempotent":            t.Annotations.Idempotent,
					"destructive":           t.Annotations.Destructive,
					"requires_confirmation": t.Annotations.RequiresConfirmation,
				},
			})
		}

		namespaces[ns.Namespace] = map[string]any{
			"name":        ns.Namespace,
			"version":     ns.Version,
			"description": ns.Description,
			"system":      system,
			"builtin":     ns.Builtin,
			"definitions": defs,
		}
	}

	return map[string]any{"namespaces": namespaces}
}

// filterToolCallsByPlugins removes tool calls whose namespace is not in the
// allowed set. When allowedNamespaces is empty all calls pass through.
// Builtin namespaces always bypass the filter.
func filterToolCallsByPlugins(calls []sdkplugins.ToolCall, builtinNamespaces map[string]struct{}, allowedPlugins []string) []sdkplugins.ToolCall {
	if len(allowedPlugins) == 0 {
		return calls
	}
	allowed := make(map[string]struct{}, len(allowedPlugins))
	for _, p := range allowedPlugins {
		allowed[strings.ToLower(p)] = struct{}{}
	}
	out := calls[:0:0]
	for _, tc := range calls {
		ns, _, ok := strings.Cut(tc.Name, "__")
		if !ok {
			out = append(out, tc)
			continue
		}
		nsLower := strings.ToLower(ns)
		if _, isBuiltin := builtinNamespaces[nsLower]; isBuiltin {
			out = append(out, tc)
			continue
		}
		if _, isAllowed := allowed[nsLower]; isAllowed {
			out = append(out, tc)
		}
	}
	return out
}

// builtinNamespaceSetFromRegistar returns a set of builtin namespace names.
func builtinNamespaceSetFromRegistar(r domtool.ToolsRegistar) map[string]struct{} {
	ns := r.ListNamespaces()
	set := make(map[string]struct{}, len(ns))
	for _, n := range ns {
		if n.Builtin {
			set[strings.ToLower(n.Namespace)] = struct{}{}
		}
	}
	return set
}

// renderResourcesBlock formats Recall hits as a <relevant-resources> block.
// Empty input returns "" so callers can skip injection entirely.
func renderResourcesBlock(items []resourceItem) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<relevant-resources>")
	for _, r := range items {
		b.WriteString("\n  <resource id=\"")
		b.WriteString(xmlEscape(r.ID))
		b.WriteString("\">\n    ")
		content := strings.ReplaceAll(strings.TrimSpace(r.Content), "\n", "\n    ")
		b.WriteString(xmlEscape(content))
		b.WriteString("\n  </resource>")
	}
	b.WriteString("\n</relevant-resources>")
	return b.String()
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// resourceItem is the prompt-layer view of a Resource consumed by renderResourcesBlock.
type resourceItem struct {
	ID      string
	Content string
}
