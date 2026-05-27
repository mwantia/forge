package system

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/tools"
)

var startTime = time.Now()

func (s *SystemService) registerForgeTools() error {
	if err := s.tools.RegisterNamespaceMetadata("forge", tools.NamespaceMetadata{
		Builtin:     true,
		Description: "Built-in forge agent tools for system introspection.",
		System: `Call forge__system_status when the user asks why something is not working or what is available.
Call forge__plugin_health for a targeted check on a specific plugin after system_status shows a degraded entry.
Call forge__agent_info for capability questions ("can you send emails?", "which models are available?").
Do NOT call system_status on every turn. Parse the response and surface each degraded or unhealthy plugin's Action field verbatim as the remediation step.`,
	}); err != nil {
		return fmt.Errorf("failed to register forge namespace metadata: %w", err)
	}

	if err := s.tools.RegisterTool("forge", sdkplugins.ToolDefinition{
		Name:        "system_status",
		Description: "Check the live health of all forge plugins and providers. Use when the user asks why something isn't working or wants to know what is available.",
		Annotations: sdkplugins.ToolAnnotations{
			ReadOnly: true,
			CostHint: sdkplugins.ToolCostCheap,
		},
		Parameters: sdkplugins.ToolParameters{
			Type:       "object",
			Properties: map[string]sdkplugins.ToolProperty{},
		},
	}, s.execSystemStatus); err != nil {
		return fmt.Errorf("failed to register forge__system_status: %w", err)
	}

	if err := s.tools.RegisterTool("forge", sdkplugins.ToolDefinition{
		Name:        "plugin_health",
		Description: "Check the live health of a single named plugin. Use when system_status shows a degraded plugin and a targeted check is needed.",
		Annotations: sdkplugins.ToolAnnotations{
			ReadOnly: true,
			CostHint: sdkplugins.ToolCostCheap,
		},
		Parameters: sdkplugins.ToolParameters{
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]sdkplugins.ToolProperty{
				"name": {Type: "string", Description: "Plugin name as listed by system_status."},
			},
		},
	}, s.execPluginHealth); err != nil {
		return fmt.Errorf("failed to register forge__plugin_health: %w", err)
	}

	if err := s.tools.RegisterTool("forge", sdkplugins.ToolDefinition{
		Name:        "agent_info",
		Description: "Return static forge agent metadata: uptime, loaded plugins, and runtime. Use for capability questions without triggering health checks.",
		Annotations: sdkplugins.ToolAnnotations{
			ReadOnly: true,
			CostHint: sdkplugins.ToolCostCheap,
		},
		Parameters: sdkplugins.ToolParameters{
			Type:       "object",
			Properties: map[string]sdkplugins.ToolProperty{},
		},
	}, s.execAgentInfo); err != nil {
		return fmt.Errorf("failed to register forge__agent_info: %w", err)
	}

	return nil
}

func (s *SystemService) execSystemStatus(ctx context.Context, _ sdkplugins.ExecuteRequest) (*sdkplugins.ExecuteResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	entries, worst := fanOutHealth(ctx, s.plugins.ListDrivers())

	out := map[string]any{
		"status":  worst,
		"plugins": entries,
		"uptime":  time.Since(startTime).Round(time.Second).String(),
	}
	b, _ := json.Marshal(out)
	return &sdkplugins.ExecuteResponse{Result: string(b)}, nil
}

func (s *SystemService) execPluginHealth(ctx context.Context, req sdkplugins.ExecuteRequest) (*sdkplugins.ExecuteResponse, error) {
	name := req.Args.Get("name").StringOr("")
	if name == "" {
		return &sdkplugins.ExecuteResponse{Result: `{"error":"name is required"}`, IsError: true}, nil
	}

	driver, ok := s.plugins.GetDriver(name)
	if !ok {
		return &sdkplugins.ExecuteResponse{Result: fmt.Sprintf(`{"error":"plugin %q not found"}`, name), IsError: true}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	h, _ := driver.Driver.GetPluginHealth(ctx)
	entry := toHealthEntry(name, driver.Capabilities, h)
	b, _ := json.Marshal(entry)
	return &sdkplugins.ExecuteResponse{Result: string(b)}, nil
}

func (s *SystemService) execAgentInfo(_ context.Context, _ sdkplugins.ExecuteRequest) (*sdkplugins.ExecuteResponse, error) {
	drivers := s.plugins.ListDrivers()
	names := make([]string, 0, len(drivers))
	for _, d := range drivers {
		names = append(names, d.Info.Name)
	}

	out := map[string]any{
		"uptime":  time.Since(startTime).Round(time.Second).String(),
		"go":      runtime.Version(),
		"plugins": names,
	}
	b, _ := json.Marshal(out)
	return &sdkplugins.ExecuteResponse{Result: string(b)}, nil
}
