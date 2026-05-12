package sessions

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func SessionsResetCmd(client func() *api.Client) *cobra.Command {
	var systemPrompt string
	var toolsVerbosity string
	var plugins []string

	cmd := &cobra.Command{
		Use:   "reset <session>",
		Short: "Re-assemble the system message from current plugin state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			newHash, branch, err := c.RegenSystemSnapshot(ctx, args[0], systemPrompt, toolsVerbosity, plugins)
			if err != nil {
				return err
			}
			if branch != "" {
				fmt.Printf("System message regenerated: %s (fork branch: %s)\n", newHash, branch)
			} else {
				fmt.Printf("System message regenerated: %s\n", newHash)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&systemPrompt, "system", "", "Session-layer system prompt template (rendered and appended)")
	cmd.Flags().StringVar(&toolsVerbosity, "tools-verbosity", "", "Override tools verbosity (full|basic|none)")
	cmd.Flags().StringSliceVar(&plugins, "plugins", nil, "Plugin namespaces to include")

	return cmd
}
