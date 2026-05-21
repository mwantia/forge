package sessions

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/refs"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func BranchCmd(client func() *v2.ForgeApi) *cobra.Command {
	var rename string
	var deleteBranch bool

	cmd := &cobra.Command{
		Use:   "branch <session> [name]",
		Short: "List, create, rename, or delete session branches",
		Long: `Manage session branches, git-style.

    branch <session>              list all branches
    branch <session> <name>       create branch at current HEAD
    branch <session> -m <old> <new>  rename a branch
    branch <session> -d <name>    delete a branch`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()
			sessionID := args[0]

			switch {
			case rename != "":
				if len(args) < 2 {
					return fmt.Errorf("usage: branch <session> -m <old-name> <new-name>")
				}
				_, err := c.Refs.Rename(ctx, refs.RefsRenameRequest{
					SessionID: sessionID,
					Ref:       rename,
					Name:      args[1],
				})
				return err

			case deleteBranch:
				if len(args) < 2 {
					return fmt.Errorf("usage: branch <session> -d <name>")
				}
				return c.Refs.Delete(ctx, refs.RefsDeleteRequest{
					SessionID: sessionID,
					Ref:       args[1],
				})

			case len(args) == 2:
				refsResp, err := c.Refs.List(ctx, refs.RefsListRequest{SessionID: sessionID})
				if err != nil {
					return err
				}
				headHash, ok := refsResp.Refs["HEAD"]
				if !ok || headHash == "" {
					return fmt.Errorf("session has no HEAD")
				}
				_, err = c.Refs.Create(ctx, refs.RefsCreateRequest{
					SessionID: sessionID,
					Name:      args[1],
					Hash:      headHash,
				})
				return err

			default:
				refsResp, err := c.Refs.List(ctx, refs.RefsListRequest{SessionID: sessionID})
				if err != nil {
					return err
				}
				if len(refsResp.Refs) == 0 {
					fmt.Println("No branches.")
					return nil
				}
				names := make([]string, 0, len(refsResp.Refs))
				for n := range refsResp.Refs {
					names = append(names, n)
				}
				sort.Strings(names)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "REF\tHASH")
				for _, n := range names {
					label := n
					if target, ok := refsResp.Symrefs[n]; ok {
						label = n + " → " + target
					}
					fmt.Fprintf(w, "%s\t%s\n", label, helpers.FormatShortHash(refsResp.Refs[n]))
				}
				return w.Flush()
			}
		},
	}

	cmd.Flags().StringVarP(&rename, "move", "m", "", "Rename: -m <old-name> <new-name>")
	cmd.Flags().BoolVarP(&deleteBranch, "delete", "d", false, "Delete the named branch")
	return cmd
}
