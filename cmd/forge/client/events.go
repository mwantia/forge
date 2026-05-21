package client

import (
	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge/cmd/forge/client/events"
	"github.com/spf13/cobra"
)

func NewEventsCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "events",
		Short: "Manage forge event endpoints",
		Long: "Event endpoints are named webhooks that external tools (cron, CI, monitoring)\n" +
			"can fire to trigger pipeline runs. Use these commands to inspect, pause,\n" +
			"resume, and manually fire event endpoints.",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *v2.ForgeApi {
		return v2.NewApi(httpAddr, httpToken)
	}

	cmd.AddCommand(events.EventsStatusCmd(client))
	cmd.AddCommand(events.EventsFireCmd(client))
	cmd.AddCommand(events.EventsPauseCmd(client))
	cmd.AddCommand(events.EventsResumeCmd(client))

	return cmd
}
