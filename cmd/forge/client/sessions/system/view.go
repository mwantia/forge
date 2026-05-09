package system

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func SystemViewCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "view <session>",
		Short: "Print the current system message and its hash",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := client().GetSystemSnapshot(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if snap.Hash == "" {
				fmt.Println("No system message yet. Dispatch a message first.")
				return nil
			}
			fmt.Printf("Hash: %s\n\n", snap.Hash)
			fmt.Println(snap.Content)
			return nil
		},
	}
}
