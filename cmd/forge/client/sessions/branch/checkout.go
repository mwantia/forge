package branch

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func BranchCheckoutCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "checkout <session> <branch>",
		Short: "Switch HEAD to a named branch (e.g. main, fork-abc)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client().CheckoutBranch(cmd.Context(), args[0], args[1])
		},
	}
}
