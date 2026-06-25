package system

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/mwantia/forge-sdk/pkg/plugin/tool"
	domsession "github.com/mwantia/forge/internal/domain/session"
	domtool "github.com/mwantia/forge/internal/domain/tool"
)

var startTime = time.Now()

func (s *SystemService) registerForgeTools() error {
	// SystemService is the sole owner of the "builtin" namespace metadata.
	// All other services (pipeline, resource, session) register tools under
	// "builtin" without re-registering metadata.
	if err := s.tools.RegisterNamespaceMetadata("builtin", domtool.NamespaceMetadata{
		Builtin:     true,
		Description: "Built-in tools for session management, resource memory, pipeline dispatch, and system introspection.",
		System: `Use session tools to manage metadata, spawn sub-sessions, and inspect message history.
Use resource tools to persist and retrieve context across turns and sessions.
Use pipeline tools to drive sub-sessions synchronously.
Use system tools for health checks and capability introspection.`,
	}); err != nil {
		return fmt.Errorf("failed to register builtin namespace metadata: %w", err)
	}

	if err := s.tools.RegisterTool("builtin", tool.ToolDefinition{
		Name:        "system_status",
		Description: "Check the live health of forge plugins and providers. Pass an optional plugin name for a targeted check; omit for a full overview.",
		Annotations: tool.ToolAnnotations{
			ReadOnly: true,
			CostHint: tool.ToolCostCheap,
			System: `Call when the user asks why something isn't working or what is available.
Do NOT call on every turn. Parse the response and surface each degraded plugin's Action field verbatim as the remediation step.
Pass "name" for a targeted check after the full status reveals a degraded entry.`,
		},
		Parameters: tool.ToolParameters{
			Type: "object",
			Properties: map[string]tool.ToolProperty{
				"name": {Type: "string", Description: "Optional plugin name for a targeted health check. Omit to check all plugins."},
			},
		},
	}, s.execSystemStatus); err != nil {
		return fmt.Errorf("failed to register builtin__system_status: %w", err)
	}

	if err := s.tools.RegisterTool("builtin", tool.ToolDefinition{
		Name:        "agent_info",
		Description: "Return static forge agent metadata: uptime, loaded plugins, and runtime. Use for capability questions without triggering health checks.",
		Annotations: tool.ToolAnnotations{
			ReadOnly: true,
			CostHint: tool.ToolCostCheap,
		},
		Parameters: tool.ToolParameters{
			Type:       "object",
			Properties: map[string]tool.ToolProperty{},
		},
	}, s.execAgentInfo); err != nil {
		return fmt.Errorf("failed to register builtin__agent_info: %w", err)
	}

	if err := s.tools.RegisterTool("builtin", tool.ToolDefinition{
		Name:        "plugin_activate",
		Description: "Load the full tool definitions and system instructions for a plugin namespace.",
		Annotations: tool.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   tool.ToolCostCheap,
			System: `Call when the user's request matches a plugin described in the system prompt but whose tools
are not yet available. The compact summary shown per plugin is enough to decide whether to activate it.
Do not attempt to activate a plugin the user has explicitly disabled — surface intent instead.`,
		},
		Parameters: tool.ToolParameters{
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]tool.ToolProperty{
				"name":    {Type: "string", Description: "Plugin namespace to activate (e.g. \"searxng\", \"consul\")."},
				"verbose": {Type: "boolean", Description: "When true, request the full operational system prompt. Defaults to false."},
			},
		},
	}, s.execPluginActivate); err != nil {
		return fmt.Errorf("failed to register builtin__plugin_activate: %w", err)
	}

	return nil
}

func (s *SystemService) execSystemStatus(ctx context.Context, req tool.ExecuteToolRequest) (*tool.ExecuteToolResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Targeted check when name is provided (replaces the former plugin_health tool).
	if name := req.Args.Get("name").StringOr(""); name != "" {
		driver, ok := s.plugins.GetDriver(name)
		if !ok {
			return tool.ExecuteErrorf("plugin %q not found", name), nil
		}
		h, _ := driver.Driver.GetPluginHealth(ctx)

		return tool.ExecuteSuccess(toHealthEntry(name, driver.Capabilities, h)), nil
	}

	// Full fan-out over all drivers.
	entries, worst := fanOutHealth(ctx, s.plugins.ListDrivers())
	return tool.ExecuteSuccess(map[string]any{
		"status":  worst,
		"plugins": entries,
		"uptime":  time.Since(startTime).Round(time.Second).String(),
	}), nil
}

func (s *SystemService) execAgentInfo(_ context.Context, _ tool.ExecuteToolRequest) (*tool.ExecuteToolResponse, error) {
	drivers := s.plugins.ListDrivers()
	names := make([]string, 0, len(drivers))
	for _, d := range drivers {
		names = append(names, d.Info.Name)
	}

	return tool.ExecuteSuccess(map[string]any{
		"uptime":  time.Since(startTime).Round(time.Second).String(),
		"go":      runtime.Version(),
		"plugins": names,
	}), nil
}

func (s *SystemService) execPluginActivate(ctx context.Context, req tool.ExecuteToolRequest) (*tool.ExecuteToolResponse, error) {
	name := strings.ToLower(req.Args.Get("name").StringOr(""))
	if name == "" {
		return tool.ExecuteErrorMsg("name is required"), nil
	}
	verbose := req.Args.Get("verbose").BoolOr(false)

	// Find the namespace in the registar.
	var target *domtool.NamespaceInfo
	for _, ns := range s.tools.ListNamespaces() {
		if strings.ToLower(ns.Namespace) == name {
			ns := ns
			target = &ns
			break
		}
	}
	if target == nil {
		return tool.ExecuteErrorf("plugin namespace %q not found", name), nil
	}

	// Load session and update its plugin config.
	sessionID := domsession.CallerSessionID(ctx)
	if sessionID != "" {
		meta, err := s.sessions.LoadSession(ctx, sessionID)
		if err == nil {
			// Find or create the PluginConfig entry for this namespace.
			found := false
			for i, p := range meta.Plugins {
				if strings.ToLower(p.Name) == name {
					if p.Disabled {
						// User hard-disabled — emit elevation request instead of enabling.
						return tool.ExecuteSuccess(map[string]any{
							"elevation_required": true,
							"plugin":             name,
							"message":            fmt.Sprintf("Plugin %q is disabled by the user. Request user approval via builtin__plugin_activate to re-enable.", name),
						}), nil
					}
					meta.Plugins[i].Enabled = true
					if verbose {
						meta.Plugins[i].Verbose = true
					}
					found = true
					break
				}
			}
			if !found {
				meta.Plugins = append(meta.Plugins, domsession.PluginConfig{
					Name:    name,
					Enabled: true,
					Verbose: verbose,
				})
			}
			_ = s.sessions.SaveSession(ctx, meta)
		}
	}

	// Return the plugin's system prompt + compact tool index.
	systemPrompt := ""
	if target.Plugin != nil {
		if s, err := target.Plugin.System(ctx, verbose); err == nil {
			systemPrompt = s
		}
	} else {
		systemPrompt = target.System
	}

	tools := make([]string, 0, len(target.Tools))
	for _, t := range target.Tools {
		tools = append(tools, t.Name)
	}

	return tool.ExecuteSuccess(map[string]any{
		"plugin":  name,
		"system":  systemPrompt,
		"tools":   tools,
		"message": fmt.Sprintf("Plugin %q activated. Full tool schemas will be available on the next turn.", name),
	}), nil
}
