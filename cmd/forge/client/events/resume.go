package events

import (
	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/events"
	"github.com/spf13/cobra"
)

func EventsResumeCmd(client func() *v2.ForgeApi) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <id>",
		Short: "Resume a paused event",
		Long:  "Resume a previously paused event endpoint, restoring normal fire handling.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().Events.Resume(cmd.Context(), events.EventsResumeRequest{ID: args[0]})
			if err != nil {
				return err
			}
			return printEventStatus(resp.Event)
		},
	}
}
