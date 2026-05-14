package events

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func EventsPauseCmd(client func() *api.Client) *cobra.Command {
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

			ev, err := c.PauseEvent(cmd.Context(), id)
			if err != nil {
				return err
			}
			if err := printEventStatus(ev); err != nil {
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
			ev, err = c.ResumeEvent(cmd.Context(), id)
			if err != nil {
				return fmt.Errorf("resume: %w", err)
			}
			return printEventStatus(ev)
		},
	}

	cmd.Flags().BoolVar(&hold, "hold", false, "Block until Ctrl+C then automatically resume")

	return cmd
}
