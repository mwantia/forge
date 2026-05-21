package client

import (
	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge/cmd/forge/client/resources"
	"github.com/spf13/cobra"
)

func NewResourceCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Manage forge resources",
		Long: "Resources are content-addressed entries in the built-in long-term store,\n" +
			"organized under a forge/ namespace tree:\n\n" +
			"  /forge/sessions/<id>/memories    — facts, preferences, decisions\n" +
			"  /forge/sessions/<id>/references  — cited docs, links, specs\n" +
			"  /forge/sessions/<id>/online      — fetched web content\n" +
			"  /forge/global                    — cross-session agent-wide facts\n\n" +
			"Memories share a single HNSW semantic graph across all sessions;\n" +
			"references and online content share another.",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Forge agent address (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token (env: FORGE_HTTP_TOKEN)")

	client := func() *v2.ForgeApi { return v2.NewApi(httpAddr, httpToken) }

	cmd.AddCommand(resources.LsCmd(client))
	cmd.AddCommand(resources.GetCmd(client))
	cmd.AddCommand(resources.StoreCmd(client))
	cmd.AddCommand(resources.RecallCmd(client))
	cmd.AddCommand(resources.ForgetCmd(client))
	cmd.AddCommand(resources.HistoryCmd(client))

	return cmd
}
