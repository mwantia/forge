package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	pluginloader "github.com/mwantia/forge/internal/plugin"
	"github.com/mwantia/forge/pkg/plugins"
)

// Sandbox provides a testing environment for plugins.
type Sandbox struct {
	log    hclog.Logger
	loader *pluginloader.Loader
}

// Result holds the result of a sandbox execution.
type Result struct {
	Content    string
	Model      string
	Provider   string
	Duration   time.Duration
	TokensUsed int
	ToolCalls  []ToolCall
}

// ToolCall represents a tool that was called during execution.
type ToolCall struct {
	Name      string
	Arguments map[string]any
	Result    any
}

// Options configures the sandbox execution.
type Options struct {
	Model       string
	Prompt      string
	Tools       []string
	MaxTokens   int
	Temperature float64
	Config      map[string]map[string]any
}

// New creates a new Sandbox instance.
func New() *Sandbox {
	log := hclog.Default().Named("sandbox")
	return &Sandbox{
		log:    log,
		loader: pluginloader.NewLoader(log),
	}
}

// LoadPlugins loads plugins from the specified directory.
func (s *Sandbox) LoadPlugins(ctx context.Context, pluginDir string, pluginConfigs map[string]map[string]any) error {
	results := s.loader.LoadAll(ctx, pluginDir, pluginConfigs)

	failed := 0
	for _, r := range results {
		if r.Error != nil {
			failed++
		}
	}

	s.log.Info("Loaded %d plugins (%d failed)", len(results)-failed, failed)
	return nil
}

// GetProvider returns the provider plugin for the given model string.
func (s *Sandbox) GetProvider(ctx context.Context, modelString string) (plugins.ProviderPlugin, string, error) {
	parts := strings.SplitN(modelString, "/", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("invalid model format, expected '<provider>/<model>', got '%s'", modelString)
	}

	providerName := parts[0]
	modelName := parts[1]

	driver := s.loader.GetDriver(providerName)
	if driver == nil {
		return nil, "", fmt.Errorf("provider '%s' not loaded (available: %v)", providerName, s.getDriverNames())
	}

	plugin, err := driver.GetProviderPlugin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("provider plugin not supported by '%s': %w", providerName, err)
	}
	if plugin == nil {
		return nil, "", fmt.Errorf("provider '%s' returned nil plugin without error", providerName)
	}

	return plugin, modelName, nil
}

// GetToolsPlugin returns a tools plugin by name.
func (s *Sandbox) GetToolsPlugin(ctx context.Context, name string) (plugins.ToolsPlugin, error) {
	driver := s.loader.GetDriver(name)
	if driver == nil {
		return nil, fmt.Errorf("tools plugin '%s' not loaded", name)
	}

	plugin, err := driver.GetToolsPlugin(ctx)
	if err != nil {
		return nil, fmt.Errorf("tools plugin not supported by '%s': %w", name, err)
	}

	return plugin, nil
}

// ListTools returns all tools from loaded tools plugins.
func (s *Sandbox) ListTools(ctx context.Context) (map[string][]plugins.ToolDefinition, error) {
	tools := make(map[string][]plugins.ToolDefinition)

	for name, driver := range s.loader.GetDrivers() {
		plugin, err := driver.GetToolsPlugin(ctx)
		if err != nil {
			continue
		}

		resp, err := plugin.List(ctx)
		if err != nil {
			s.log.Warn("Failed to list tools from '%s': %v", name, err)
			continue
		}

		tools[name] = resp.Tools
	}

	return tools, nil
}

// Execute runs a prompt through the specified model and provider.
// It handles tool calling by executing tools and continuing the conversation.
func (s *Sandbox) Execute(ctx context.Context, opts Options) (*Result, error) {
	start := time.Now()

	provider, modelName, err := s.GetProvider(ctx, opts.Model)
	if err != nil {
		return nil, err
	}

	// Collect messages starting with user prompt
	messages := []plugins.Message{{Role: "user", Content: opts.Prompt}}

	// Collect tools and build tool name -> plugin map
	var reqTools []plugins.Tool
	toolToPlugin := make(map[string]string)
	toolsPlugins := make(map[string]plugins.ToolsPlugin)

	// Collect tools from loaded plugins
	allTools, err := s.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	s.log.Debug("Available tools from plugins", "tools", allTools)

	// If no specific tools requested, use all loaded tools plugins
	toolPluginsToUse := opts.Tools
	if len(toolPluginsToUse) == 0 {
		for name := range allTools {
			toolPluginsToUse = append(toolPluginsToUse, name)
		}
	}

	for _, toolPlugin := range toolPluginsToUse {
		if tools, ok := allTools[toolPlugin]; ok {
			plugin, err := s.GetToolsPlugin(ctx, toolPlugin)
			if err != nil {
				continue
			}
			toolsPlugins[toolPlugin] = plugin
			for _, t := range tools {
				reqTools = append(reqTools, plugins.Tool{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				})
				toolToPlugin[t.Name] = toolPlugin

				// Debug: log each tool definition
				paramsJSON, _ := json.Marshal(t.Parameters)
				s.log.Debug("Tool definition", "name", t.Name, "description", t.Description, "parameters", string(paramsJSON))
			}
		}
	}

	result := &Result{
		Model:     modelName,
		ToolCalls: make([]ToolCall, 0),
	}
	if parts := strings.SplitN(opts.Model, "/", 2); len(parts) == 2 {
		result.Provider = parts[0]
	}

	// Tool execution loop
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		req := plugins.GenerateRequest{
			Model:       modelName,
			Messages:    messages,
			Temperature: opts.Temperature,
			MaxTokens:   opts.MaxTokens,
			Tools:       reqTools,
		}

		// Debug: log the request before sending
		s.log.Debug("Generate request", "model", modelName, "tools", len(reqTools), "messages", len(messages))
		for j, msg := range messages {
			s.log.Debug("Message", "index", j, "role", msg.Role, "content_len", len(msg.Content), "tool_calls", len(msg.ToolCalls))
		}
		for j, tool := range reqTools {
			paramsJSON, _ := json.Marshal(tool.Parameters)
			s.log.Debug("Tool in request", "index", j, "name", tool.Name, "parameters", string(paramsJSON))
		}

		resp, err := provider.Generate(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("generation failed: %w", err)
		}

		// Add assistant message to history (including tool calls if present)
		assistantMsg := plugins.Message{
			Role:    resp.Role,
			Content: resp.Content,
		}
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = resp.ToolCalls
		}
		messages = append(messages, assistantMsg)

		if resp.Usage != nil {
			result.TokensUsed += resp.Usage.InputTokens + resp.Usage.OutputTokens
		}

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 {
			result.Content = resp.Content
			result.Duration = time.Since(start)
			return result, nil
		}

		// Process tool calls
		for _, tc := range resp.ToolCalls {
			pluginName := toolToPlugin[tc.Name]
			plugin, ok := toolsPlugins[pluginName]
			if !ok {
				s.log.Warn("No plugin found for tool '%s'", tc.Name)
				messages = append(messages, plugins.Message{
					Role:       "tool",
					Content:    fmt.Sprintf(`{"error": "tool not found: %s"}`, tc.Name),
					ToolCallID: tc.ID,
					Name:       tc.Name,
				})
				continue
			}

			execResp, err := plugin.Execute(ctx, plugins.ExecuteRequest{
				Tool:      tc.Name,
				Arguments: tc.Arguments,
			})
			if err != nil {
				s.log.Warn("Tool '%s' execution failed: %v", tc.Name, err)
				messages = append(messages, plugins.Message{
					Role:       "tool",
					Content:    fmt.Sprintf(`{"error": %s}`, err.Error()),
					ToolCallID: tc.ID,
					Name:       tc.Name,
				})
				continue
			}

			// Record tool call
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				Name:      tc.Name,
				Arguments: tc.Arguments,
				Result:    execResp.Result,
			})

			// Append tool result as a message
			resultJSON, _ := json.Marshal(execResp.Result)
			messages = append(messages, plugins.Message{
				Role:       "tool",
				Content:    string(resultJSON),
				ToolCallID: tc.ID,
				Name:       tc.Name,
			})
		}
	}

	result.Content = "max tool iterations reached"
	result.Duration = time.Since(start)
	return result, nil
}

// Close cleans up all loaded plugins.
func (s *Sandbox) Close() {
	s.loader.Close()
}

func (s *Sandbox) getDriverNames() []string {
	names := make([]string, 0, len(s.loader.GetDrivers()))
	for name := range s.loader.GetDrivers() {
		names = append(names, name)
	}
	return names
}
