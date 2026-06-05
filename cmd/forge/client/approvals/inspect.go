package approvals

import (
	"encoding/json"
	"fmt"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	sdkapprovals "github.com/mwantia/forge-sdk/pkg/api/v2/approvals"
	"github.com/spf13/cobra"
)

func ApprovalsInspectCmd(client func() *v2.ForgeApi) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <id>",
		Short: "Inspect an approval by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			resp, err := c.Approvals.Get(ctx, sdkapprovals.ApprovalsGetRequest{ID: args[0]})
			if err != nil {
				return err
			}

			b, err := json.MarshalIndent(resp.Approval, "", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(b))
			return nil
		},
	}
}
