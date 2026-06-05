package approvals

import (
	"fmt"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	sdkapprovals "github.com/mwantia/forge-sdk/pkg/api/v2/approvals"
	"github.com/spf13/cobra"
)

func ApprovalsAllowCmd(client func() *v2.ForgeApi) *cobra.Command {
	return &cobra.Command{
		Use:   "allow <id>",
		Short: "Approve a pending tool call",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			id := args[0]

			rec, err := c.Approvals.Get(ctx, sdkapprovals.ApprovalsGetRequest{ID: id})
			if err != nil {
				return err
			}

			if err := c.Approvals.Respond(ctx, sdkapprovals.ApprovalsRespondRequest{
				ID:    id,
				Allow: true,
			}); err != nil {
				return err
			}

			fmt.Printf("✓ approved %s (%s)\n", id, rec.Approval.Title)
			return nil
		},
	}
}
