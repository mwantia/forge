package events

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func EventsGetCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get event details and live queue state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ev, err := client().GetEvent(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			return printEventStatus(ev)
		},
	}
}
