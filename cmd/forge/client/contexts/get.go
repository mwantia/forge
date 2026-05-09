package contexts

import (
	"encoding/json"
	"os"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func ContextsGetCmd(client func() *api.Client) *cobra.Command {
	var materialized bool

	cmd := &cobra.Command{
		Use:   "get <hash>",
		Short: "Print a stored PromptContext (raw or materialized)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")

			if materialized {
				out, err := c.MaterializeContext(ctx, args[0])
				if err != nil {
					return err
				}

				return enc.Encode(out)
			}

			raw, err := c.GetContext(ctx, args[0])
			if err != nil {
				return err
			}

			return enc.Encode(raw)
		},
	}

	cmd.Flags().BoolVar(&materialized, "materialized", false, "Resolve message hashes into the full chat slice")
	return cmd
}
