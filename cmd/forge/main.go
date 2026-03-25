package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/cmd/forge/server"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/sandbox"
	wlog "github.com/mwantia/forge/pkg/log"
	_ "github.com/mwantia/forge/plugins/ollama"    // Import to register ollama plugin
	_ "github.com/mwantia/forge/plugins/skills"    // Import to register skills plugin
	_ "github.com/mwantia/forge/plugins/workspace" // Import to register workspace plugin
	"github.com/spf13/cobra"
)

var (
	// Root flags
	LogLevelFlag   string
	NoLogColorFlag bool
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
			logger := hclog.New(&hclog.LoggerOptions{
				Name:        "forge",
				DisableTime: true,
				Level:       hclog.LevelFromString(LogLevelFlag),
				Output:      wlog.LogWrapper(os.Stdout, !NoLogColorFlag),
				JSONFormat:  false,
			})
			hclog.SetDefault(logger)
			log.SetOutput(io.Discard)

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&LogLevelFlag, "log-level", "info", "Defines the threshold for the logger")
	cmd.PersistentFlags().BoolVar(&NoLogColorFlag, "no-color", false, "Disables colored command output")

	cmd.AddCommand(server.NewAgentCommand())
	cmd.AddCommand(server.NewPluginCommand())
	cmd.AddCommand(newSandboxCommand())

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
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

			return sandbox.NewSandbox(*config.NewDefaultAgentConfig()).Run(ctx, sandbox.SandboxFlags{
				Model:  SandboxModelFlag,
				Prompt: args[0],
			})

			/*pluginConfigs := make(map[string]map[string]any)


			result, err := sb.Execute(ctx, opts)
			if err != nil {
				return fmt.Errorf("execution failed: %w", err)
			}

			fmt.Printf("\n--- Result ---\n")
			fmt.Printf("Provider: %s\n", result.Provider)
			fmt.Printf("Model:    %s\n", result.Model)
			fmt.Printf("Duration: %s\n", result.Duration)
			if result.TokensUsed > 0 {
				fmt.Printf("Tokens:   %d\n", result.TokensUsed)
			}
			fmt.Printf("\n%s\n", result.Content)

			return nil*/
		},
	}

	cmd.Flags().StringVar(&SandboxModelFlag, "model", "", "Model to use (format: provider/model, required)")
	cmd.MarkFlagRequired("model")

	return cmd
}
