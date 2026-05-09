package events

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func EventsResumeCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <id>",
		Short: "Resume a paused event",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ev, err := client().ResumeEvent(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printEventStatus(ev)
		},
	}
}
