package system

import (
	"fmt"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SystemEditCmd(client func() *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <session>",
		Short: "Open the system message in $EDITOR and save the result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			snap, err := c.GetSystemSnapshot(ctx, args[0])
			if err != nil {
				return err
			}

			edited, err := helpers.OpenInEditor(snap.Content)
			if err != nil {
				return err
			}
			if strings.TrimSpace(edited) == "" {
				return fmt.Errorf("empty content; aborting")
			}
			if edited == snap.Content {
				return fmt.Errorf("no changes; aborting")
			}

			newHash, branch, err := c.EditSystemSnapshot(ctx, args[0], edited)
			if err != nil {
				return err
			}
			if branch != "" {
				fmt.Printf("System message updated: %s (fork branch: %s)\n", newHash, branch)
			} else {
				fmt.Printf("System message updated: %s\n", newHash)
			}
			return nil
		},
	}

	return cmd
}
