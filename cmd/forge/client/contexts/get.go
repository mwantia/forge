package contexts

import (
	"encoding/json"
	"os"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/pipeline"
	"github.com/spf13/cobra"
)

func ContextsGetCmd(client func() *v2.ForgeApi) *cobra.Command {
	var materialized bool

	cmd := &cobra.Command{
		Use:   "get <hash>",
		Short: "Print a stored PromptContext (raw or materialized)",
		Long: "Fetch a stored PromptContext blob by its hash.\n\n" +
			"By default the raw JSON is printed. Pass --materialized to resolve all message hashes\n" +
			"into the full rendered chat slice, including the assembled system prompt.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")

			if materialized {
				resp, err := c.Pipeline.MaterializeContext(ctx, pipeline.PipelineMaterializeContextRequest{Hash: args[0]})
				if err != nil {
					return err
				}
				return enc.Encode(resp.Context)
			}

			resp, err := c.Pipeline.GetContext(ctx, pipeline.PipelineGetContextRequest{Hash: args[0]})
			if err != nil {
				return err
			}
			return enc.Encode(resp.Raw)
		},
	}

	cmd.Flags().BoolVar(&materialized, "materialized", false, "Resolve message hashes into the full chat slice")
	return cmd
}
