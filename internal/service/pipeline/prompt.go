package pipeline

import (
	"context"
	"strings"

	"github.com/hashicorp/go-hclog"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session"
	"github.com/mwantia/forge/internal/service/template"
	"github.com/mwantia/forge/internal/service/tools"
)

// promptLayers is the deterministic, cache-friendly view of every fragment
// that contributes to the assembled system prompt for a single pipeline turn.
//
// Layer order is chosen for prompt-cache stability:
//
//	agent → builtins (fixed) → model → plugins (sorted) → session
//
// `agent` and `builtins` are agent-process-global and never vary across
// requests, so they sit at the front for maximum cache reuse. `model` is
// per-session but stable across turns. `plugins` is the volatile bucket —
// adding or removing a plugin only invalidates the suffix, not the
// agent+builtins prefix.
//
// Every field is captured at request-build time so assembly is a pure
// function of `promptLayers`.
type promptLayers struct {
	agent     string
	builtins  []pluginPromptBlock
	model     string
	plugins   []pluginPromptBlock
	session   string
	resources string // pre-rendered <relevant-resources> block, populated per-turn
}

type pluginPromptBlock struct {
	name        string
	version     string
	description string
	system      string
	tools       []toolPromptEntry
}

type toolPromptEntry struct {
	name   string // fully-qualified "namespace__tool" form shown to the agent
	system string
}

// collectPromptLayers gathers every prompt fragment the pipeline knows about
// for the current turn, calls each plugin's System(ctx) once, and returns a
// fully-ordered `promptLayers`.
//
// Plugin-level System() failures are logged and skipped (the rest of the
// fragments still contribute) — a misbehaving plugin must never break the
// pipeline.
//
// meta.ToolsVerbosity controls how much plugin/tool content is included:
//   - "full" (default): plugin prose + per-tool annotations
//   - "basic": plugin-level prose only, no per-tool annotations
//   - "none": no plugin or tool blocks at all
//
// meta.Plugins, when non-empty, restricts which external plugin namespaces
// appear in the prompt. Built-in namespaces always pass through.
func collectPromptLayers(ctx context.Context, agentSystem string, modelSystem string, meta *session.SessionMetadata, registar tools.ToolsRegistar, logger hclog.Logger) promptLayers {
	layers := promptLayers{
		agent:   agentSystem,
		model:   modelSystem,
		session: meta.System,
	}

	verbosity := meta.ToolsVerbosity
	if verbosity == "" {
		verbosity = "full"
	}

	if verbosity == "none" {
		layers.builtins = []pluginPromptBlock{}
		layers.plugins = []pluginPromptBlock{}
		return layers
	}

	// Build allowed-plugin index (lower-cased) for O(1) lookup.
	// An empty set means all external plugins are allowed.
	allowedPlugins := make(map[string]struct{}, len(meta.Plugins))
	for _, p := range meta.Plugins {
		allowedPlugins[strings.ToLower(p)] = struct{}{}
	}

	namespaces := registar.ListNamespaces()
	layers.builtins = make([]pluginPromptBlock, 0, len(namespaces))
	layers.plugins = make([]pluginPromptBlock, 0, len(namespaces))
	for _, ns := range namespaces {
		// Apply the plugins allow-list only to external (non-builtin) namespaces.
		if !ns.Builtin && len(allowedPlugins) > 0 {
			if _, ok := allowedPlugins[strings.ToLower(ns.Namespace)]; !ok {
				continue
			}
		}

		block := pluginPromptBlock{
			name:        ns.Namespace,
			version:     ns.Version,
			description: ns.Description,
		}
		if ns.Plugin != nil {
			prompt, err := ns.Plugin.System(ctx)
			if err != nil {
				logger.Warn("Failed to fetch plugin system prompt", "namespace", ns.Namespace, "error", err)
			} else {
				block.system = prompt
			}
		} else {
			block.system = ns.System
		}

		// "basic" verbosity: include plugin-level prose but omit per-tool annotations.
		if verbosity == "full" {
			block.tools = make([]toolPromptEntry, 0, len(ns.Tools))
			for _, t := range ns.Tools {
				if t.Annotations.System == "" {
					continue
				}
				block.tools = append(block.tools, toolPromptEntry{
					name:   t.Name,
					system: t.Annotations.System,
				})
			}
		}

		if ns.Builtin {
			layers.builtins = append(layers.builtins, block)
		} else {
			layers.plugins = append(layers.plugins, block)
		}
	}
	return layers
}

// assembleSystemPrompt joins promptLayers into a single markdown system
// message. Empty fragments are silently skipped. Each fragment is rendered
// through `tmpl` so session/template variables (${session.id}, ${now()}, ...)
// resolve consistently regardless of layer.
//
// Layer order is fixed; ordering inside the plugins layer is the responsibility
// of `tools.ToolsRegistar.ListNamespaces` (alphabetic by namespace + tool name).
//
// The output is a stable byte sequence for any given input — Go map iteration
// must not leak in here.
func assembleSystemPrompt(p promptLayers, tmpl *template.Template, logger hclog.Logger) string {
	render := func(origin, text string) string {
		text = strings.TrimSpace(text)
		if text == "" {
			return ""
		}
		out, err := tmpl.Render(text)
		if err != nil {
			logger.Warn("Failed to render system prompt fragment", "origin", origin, "error", err)
			return text
		}
		return strings.TrimSpace(out)
	}

	var b strings.Builder
	appendSection := func(text string) {
		if text == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(text)
	}

	renderBlock := func(originPrefix string, plg pluginPromptBlock) string {
		header := pluginHeader(plg)
		body := render(originPrefix+":"+plg.name, plg.system)
		hasTools := false
		for _, t := range plg.tools {
			if t.system != "" {
				hasTools = true
				break
			}
		}
		if header == "" && body == "" && !hasTools {
			return ""
		}

		var section strings.Builder
		if header != "" {
			section.WriteString(header)
		}
		if body != "" {
			if section.Len() > 0 {
				section.WriteString("\n\n")
			}
			section.WriteString(body)
		}
		for _, t := range plg.tools {
			rendered := render("tool:"+t.name, t.system)
			if rendered == "" {
				continue
			}
			if section.Len() > 0 {
				section.WriteString("\n\n")
			}
			section.WriteString("### ")
			section.WriteString(t.name)
			section.WriteString("\n")
			section.WriteString(rendered)
		}
		return section.String()
	}

	appendSection(render("agent", p.agent))
	for _, blt := range p.builtins {
		appendSection(renderBlock("builtin", blt))
	}
	appendSection(render("model", p.model))
	for _, plg := range p.plugins {
		appendSection(renderBlock("plugin", plg))
	}
	appendSection(render("session", p.session))
	// Resources are last: they're the per-turn slot, most cache-volatile.
	appendSection(p.resources)

	return b.String()
}

// builtinNamespaceSet returns the set of namespace names that are marked
// builtin in the collected prompt layers. Used by filterToolCallsByPlugins so
// builtin tools are never removed by the plugins allow-list.
func builtinNamespaceSet(layers promptLayers) map[string]struct{} {
	set := make(map[string]struct{}, len(layers.builtins))
	for _, b := range layers.builtins {
		set[strings.ToLower(b.name)] = struct{}{}
	}
	return set
}

// filterToolCallsByPlugins removes tool calls whose namespace is not in the
// allowed set. When allowedNamespaces is empty all calls pass through.
// Built-in namespaces (those whose NamespaceInfo.Builtin == true) are not
// consulted here — the caller already knows which calls to keep because
// GetAllToolCalls returns the flat name list. We therefore treat any namespace
// absent from the allow-list as blocked, regardless of builtin status, unless
// the allow-list itself is empty.
//
// To preserve built-in tools unconditionally, callers should NOT include them
// in GetAllToolCalls filtering — instead this helper is only called when
// meta.Plugins is non-empty, and the caller is responsible for deciding
// whether builtins should bypass the filter. The current contract: builtins
// always bypass (their namespace is never in the deny set).
func filterToolCallsByPlugins(calls []sdkplugins.ToolCall, builtinNamespaces map[string]struct{}, allowedPlugins []string) []sdkplugins.ToolCall {
	if len(allowedPlugins) == 0 {
		return calls
	}
	allowed := make(map[string]struct{}, len(allowedPlugins))
	for _, p := range allowedPlugins {
		allowed[strings.ToLower(p)] = struct{}{}
	}
	out := calls[:0:0] // fresh slice, same underlying array reuse avoided
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

// renderResourcesBlock formats Recall hits as a <relevant-resources>
// system block. Empty input returns "" so assembleSystemPrompt can elide
// the section entirely.
func renderResourcesBlock(items []resourceItem) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<relevant-resources>")
	for _, r := range items {
		b.WriteString("\n  <resource id=\"")
		b.WriteString(r.ID)
		b.WriteString("\">\n    ")
		b.WriteString(strings.ReplaceAll(strings.TrimSpace(r.Content), "\n", "\n    "))
		b.WriteString("\n  </resource>")
	}
	b.WriteString("\n</relevant-resources>")
	return b.String()
}

// resourceItem is the prompt-layer view of a Resource — only the bits the
// model actually consumes. Decouples the prompt code from the SDK type.
type resourceItem struct {
	ID      string
	Content string
}

// pluginHeader formats the per-plugin block header per the proposal:
//
//	## <name> (<version>) - <description>
//
// Pieces are omitted gracefully when missing so namespaces without metadata
// (e.g. built-in session/memory tools) still get a clean header.
func pluginHeader(plg pluginPromptBlock) string {
	if plg.name == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(plg.name)
	if plg.version != "" {
		b.WriteString(" (")
		b.WriteString(plg.version)
		b.WriteString(")")
	}
	if plg.description != "" {
		b.WriteString(" - ")
		b.WriteString(plg.description)
	}
	return b.String()
}
