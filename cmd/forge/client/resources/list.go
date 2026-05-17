package resources

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func ListCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list <path>",
		Short: "List resources at a path",
		Long: "List all resources stored at the given path.\n\n" +
			"Path examples:\n" +
			"  /forge/sessions/<id>/memories\n" +
			"  /forge/sessions/<id>/references\n" +
			"  /forge/global",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resources, err := client().ListResources(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if len(resources) == 0 {
				fmt.Println("No resources found.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSCORE\tTAGS\tPREVIEW")
			for _, r := range resources {
				preview := strings.ReplaceAll(r.Content, "\n", " ")
				if len(preview) > 60 {
					preview = preview[:57] + "..."
				}
				tags := strings.Join(r.Tags, ",")
				fmt.Fprintf(w, "%s\t%.2f\t%s\t%s\n", r.ID, r.Score, tags, preview)
			}
			return w.Flush()
		},
	}
}
