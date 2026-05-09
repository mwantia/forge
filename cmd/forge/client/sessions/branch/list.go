package branch

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func BranchListCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list <session>",
		Short: "List refs (HEAD + branches) for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, symrefs, err := client().ListBranchesWithSymrefs(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if len(refs) == 0 {
				fmt.Println("No refs.")
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
		},
	}
}
