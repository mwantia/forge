package sessions

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsStatusCmd(client func() *api.Client) *cobra.Command {
	var limit, offset int
	var parent string
	var skipEmpty, archived, detailed bool

	cmd := &cobra.Command{
		Use:   "status <id>",
		Short: "Get session details or list all available sessions",
		Long: "Without an argument, lists all active sessions as a table. With a session ID\n" +
			"or name, prints the full metadata for that session: ID, model, token usage,\n" +
			"and timestamps. Use --archived to list archived sessions instead.",
		Args: cobra.MaximumNArgs(1),
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

				fmt.Fprintln(w, "ID\tNAME\tTITLE\tDESCRIPTION\tPLUGINS\tMODEL\tPARENT\tCREATED")

				for _, s := range sessions {
					plugins := strings.Join(s.Plugins, ",")
					if plugins == "" {
						plugins = "all"
					}
					createdAt := s.CreatedAt.Format(time.DateTime)

					fmt.Fprintf(w, "%s\t%s\t%.20s\t%.40s\t%s\t%s\t%s\t%s", s.ID, s.Name, s.Title, s.Description, plugins, s.Model, s.Parent, createdAt)

					fmt.Fprintln(w)
				}

				return w.Flush()
			}

			meta, err := c.GetSession(ctx, args[0])
			if err != nil {
				return err
			}

			helpers.PrintSession(meta, skipEmpty)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of sessions to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of sessions to skip")
	cmd.Flags().StringVar(&parent, "parent", "", "Filter by parent session ID")
	cmd.Flags().BoolVarP(&skipEmpty, "skip-empty", "s", false, "Skip parameters with empty values")
	cmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Full content and metadata, no truncation")
	cmd.Flags().BoolVar(&archived, "archived", false, "List archived sessions instead of active ones")

	return cmd
}
