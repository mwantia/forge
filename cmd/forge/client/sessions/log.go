package sessions

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsLogCmd(client func() *api.Client) *cobra.Command {
	var branch string
	var limit, offset int
	var verbose, detailed, table bool

	cmd := &cobra.Command{
		Use:   "log <session>",
		Short: "Walk a branch's parent chain (HEAD by default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client()
			ctx := cmd.Context()

			msgs, err := c.ListMessagesForRef(ctx, args[0], branch, limit)
			if err != nil {
				return err
			}

			if table {
				return helpers.PrintSessionLogTable(msgs[offset:])
			}

			meta, err := c.GetSession(ctx, args[0])
			if err != nil {
				return err
			}
			refs, err := c.ListBranches(ctx, args[0])
			if err != nil {
				return err
			}
			byHash := map[string][]string{}
			for n, h := range refs {
				byHash[h] = append(byHash[h], n)
			}

			helpers.PrintSessionLogHeader(meta, msgs)
			fmt.Println()

			for i := len(msgs) - 1; i >= 0; i-- {
				helpers.PrintSessionLogEntry(msgs[i], byHash, verbose, detailed)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch to walk (default HEAD)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max entries (0 = all)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of messages to skip (table mode only)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Include full hashes, parent, and context_hash")
	cmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Full content and args, no truncation; implies --verbose")
	cmd.Flags().BoolVar(&table, "table", false, "Render as a flat table instead of DAG view")
	return cmd
}
