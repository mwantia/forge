package events

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/events"
	"github.com/mwantia/forge-sdk/pkg/api/v2/refs"
	"github.com/mwantia/forge-sdk/pkg/api/v2/sessions"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func EventsStatusCmd(client func() *v2.ForgeApi) *cobra.Command {
	var detailed bool

	cmd := &cobra.Command{
		Use:   "status [id]",
		Short: "Show event status and allocation history, or list all events",
		Long: "Without an argument, lists all configured event endpoints as a table.\n" +
			"With an event ID, shows the full status including the associated session,\n" +
			"queue state, and the webhook URL to use for external triggers.\n" +
			"Pass --detailed to include the full allocation history.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			if len(args) == 0 {
				eventsResp, err := c.Events.List(ctx, events.EventsListRequest{})
				if err != nil {
					return err
				}

				if len(eventsResp.Events) == 0 {
					fmt.Println("No events configured.")
					return nil
				}

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tSTATE\tSESSION\tLAST BRANCH\tDescription")
				for _, ev := range eventsResp.Events {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ev.ID, ev.State, ev.Session, ev.LastBranch, ev.Description)
				}

				return w.Flush()
			}

			evResp, err := c.Events.Get(ctx, events.EventsGetRequest{ID: args[0]})
			if err != nil {
				return err
			}
			ev := evResp.Event

			helpers.PrintEventStatus(ev)
			fmt.Println("\nSession:")

			if ev.Session != "" {
				sessResp, err := c.Sessions.Get(ctx, sessions.SessionsGetRequest{ID: ev.Session})
				if err != nil {
					return err
				}
				helpers.PrintSession(sessResp.Session, true)
			} else {
				fmt.Println("  No session configured")
				return nil
			}

			helpers.PrintEventOptions(ev)
			helpers.PrintEventQueue(ev)

			if detailed {
				prefix := "event/" + args[0] + "-"
				refsResp, err := c.Refs.List(ctx, refs.RefsListRequest{
					SessionID: ev.Session,
					Prefix:    prefix,
				})
				if err != nil {
					return err
				}

				branches := make([]events.EventBranch, 0, len(refsResp.Refs))
				for name, hash := range refsResp.Refs {
					eb := events.EventBranch{Name: name, Hash: hash}
					// Parse "event/<id>-<RFC3339>" to extract FiredAt
					if after, ok := strings.CutPrefix(name, prefix); ok {
						if t, err := time.Parse(time.RFC3339, after); err == nil {
							eb.FiredAt = t
						}
					}
					branches = append(branches, eb)
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
