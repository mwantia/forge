package sessions

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsCreateCmd(client func() *api.Client) *cobra.Command {
	var (
		name              string
		model             string
		toolsVerbosity    string
		plugins           []string
		maxToolIterations int
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new session",
		Long: "Create a new session with the specified model.\n\n" +
			"The system prompt is assembled lazily on the first commit using the session's\n" +
			"tool verbosity and plugin settings. Pass --tools-verbosity and --plugins to\n" +
			"control what gets assembled into the system block.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			req := api.CreateSessionRequest{
				Name:              name,
				Model:             model,
				MaxToolIterations: maxToolIterations,
				ToolsVerbosity:    toolsVerbosity,
				Plugins:           plugins,
			}
			meta, err := client().CreateSession(ctx, req)
			if err != nil {
				return err
			}
			helpers.PrintSession(meta, true)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Session name (auto-generated if not set)")
	cmd.Flags().StringVar(&model, "model", "", "Model to use (format: provider/model)")
	cmd.Flags().StringVar(&toolsVerbosity, "tools-verbosity", "", "Tools verbosity for system assembly (full|basic|none)")
	cmd.Flags().StringSliceVar(&plugins, "plugins", nil, "Plugin namespaces to include in system assembly")
	cmd.Flags().IntVar(&maxToolIterations, "max-tool-iterations", 0, "Maximum tool call iterations (0 = default)")
	cmd.MarkFlagRequired("model")

	return cmd
}
