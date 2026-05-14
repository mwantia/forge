package contexts

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func ContextsReplayCmd(client func() *api.Client) *cobra.Command {
	var httpAddr, httpToken, model string

	cmd := &cobra.Command{
		Use:   "replay <hash>",
		Short: "Replay a stored PromptContext through a provider and stream the response",
		Long: "Re-dispatch a stored PromptContext through the provider without persisting anything.\n\n" +
			"The original session is untouched. Useful for diffing model behaviour across providers\n" +
			"or reproducing a bug. Optionally override the recorded model with --model.\n" +
			"Output is streamed to stdout.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			ch, err := c.ReplayContext(ctx, args[0], model)
			if err != nil {
				return err
			}

			printed := false
			for ev := range ch {
				parsed, err := api.ParseWireEvent(ev)
				if err != nil {
					continue
				}

				switch e := parsed.(type) {
				case api.ChunkEvent:
					if e.Text == "" {
						continue
					}
					fmt.Print(e.Text)
					printed = true
				case api.ErrorEvent:
					return fmt.Errorf("replay error: %s", e.Message)
				}
			}

			if printed {
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.Flags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")
	cmd.Flags().StringVar(&model, "model", "", "Override the recorded model alias (e.g. ollama/llama3.2)")

	return cmd
}
