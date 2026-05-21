package sessions

import (
	"fmt"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/refs"
	"github.com/spf13/cobra"
)

func BranchCheckoutCmd(client func() *v2.ForgeApi) *cobra.Command {
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
				refsResp, err := c.Refs.List(ctx, refs.RefsListRequest{SessionID: sessionID})
				if err != nil {
					return err
				}
				headHash, ok := refsResp.Refs["HEAD"]
				if !ok || headHash == "" {
					return fmt.Errorf("session has no HEAD")
				}
				if _, err := c.Refs.Create(ctx, refs.RefsCreateRequest{
					SessionID: sessionID,
					Name:      branchName,
					Hash:      headHash,
				}); err != nil {
					return err
				}
			}

			_, err := c.Refs.Checkout(ctx, refs.RefsCheckoutRequest{
				SessionID: sessionID,
				Branch:    branchName,
			})
			return err
		},
	}

	cmd.Flags().BoolVarP(&newBranch, "new-branch", "b", false, "Create and switch to a new branch")
	return cmd
}
