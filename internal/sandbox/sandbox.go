package sandbox

import (
	"context"
	"fmt"
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

	maxIterations := 10
	for range maxIterations {
		s.log.Debug("Chat request", "model", modelName, "messages", len(messages))

		resp, err := provider.Chat(ctx, messages, nil, model)
		if err != nil {
			return fmt.Errorf("generation failed: %w", err)
		}

		messages = append(messages, plugins.ChatMessage{
			Role:    resp.Role,
			Content: resp.Content,
		})

		if len(resp.ToolCalls) == 0 {
			fmt.Println(resp.Content)
			s.Cleanup()
			return nil
		}
	}

	s.Cleanup()
	return fmt.Errorf("max tool iterations reached")
}

// Close cleans up all loaded plugins.
func (s *Sandbox) Cleanup() {
	s.registry.CleanupDrivers()
}
