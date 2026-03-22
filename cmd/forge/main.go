package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/internal/agent"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/plugin"
	"github.com/mwantia/forge/internal/sandbox"
	"github.com/mwantia/forge/pkg/log"
	"github.com/mwantia/forge/pkg/plugins"
	_ "github.com/mwantia/forge/plugins/ollama" // Import to register ollama plugin
	_ "github.com/mwantia/forge/plugins/skills" // Import to register skills plugin
	_ "github.com/mwantia/forge/plugins/stub"   // Import to register stub plugin
	"github.com/spf13/cobra"
)

var (
	// Root flags
	PathFlag    string
	NoColorFlag bool
	ConfigFlag  *config.AgentConfig
	// Agent flags
	OnceFlag bool
	// Sandbox flags
	SandboxModelFlag       string
	SandboxTemperatureFlag float64
	SandboxMaxTokenFlag    int
)

func main() {
	cmd := &cobra.Command{
		Use:   "forge",
		Short: "System for forging",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Parse(PathFlag)
			if err != nil {
				return fmt.Errorf("unable to parse config: %w", err)
			}

			logger := hclog.New(&hclog.LoggerOptions{
				Name:        "forge",
				DisableTime: true,
				Level:       hclog.LevelFromString(cfg.LogLevel),
				Output:      log.LogWrapper(os.Stdout, !NoColorFlag),
				JSONFormat:  false,
			})
			hclog.SetDefault(logger)

			ConfigFlag = cfg
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&PathFlag, "path", "", "Defines the configuration path used by this application")
	cmd.PersistentFlags().BoolVar(&NoColorFlag, "no-color", false, "Disables colored command output")

	cmd.AddCommand(newAgentCommand())
	cmd.AddCommand(newPluginCommand())
	cmd.AddCommand(newSandboxCommand())

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run forge agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()

			return agent.NewAgent(*ConfigFlag).Serve(OnceFlag, ctx)
		},
	}

	cmd.Flags().BoolVar(&OnceFlag, "once", false, "Run agent once and exit immediately after startup tests")
	return cmd
}

func newPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin [name]",
		Short: "Serve a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			factory := plugins.Get(name)
			if factory == nil {
				return fmt.Errorf("unknown plugin: %s (available: %v)", name, plugins.Names())
			}

			plugins.Serve(factory)
			return nil
		},
	}

	return cmd
}

func newSandboxCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox --model <provider/model> <prompt>",
		Short: "Test plugins without running the full agent",
		Long: `Run a quick test of provider and tools plugins.

Example:
    forge sandbox --model ollama/llama2 "What is the capital of France?"
    forge sandbox --model ollama/llama2 --tools skills "Help me with this task"

Model format: <provider>/<model> (e.g., ollama/llama2, stub/test)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()

			sb := sandbox.New()
			defer sb.Close()

			// Build plugin configs from the loaded agent config
			pluginConfigs := make(map[string]map[string]any)
			for _, pc := range ConfigFlag.Plugins {
				if pc.Config != nil && pc.Config.Body != nil {
					cfgMap, err := plugin.ParseHclBody(pc.Config.Body)
					if err != nil {
						continue
					}
					pluginConfigs[pc.Name] = cfgMap
				}
			}

			if err := sb.LoadPlugins(ctx, ConfigFlag.PluginDir, pluginConfigs); err != nil {
				return fmt.Errorf("failed to load plugins: %w", err)
			}

			opts := sandbox.Options{
				Model:       SandboxModelFlag,
				Prompt:      args[0],
				Temperature: SandboxTemperatureFlag,
				MaxTokens:   SandboxMaxTokenFlag,
				Config:      pluginConfigs,
			}

			result, err := sb.Execute(ctx, opts)
			if err != nil {
				return fmt.Errorf("execution failed: %w", err)
			}

			// Output result
			fmt.Printf("\n--- Result ---\n")
			fmt.Printf("Provider: %s\n", result.Provider)
			fmt.Printf("Model:    %s\n", result.Model)
			fmt.Printf("Duration: %s\n", result.Duration)
			if result.TokensUsed > 0 {
				fmt.Printf("Tokens:   %d\n", result.TokensUsed)
			}
			fmt.Printf("\n%s\n", result.Content)

			return nil
		},
	}

	cmd.Flags().StringVar(&SandboxModelFlag, "model", "", "Model to use (format: provider/model, required)")
	cmd.Flags().Float64Var(&SandboxTemperatureFlag, "temperature", 0.7, "Temperature for generation")
	cmd.Flags().IntVar(&SandboxMaxTokenFlag, "max-tokens", 0, "Maximum tokens to generate (0 = unlimited)")

	cmd.MarkFlagRequired("model")

	return cmd
}
