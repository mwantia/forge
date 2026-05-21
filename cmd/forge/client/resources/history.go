package resources

import (
	"fmt"
	"os"
	"text/tabwriter"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/resources"
	"github.com/spf13/cobra"
)

func HistoryCmd(client func() *v2.ForgeApi) *cobra.Command {
	return &cobra.Command{
		Use:   "history <path> <name>",
		Short: "Show revision history for a resource",
		Long: "Walk the parent-hash chain for a named resource, showing each revision's\n" +
			"hash, timestamp, and index metadata.\n\n" +
			"Example:\n" +
			"  forge resources history /forge/sessions/<id>/memories dark-mode-pref",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().Resources.History(cmd.Context(), resources.ResourcesHistoryRequest{
				Path: args[0],
				Name: args[1],
			})
			if err != nil {
				return err
			}
			if len(resp.History) == 0 {
				fmt.Println("No history found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "#\tHASH\tCREATED\tINDEXED BY")
			for i, rev := range resp.History {
				short := rev.Hash
				if len(short) > 12 {
					short = short[:12]
				}
				indexedBy := rev.IndexedBy
				if indexedBy == "" {
					indexedBy = "-"
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i, short, rev.CreatedAt.Format("2006-01-02 15:04:05"), indexedBy)
			}
			return w.Flush()
		},
	}
}
