package events

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func EventsStatusCmd(client func() *api.Client) *cobra.Command {
	var detailed bool

	cmd := &cobra.Command{
		Use:   "status [id]",
		Short: "Show event status and allocation history, or list all events",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			if len(args) == 0 {
				events, err := c.ListEvents(ctx)
				if err != nil {
					return err
				}

				if len(events) == 0 {
					fmt.Println("No events configured.")
					return nil
				}

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tSTATE\tSESSION\tLAST BRANCH\tDescription")
				for _, ev := range events {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ev.ID, ev.State, ev.Session, ev.LastBranch, ev.Description)
				}

				return w.Flush()
			}

			ev, err := c.GetEvent(ctx, args[0])
			if err != nil {
				return err
			}

			helpers.PrintEventStatus(ev)
			fmt.Println("\nSession:")

			if ev.Session != "" {
				session, err := c.GetSession(ctx, ev.Session)
				if err != nil {
					return err
				}

				helpers.PrintSession(session)
			} else {
				fmt.Println("  No session configured")

				return nil
			}

			helpers.PrintEventOptions(ev)
			helpers.PrintEventQueue(ev)

			if detailed {
				branches, err := c.ListEventBranches(ctx, ev.Session, args[0])
				if err != nil {
					return err
				}

				fmt.Println("\nBranches:")
				if err := helpers.PrintEventBranches(branches); err != nil {
					return err
				}
			}

			fmt.Printf("\n==> Commit events by using the webhook: %s/v1/events/%s/fire\n", c.GetAddress(), ev.ID)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Show full allocation history")

	return cmd
}
