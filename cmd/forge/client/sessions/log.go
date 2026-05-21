package sessions

import (
	"fmt"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/refs"
	"github.com/mwantia/forge-sdk/pkg/api/v2/sessions"
	"github.com/mwantia/forge-sdk/pkg/api/v2/transport"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsLogCmd(client func() *v2.ForgeApi) *cobra.Command {
	var branch string
	var limit, offset int
	var verbose, detailed, table bool

	cmd := &cobra.Command{
		Use:   "log <session>",
		Short: "Walk a branch's parent chain (HEAD by default)",
		Long: "Walk a session's message chain from the tip of HEAD (or --branch) back to the root.\n\n" +
			"Pass --verbose for full hashes and context hashes, --detailed for full message\n" +
			"content without truncation. --table renders a flat table suitable for scripting.\n\n" +
			"<session> accepts an ID or name.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client()
			ctx := cmd.Context()

			msgsResp, err := c.Sessions.ListMessages(ctx, sessions.SessionsListMessagesRequest{
				SessionID:  args[0],
				Ref:        branch,
				Pagination: transport.Pagination{Limit: limit},
			})
			if err != nil {
				return err
			}
			msgs := msgsResp.Messages

			if table {
				return helpers.PrintSessionLogTable(msgs[offset:])
			}

			metaResp, err := c.Sessions.Get(ctx, sessions.SessionsGetRequest{ID: args[0]})
			if err != nil {
				return err
			}
			refsResp, err := c.Refs.List(ctx, refs.RefsListRequest{SessionID: args[0]})
			if err != nil {
				return err
			}
			byHash := map[string][]string{}
			for n, h := range refsResp.Refs {
				byHash[h] = append(byHash[h], n)
			}

			helpers.PrintSessionLogHeader(metaResp.Session, msgs)
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
