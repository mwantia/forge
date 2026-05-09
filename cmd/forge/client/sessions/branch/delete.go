package branch

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func BranchDeleteCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <session> <name>",
		Short: "Delete a branch ref",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client().DeleteBranch(cmd.Context(), args[0], args[1])
		},
	}
}
