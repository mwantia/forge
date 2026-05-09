package messages

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func SessionsMessagesCompactCmd(client func() *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compact <id>",
		Short: "Compact messages in a session by removing tool call entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := client().CompactMessages(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Compacted: %d → %d messages (%d deleted)\n",
				result.Before, result.After, result.Deleted)
			return nil
		},
	}
	return cmd
}
