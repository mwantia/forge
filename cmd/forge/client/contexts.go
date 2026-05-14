package client

import (
	"encoding/json"
	"os"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/client/contexts"
	"github.com/spf13/cobra"
)

// NewContextsCommand builds the `forge contexts` command tree, the
// debugging surface for stored PromptContext objects (docs/03 §3.4).
func NewContextsCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "contexts",
		Short: "Inspect stored PromptContext objects",
		Long: "PromptContext objects record the exact provider, model, message hashes, and tool catalog hash\n" +
			"sent during a pipeline turn. Use these commands to inspect, materialize, or replay any recorded context.",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client {
		return api.New(httpAddr, httpToken)
	}

	cmd.AddCommand(contexts.ContextsReplayCmd(client))
	cmd.AddCommand(contexts.ContextsGetCmd(client))

	return cmd
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	return enc.Encode(v)
}
