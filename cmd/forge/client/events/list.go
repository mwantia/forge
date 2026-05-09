package events

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func EventsListCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured events",
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := client().ListEvents(cmd.Context())
			if err != nil {
				return err
			}

			if len(events) == 0 {
				fmt.Println("No events configured.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSESSION\tDESCRIPTION")
			for _, ev := range events {
				fmt.Fprintf(w, "%s\t%s\t%s\n", ev.ID, ev.Session, ev.Description)
			}

			return w.Flush()
		},
	}
}
