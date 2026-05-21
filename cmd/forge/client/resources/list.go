package resources

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/resources"
	"github.com/spf13/cobra"
)

func LsCmd(client func() *v2.ForgeApi) *cobra.Command {
	return &cobra.Command{
		Use:   "ls [path]",
		Short: "List resources or virtual directories at a path",
		Long: "List resources stored at the given path, or virtual subdirectories when\n" +
			"the path has no direct resources.\n\n" +
			"Path examples:\n" +
			"  forge resources ls /              — top-level directories\n" +
			"  forge resources ls /forge         — subdirectories under forge\n" +
			"  forge resources ls /forge/sessions/<id>/memories  — actual resources\n\n" +
			"'.' and '/' both refer to the root.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/"
			if len(args) == 1 {
				path = args[0]
			}
			if path == "." || path == "" {
				path = "/"
			}

			resp, err := client().Resources.List(cmd.Context(), resources.ResourcesListRequest{Path: path})
			if err != nil {
				return err
			}
			if len(resp.Resources) == 0 {
				fmt.Println("No entries found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for _, r := range resp.Resources {
				if r.Type == "dir" {
					fmt.Fprintf(w, "%s/\n", r.ID)
				} else {
					preview := strings.ReplaceAll(r.Content, "\n", " ")
					if len(preview) > 60 {
						preview = preview[:57] + "..."
					}
					tags := strings.Join(r.Tags, ",")
					fmt.Fprintf(w, "%s\t%s\t%s\n", r.ID, tags, preview)
				}
			}
			return w.Flush()
		},
	}
}
