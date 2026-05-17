package resources

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func ForgetCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "forget <path> <name>",
		Short: "Delete a resource by name",
		Long: "Delete the resource with the given name at the given path.\n" +
			"The content object is retained in the DAG until a GC pass; only the ref is removed.\n\n" +
			"Example:\n" +
			"  forge resources forget /forge/sessions/<id>/memories dark-mode-pref",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := client().ForgetResource(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("forgotten: %s\n", args[1])
			return nil
		},
	}
}
