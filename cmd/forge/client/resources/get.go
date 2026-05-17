package resources

import (
	"fmt"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func GetCmd(client func() *api.Client) *cobra.Command {
	var render bool

	cmd := &cobra.Command{
		Use:   "get <path> <name>",
		Short: "Fetch a single resource by name",
		Long: "Fetch and print the full content of a resource.\n" +
			"Metadata is printed as a YAML front-matter header.\n" +
			"Pass --render to apply markdown rendering to the content.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := client().GetResource(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}

			var sb strings.Builder
			sb.WriteString("---\n")
			fmt.Fprintf(&sb, "name: %s\n", r.ID)
			if len(r.Tags) > 0 {
				fmt.Fprintf(&sb, "tags: [%s]\n", strings.Join(r.Tags, ", "))
			}
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
				fmt.Println(helpers.RenderMarkdown(result, false))
			} else {
				fmt.Println(result)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&render, "render", false, "Apply markdown rendering to content")
	return cmd
}
