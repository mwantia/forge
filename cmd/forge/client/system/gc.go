package system

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func SystemGCCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "gc",
		Short: "Garbage-collect unreachable objects from the object store",
		Long:  "Walks every session ref, marks reachable objects, and sweeps the rest.\nThis is a destructive operation and cannot be undone.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := client().SystemGC(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Printf("total: %d  kept: %d  swept: %d\n", result.Total, result.Kept, result.Swept)
			return nil
		},
	}
}
