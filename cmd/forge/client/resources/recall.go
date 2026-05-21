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

func RecallCmd(client func() *v2.ForgeApi) *cobra.Command {
	var query string
	var limit int
	var tags []string

	cmd := &cobra.Command{
		Use:   "recall <path>",
		Short: "Search resources by content query and filters",
		Long: "Search resources at the given path using content query, tags, and metadata filters.\n" +
			"When embed_model is configured on the agent, uses HNSW semantic search for exact paths.\n\n" +
			"Examples:\n" +
			"  forge resources recall /forge/sessions/<id>/memories --query \"project deadline\"\n" +
			"  forge resources recall /forge/global --query \"auth decisions\" --tag decision --limit 10",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().Resources.Recall(cmd.Context(), resources.ResourcesRecallRequest{
				Path:  args[0],
				Query: query,
				Tags:  tags,
				Limit: limit,
			})
			if err != nil {
				return err
			}
			if len(resp.Resources) == 0 {
				fmt.Println("No matching resources.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SCORE\tNAME\tTAGS\tPREVIEW")
			for _, r := range resp.Resources {
				preview := strings.ReplaceAll(r.Content, "\n", " ")
				if len(preview) > 60 {
					preview = preview[:57] + "..."
				}
				fmt.Fprintf(w, "%.2f\t%s\t%s\t%s\n", r.Score, r.ID, strings.Join(r.Tags, ","), preview)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "Content search query")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum results to return")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag filter — must carry all listed tags (repeatable)")
	return cmd
}
