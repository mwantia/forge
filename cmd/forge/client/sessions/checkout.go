package sessions

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func BranchCheckoutCmd(client func() *api.Client) *cobra.Command {
	var newBranch bool

	cmd := &cobra.Command{
		Use:   "checkout [-b] <session> <branch>",
		Short: "Switch HEAD to a branch; -b creates and switches in one step",
		Long: "Move HEAD to the named branch. With -b, the branch is created at the current\n" +
			"HEAD tip first, then HEAD is moved to it.\n\n" +
			"Equivalent to 'git checkout [-b] <branch>'.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()
			sessionID, branchName := args[0], args[1]

			if newBranch {
				refs, err := c.ListBranches(ctx, sessionID)
				if err != nil {
					return err
				}
				headHash, ok := refs["HEAD"]
				if !ok || headHash == "" {
					return fmt.Errorf("session has no HEAD")
				}
				if err := c.CreateBranch(ctx, sessionID, branchName, headHash); err != nil {
					return err
				}
			}

			return c.CheckoutBranch(ctx, sessionID, branchName)
		},
	}

	cmd.Flags().BoolVarP(&newBranch, "new-branch", "b", false, "Create and switch to a new branch")
	return cmd
}
