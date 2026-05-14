package sessions

import (
	"fmt"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsEditCmd(client func() *api.Client) *cobra.Command {
	var branch string

	cmd := &cobra.Command{
		Use:   "edit <session> <msg-hash-prefix>",
		Short: "Open a message in $EDITOR and re-commit from its parent",
		Long: "Fetch the message at <msg-hash-prefix>, open its content in $EDITOR, then\n" +
			"commit the edited text using fork_from to branch off the message's parent.\n\n" +
			"The auto-created fork-* branch can be renamed with --branch.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			orig, err := c.GetMessage(ctx, args[0], args[1])
			if err != nil {
				return err
			}

			edited, err := helpers.OpenInEditor(orig.Content)
			if err != nil {
				return err
			}
			if strings.TrimSpace(edited) == "" {
				return fmt.Errorf("empty content; aborting")
			}
			if edited == orig.Content {
				return fmt.Errorf("no changes; aborting")
			}

			activeRef, events, err := c.SendMessage(ctx, args[0], edited, api.CommitOptions{
				ForkFrom: args[1],
			})
			if err != nil {
				return err
			}
			for ev := range events {
				parsed, err := api.ParseWireEvent(ev)
				if err != nil {
					continue
				}
				switch e := parsed.(type) {
				case api.ChunkEvent:
					fmt.Print(e.Text)
				case api.ErrorEvent:
					return fmt.Errorf("commit error: %s", e.Message)
				}
			}
			fmt.Println()

			if branch != "" && activeRef != "" && activeRef != branch {
				if err := c.RenameBranch(ctx, args[0], activeRef, branch); err != nil {
					return fmt.Errorf("edit succeeded but branch rename failed: %w", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "", "Rename the auto-created fork-* ref to this name after commit")
	return cmd
}
