package pipeline

import (
	"context"
	"strings"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	appsession "github.com/mwantia/forge/internal/application/session"
	domtool "github.com/mwantia/forge/internal/domain/tool"
)

// buildToolsData assembles the live tool namespace tree injected into every
// system-prompt render as {{ .tools }}. Plugin System() calls are made once
// per turn; failures are silently skipped so a misbehaving plugin never
// breaks the prompt.
//
// All non-builtin namespaces are included unless the user hard-disabled them.
// Tool schemas are withheld until the agent calls builtin__plugin_activate,
// which sets Enabled=true in the session metadata for that namespace.
func buildToolsData(ctx context.Context, registar domtool.ToolsRegistar, plugins []appsession.PluginConfig) map[string]any {
	pluginMap := make(map[string]appsession.PluginConfig, len(plugins))
	for _, p := range plugins {
		pluginMap[strings.ToLower(p.Name)] = p
	}

	namespaces := make(map[string]any)

	for _, ns := range registar.ListNamespaces() {
		cfg, hasCfg := pluginMap[strings.ToLower(ns.Namespace)]

		if !ns.Builtin {
			// Skip only if the user hard-disabled this plugin.
			if hasCfg && cfg.Disabled {
				continue
			}
		}

		verbose := hasCfg && cfg.Verbose

		system := ns.System
		if ns.Plugin != nil {
			if s, err := ns.Plugin.System(ctx, verbose); err == nil {
				system = s
			}
		}

		// Tool schemas are shown only for builtins and explicitly enabled plugins.
		// All others appear as a description-only entry; the agent calls
		// builtin__plugin_activate to request full schemas on the next turn.
		activated := ns.Builtin || (hasCfg && cfg.Enabled)

		var defs []any
		if activated {
			defs = make([]any, 0, len(ns.Tools))
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
		}

		namespaces[ns.Namespace] = map[string]any{
			"name":        ns.Namespace,
			"version":     ns.Version,
			"description": ns.Description,
			"system":      system,
			"builtin":     ns.Builtin,
			"activated":   activated,
			"definitions": defs,
		}
	}

	return map[string]any{"namespaces": namespaces}
}

// filterToolCallsByPlugins removes tool calls whose namespace is not enabled.
// Builtin namespaces always bypass the filter. Only plugins with Enabled=true
// (and Disabled=false) are allowed through.
func filterToolCallsByPlugins(calls []sdkplugins.ToolCall, builtinNamespaces map[string]struct{}, plugins []appsession.PluginConfig) []sdkplugins.ToolCall {
	allowed := make(map[string]struct{}, len(plugins))
	for _, p := range plugins {
		if p.Enabled && !p.Disabled {
			allowed[strings.ToLower(p.Name)] = struct{}{}
		}
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
