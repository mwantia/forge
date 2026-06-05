package approvals

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	sdkapprovals "github.com/mwantia/forge-sdk/pkg/api/v2/approvals"
	"github.com/spf13/cobra"
)

func ApprovalsListCmd(client func() *v2.ForgeApi) *cobra.Command {
	var status, plugin string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List approvals",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			resp, err := c.Approvals.List(ctx, sdkapprovals.ApprovalsListRequest{
				Status: status,
				Plugin: plugin,
			})
			if err != nil {
				return err
			}

			if len(resp.Approvals) == 0 {
				fmt.Println("No approvals found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTYPE\tTITLE\tSTATUS\tAGE")
			for _, a := range resp.Approvals {
				age := fmt.Sprintf("%ds", int(time.Since(a.CreatedAt).Seconds()))
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", a.ID, a.Type, a.Title, a.Status, age)
			}

			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&status, "status", "pending", "Filter by status: pending|all|resolved|denied")
	cmd.Flags().StringVar(&plugin, "plugin", "", "Filter by plugin name")

	return cmd
}
