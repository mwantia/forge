package resources

import (
	"fmt"
	"strings"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/resources"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func GetCmd(client func() *v2.ForgeApi) *cobra.Command {
	var render bool

	cmd := &cobra.Command{
		Use:   "get <path> <name>",
		Short: "Fetch a single resource by name",
		Long: "Fetch and print the full content of a resource.\n" +
			"Metadata is printed as a YAML front-matter header.\n" +
			"Pass --render to apply markdown rendering to the content.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().Resources.Get(cmd.Context(), resources.ResourcesGetRequest{
				Path: args[0],
				Name: args[1],
			})
			if err != nil {
				return err
			}
			r := resp.Resource

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
