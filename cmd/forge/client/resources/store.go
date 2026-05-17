package resources

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func StoreCmd(client func() *api.Client) *cobra.Command {
	var name, content, file string
	var tags []string

	cmd := &cobra.Command{
		Use:   "store <path>",
		Short: "Store a resource at a path",
		Long: "Store a resource at the given path. Content is read from --content, --file, or stdin.\n\n" +
			"Examples:\n" +
			"  forge resources store /forge/sessions/<id>/memories --name goal --content \"ship v2 by Q3\"\n" +
			"  forge resources store /forge/global --name policy --file policy.md\n" +
			"  echo \"some text\" | forge resources store /forge/sessions/<id>/references --name api-notes",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := content

			if text == "" && file != "" {
				b, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				text = string(b)
			}

			if text == "" {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				text = strings.TrimRight(string(b), "\n")
			}

			if text == "" {
				return fmt.Errorf("no content: provide --content, --file, or pipe via stdin")
			}

			res, err := client().StoreResource(cmd.Context(), args[0], api.StoreResourceRequest{
				Name:    name,
				Content: text,
				Tags:    tags,
			})
			if err != nil {
				return err
			}

			fmt.Printf("stored: %s\n", res.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Human-readable name (ref key). Derived from content hash if omitted.")
	cmd.Flags().StringVarP(&content, "content", "c", "", "Content to store")
	cmd.Flags().StringVarP(&file, "file", "f", "", "Read content from file")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag to attach (repeatable: --tag foo --tag bar)")
	return cmd
}
