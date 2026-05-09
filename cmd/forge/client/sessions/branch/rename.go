package branch

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func BranchRenameCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <session> <old-name> <new-name>",
		Short: "Rename a branch ref",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client().RenameBranch(cmd.Context(), args[0], args[1], args[2])
		},
	}
}
