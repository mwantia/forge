package sessions

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func BranchCmd(client func() *api.Client) *cobra.Command {
	var rename string
	var delete bool

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
				// -m <old> <new>
				if len(args) < 2 {
					return fmt.Errorf("usage: branch <session> -m <old-name> <new-name>")
				}
				return c.RenameBranch(ctx, sessionID, rename, args[1])

			case delete:
				// -d <name>
				if len(args) < 2 {
					return fmt.Errorf("usage: branch <session> -d <name>")
				}
				return c.DeleteBranch(ctx, sessionID, args[1])

			case len(args) == 2:
				// create at HEAD
				refs, err := c.ListBranches(ctx, sessionID)
				if err != nil {
					return err
				}
				headHash, ok := refs["HEAD"]
				if !ok || headHash == "" {
					return fmt.Errorf("session has no HEAD")
				}
				return c.CreateBranch(ctx, sessionID, args[1], headHash)

			default:
				// list
				refs, symrefs, err := c.ListBranchesWithSymrefs(ctx, sessionID)
				if err != nil {
					return err
				}
				if len(refs) == 0 {
					fmt.Println("No branches.")
					return nil
				}
				names := make([]string, 0, len(refs))
				for n := range refs {
					names = append(names, n)
				}
				sort.Strings(names)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "REF\tHASH")
				for _, n := range names {
					label := n
					if target, ok := symrefs[n]; ok {
						label = n + " → " + target
					}
					fmt.Fprintf(w, "%s\t%s\n", label, helpers.FormatShortHash(refs[n]))
				}
				return w.Flush()
			}
		},
	}

	cmd.Flags().StringVarP(&rename, "move", "m", "", "Rename: -m <old-name> <new-name>")
	cmd.Flags().BoolVarP(&delete, "delete", "d", false, "Delete the named branch")
	return cmd
}
