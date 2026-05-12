package sessions

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsStatusCmd(client func() *api.Client) *cobra.Command {
	var limit, offset int
	var parent string
	var archived bool

	cmd := &cobra.Command{
		Use:   "status <id>",
		Short: "Get session details or list all available sessions",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			if len(args) == 0 {
				sessions, err := c.ListSessions(ctx, parent, archived, offset, limit)
				if err != nil {
					return err
				}

				if len(sessions) == 0 {
					fmt.Println("No sessions found.")
					return nil
				}

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tNAME\tMODEL\tCREATED")

				for _, s := range sessions {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						s.ID,
						s.Name,
						s.Model,
						s.CreatedAt.Format(time.DateTime),
					)
				}

				return w.Flush()
			}

			meta, err := c.GetSession(ctx, args[0])
			if err != nil {
				return err
			}

			helpers.PrintSession(meta)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of sessions to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of sessions to skip")
	cmd.Flags().StringVar(&parent, "parent", "", "Filter by parent session ID")
	cmd.Flags().BoolVar(&archived, "archived", false, "List archived sessions instead of active ones")

	return cmd
}
