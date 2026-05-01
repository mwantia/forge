package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

// NewContextsCommand builds the `forge contexts` command tree, the
// debugging surface for stored PromptContext objects (docs/03 §3.4).
func NewContextsCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "contexts",
		Short: "Inspect stored PromptContext objects",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client { return api.New(httpAddr, httpToken) }

	cmd.AddCommand(newContextsShowCmd(client))
	return cmd
}

// NewReplayCommand builds the top-level `forge replay <hash>` command, a
// shortcut for re-dispatching a stored PromptContext.
func NewReplayCommand() *cobra.Command {
	var httpAddr, httpToken, model string

	cmd := &cobra.Command{
		Use:   "replay <hash>",
		Short: "Replay a stored PromptContext through a provider and stream the response",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := api.New(httpAddr, httpToken)
			return streamReplay(cmd.Context(), c, args[0], model)
		},
	}

	cmd.Flags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.Flags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")
	cmd.Flags().StringVar(&model, "model", "", "Override the recorded model alias (e.g. ollama/llama3.2)")

	return cmd
}

func newContextsShowCmd(client func() *api.Client) *cobra.Command {
	var materialized bool

	cmd := &cobra.Command{
		Use:   "show <hash>",
		Short: "Print a stored PromptContext (raw or materialized)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			if materialized {
				out, err := c.MaterializeContext(ctx, args[0])
				if err != nil {
					return err
				}
				return printJSON(out)
			}

			raw, err := c.GetContext(ctx, args[0])
			if err != nil {
				return err
			}
			return printJSON(raw)
		},
	}

	cmd.Flags().BoolVar(&materialized, "materialized", false, "Resolve message hashes into the full chat slice")
	return cmd
}

func streamReplay(ctx context.Context, c *api.Client, hash, model string) error {
	ch, err := c.ReplayContext(ctx, hash, model)
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
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
