package client

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func NewResourceCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Inspect forge resources",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client { return api.New(httpAddr, httpToken) }

	cmd.AddCommand(newResourceListCmd(client))
	cmd.AddCommand(newResourcePreviewCmd(client))

	return cmd
}

func newResourceListCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list <namespace>",
		Short: "List resources in a namespace",
		Args:  cobra.ExactArgs(1),
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
			fmt.Fprintln(w, "ID\tPREVIEW")
			for _, r := range resources {
				preview := strings.ReplaceAll(r.Content, "\n", " ")
				if len(preview) > 80 {
					preview = preview[:77] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\n", r.ID, preview)
			}
			return w.Flush()
		},
	}
}

func newResourcePreviewCmd(client func() *api.Client) *cobra.Command {
	var render bool

	cmd := &cobra.Command{
		Use:   "preview <namespace> <id>",
		Short: "Preview a memory resource by id",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := client().GetResource(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}

			var sb strings.Builder
			sb.WriteString("---\n")

			fmt.Fprintf(&sb, "id: %s\n", r.ID)
			fmt.Fprintf(&sb, "score: %.2f\n", r.Score)

			if len(r.Metadata) > 0 {
				sb.WriteString("metadata:\n")
				for k, v := range r.Metadata {
					fmt.Fprintf(&sb, "  %s: %v\n", k, v)
				}
			}

			sb.WriteString("---\n\n")
			sb.WriteString(r.Content)

			result := sb.String()

			if render {
				markdown := renderMarkdown(result, false)
				fmt.Println(markdown)
			} else {
				fmt.Println(result)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&render, "render", false, "Print raw content without markdown rendering")
	return cmd
}
