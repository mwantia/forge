package branch

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func BranchCreateCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "create <session> <name> <hash>",
		Short: "Create a branch ref pointing at a message hash",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client().CreateBranch(cmd.Context(), args[0], args[1], args[2])
		},
	}
}
