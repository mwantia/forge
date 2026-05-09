package client

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/client/events"
	"github.com/spf13/cobra"
)

func NewEventsCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "events",
		Short: "Manage forge event endpoints",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client {
		return api.New(httpAddr, httpToken)
	}

	cmd.AddCommand(events.EventsListCmd(client))
	cmd.AddCommand(events.EventsGetCmd(client))
	cmd.AddCommand(events.EventsFireCmd(client))
	cmd.AddCommand(events.EventsPauseCmd(client))
	cmd.AddCommand(events.EventsResumeCmd(client))

	return cmd
}
