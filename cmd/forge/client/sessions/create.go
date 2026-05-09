package sessions

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsCreateCmd(client func() *api.Client) *cobra.Command {
	var (
		name              string
		model             string
		systemPrompt      string
		toolsVerbosity    string
		plugins           []string
		maxToolIterations int
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new session",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()
			req := api.CreateSessionRequest{
				Name:              name,
				Model:             model,
				MaxToolIterations: maxToolIterations,
			}
			meta, err := c.CreateSession(ctx, req)
			if err != nil {
				return err
			}
			// Assemble and store the initial system message via regen.
			if _, _, err := c.RegenSystemSnapshot(ctx, meta.ID, systemPrompt, toolsVerbosity, plugins); err != nil {
				return fmt.Errorf("session created but system init failed: %w", err)
			}
			helpers.PrintSession(meta)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Session name (auto-generated if not set)")
	cmd.Flags().StringVar(&model, "model", "", "Model to use (format: provider/model)")
	cmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "Session-layer system prompt template (template vars like ${session.id} are rendered)")
	cmd.Flags().StringVar(&toolsVerbosity, "tools-verbosity", "", "Tools verbosity for system assembly (full|basic|none)")
	cmd.Flags().StringSliceVar(&plugins, "plugins", nil, "Plugin namespaces to include in system assembly")
	cmd.Flags().IntVar(&maxToolIterations, "max-tool-iterations", 0, "Maximum tool call iterations (0 = default)")
	cmd.MarkFlagRequired("model")

	return cmd
}
