package approvals

import (
	"encoding/json"
	"fmt"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/spf13/cobra"
)

func ApprovalsWatchCmd(client func() *v2.ForgeApi) *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch approval events in real time (Ctrl-C to stop)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			ch, err := c.Approvals.Watch(ctx)
			if err != nil {
				return err
			}

			for rec := range ch {
				b, _ := json.Marshal(rec)
				fmt.Println(string(b))
			}

			return nil
		},
	}
}
