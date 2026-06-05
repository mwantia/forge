package approvals

import (
	"fmt"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	sdkapprovals "github.com/mwantia/forge-sdk/pkg/api/v2/approvals"
	"github.com/spf13/cobra"
)

func ApprovalsDenyCmd(client func() *v2.ForgeApi) *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "deny <id>",
		Short: "Deny a pending tool call",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			id := args[0]

			if err := c.Approvals.Respond(ctx, sdkapprovals.ApprovalsRespondRequest{
				ID:     id,
				Allow:  false,
				Reason: reason,
			}); err != nil {
				return err
			}

			fmt.Printf("✗ denied %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Reason for denial")

	return cmd
}
