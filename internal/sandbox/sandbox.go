package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/internal/config"
	reg "github.com/mwantia/forge/internal/plugins"
	"github.com/mwantia/forge/pkg/plugins"
)

// Sandbox provides a testing environment for plugins.
type Sandbox struct {
	log      hclog.Logger
	cfg      config.AgentConfig
	registry *reg.PluginRegistry
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
func NewSandbox(cfg config.AgentConfig) *Sandbox {
	log := hclog.Default().Named("sandbox")
	return &Sandbox{
		log:      log,
		cfg:      cfg,
		registry: reg.NewRegistry(log),
	}
}

func (s *Sandbox) Run(ctx context.Context, flags SandboxFlags) error {
	// Load configured plugins
	s.log.Debug("Loading configured plugins...")
	if err := s.registry.ServePlugins(ctx, s.cfg.PluginDir, s.cfg.Plugins); err != nil {
		s.log.Error("Plugins failed to load", "errors", err.Error())
	}

	parts := strings.SplitN(flags.Model, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid model format, expected '<provider>/<model>', got '%s'", flags.Model)
	}

	providerName := parts[0]
	modelName := parts[1]

	provider, err := s.registry.GetProviderPlugin(ctx, providerName)
	if err != nil {
		return fmt.Errorf("provider plugin not supported by '%s': %w", providerName, err)
	}

	s.log.Trace("Model", "name", modelName)

	// Collect tools from all loaded tools plugins.
	// toolsMap maps tool name → the plugin that owns it for dispatch during execution.
	toolsMap := make(map[string]plugins.ToolsPlugin)
	var availableTools []plugins.ToolCall

	for driverName, tp := range s.registry.GetAllToolsPlugins(ctx) {
		resp, err := tp.List(ctx)
		if err != nil {
			s.log.Warn("Failed to list tools from plugin", "driver", driverName, "error", err)
			continue
		}
		for _, def := range resp.Tools {
			availableTools = append(availableTools, plugins.ToolCall(def))
			toolsMap[def.Name] = tp
		}
	}

	s.log.Debug("Available tools", "count", len(availableTools))

	messages := []plugins.ChatMessage{
		{
			Role:    "user",
			Content: flags.Prompt,
		},
	}

	model := &plugins.Model{
		ModelName:   modelName,
		Temperature: 0.7,
	}

	defer s.Cleanup()
	maxIterations := 20
	for range maxIterations {
		s.log.Debug("Chat request", "model", modelName, "messages", len(messages))

		stream, err := provider.Chat(ctx, messages, availableTools, model)
		if err != nil {
			return fmt.Errorf("generation failed: %w", err)
		}

		// Drain the stream, printing deltas as they arrive.
		var role, content string
		toolCalls := make([]plugins.ChatToolCall, 0)
		for {
			chunk, err := stream.Recv()
			if err != nil {
				stream.Close()
				if err == io.EOF {
					break
				}
				return fmt.Errorf("stream error: %w", err)
			}
			if role == "" && chunk.Role != "" {
				role = chunk.Role
			}
			fmt.Printf("%s", chunk.Delta)

			content += chunk.Delta
			toolCalls = append(toolCalls, chunk.ToolCalls...)

			if chunk.Done {
				stream.Close()
				break
			}
		}
		fmt.Println() // newline after streamed output

		// Append the assistant turn, preserving any tool calls on the message.
		assistantMsg := plugins.ChatMessage{
			Role:    role,
			Content: content,
		}
		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = &plugins.ChatMessageToolCalls{
				ToolCalls: toolCalls,
			}
		}
		messages = append(messages, assistantMsg)

		if len(toolCalls) == 0 {

			return nil
		}

		// Execute each tool call and feed the results back as "tool" messages.
		for _, tc := range toolCalls {
			s.log.Debug("Executing tool call", "tool", tc.Name, "id", tc.ID)

			var resultContent string
			tp, ok := toolsMap[tc.Name]
			if !ok {
				resultContent = fmt.Sprintf("error: tool '%s' not found", tc.Name)
				s.log.Warn("Tool not found", "tool", tc.Name)
			} else {
				execResp, err := tp.Execute(ctx, plugins.ExecuteRequest{
					Tool:      tc.Name,
					Arguments: tc.Arguments,
					CallID:    tc.ID,
				})
				if err != nil {
					resultContent = fmt.Sprintf("error: %v", err)
					s.log.Warn("Tool execution error", "tool", tc.Name, "error", err)
				} else {
					b, _ := json.Marshal(execResp.Result)
					resultContent = string(b)
					if execResp.IsError {
						s.log.Warn("Tool returned error result", "tool", tc.Name, "result", resultContent)
					}
				}
			}

			messages = append(messages, plugins.ChatMessage{
				Role:    "tool",
				Content: resultContent,
			})
		}
	}

	return fmt.Errorf("max tool iterations reached")
}

// Close cleans up all loaded plugins.
func (s *Sandbox) Cleanup() {
	s.registry.CleanupDrivers()
}
