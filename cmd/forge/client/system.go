package client

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/client/system"
	"github.com/spf13/cobra"
)

func NewSystemCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "system",
		Short: "System operations for the forge agent",
		Long: "Low-level operations on the running forge agent: log monitoring,\n" +
			"object-store garbage collection, and DAG inspection tools.",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client {
		return api.New(httpAddr, httpToken)
	}

	cmd.AddCommand(system.SystemMonitorCmd(client))
	cmd.AddCommand(system.SystemGCCmd(client))
	cmd.AddCommand(system.SystemDagCmd(client))

	return cmd
}
