package events

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/events"
	"github.com/spf13/cobra"
)

func EventsPauseCmd(client func() *v2.ForgeApi) *cobra.Command {
	var hold bool

	cmd := &cobra.Command{
		Use:   "pause <id>",
		Short: "Pause an event (fires return 503 while paused)",
		Long: "Pause an event endpoint so that incoming fire requests return 503.\n\n" +
			"With --hold, the command blocks until Ctrl+C, then automatically resumes the endpoint.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			c := client()

			resp, err := c.Events.Pause(cmd.Context(), events.EventsPauseRequest{ID: id})
			if err != nil {
				return err
			}
			if err := printEventStatus(resp.Event); err != nil {
				return err
			}

			if !hold {
				return nil
			}

			fmt.Fprintln(os.Stderr, "\nEvent paused. Press Ctrl+C to resume.")

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			<-sig
			signal.Stop(sig)

			fmt.Fprintln(os.Stderr, "\nResuming event...")
			resumeResp, err := c.Events.Resume(cmd.Context(), events.EventsResumeRequest{ID: id})
			if err != nil {
				return fmt.Errorf("resume: %w", err)
			}
			return printEventStatus(resumeResp.Event)
		},
	}

	cmd.Flags().BoolVar(&hold, "hold", false, "Block until Ctrl+C then automatically resume")

	return cmd
}
