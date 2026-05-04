package client

import (
	"fmt"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

// newSessionsSystemCmd builds the `forge sessions system` subgroup.
func newSessionsSystemCmd(client func() *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Manage the system message for a session",
	}
	cmd.AddCommand(newSystemShowCmd(client))
	cmd.AddCommand(newSystemEditCmd(client))
	cmd.AddCommand(newSystemRegenCmd(client))
	return cmd
}

func newSystemShowCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "show <session>",
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

func newSystemEditCmd(client func() *api.Client) *cobra.Command {
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

			edited, err := openInEditor(snap.Content)
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

func newSystemRegenCmd(client func() *api.Client) *cobra.Command {
	var systemPrompt string
	var toolsVerbosity string
	var plugins []string

	cmd := &cobra.Command{
		Use:   "regen <session>",
		Short: "Re-assemble the system message from current plugin state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			newHash, branch, err := c.RegenSystemSnapshot(ctx, args[0], systemPrompt, toolsVerbosity, plugins)
			if err != nil {
				return err
			}
			if branch != "" {
				fmt.Printf("System message regenerated: %s (fork branch: %s)\n", newHash, branch)
			} else {
				fmt.Printf("System message regenerated: %s\n", newHash)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&systemPrompt, "system", "", "Session-layer system prompt template (rendered and appended to the assembled prompt)")
	cmd.Flags().StringVar(&toolsVerbosity, "tools-verbosity", "", "Override tools verbosity for this regen (full|basic|none)")
	cmd.Flags().StringSliceVar(&plugins, "plugins", nil, "Plugin namespaces to include")
	return cmd
}
