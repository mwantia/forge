package sessions

import (
	"fmt"
	"strings"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/refs"
	"github.com/mwantia/forge-sdk/pkg/api/v2/sessions"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsEditCmd(client func() *v2.ForgeApi) *cobra.Command {
	var branch, editor string
	var noSwitch bool

	cmd := &cobra.Command{
		Use:   "edit <session> <msg-hash-prefix>",
		Short: "Open a message in $EDITOR and re-commit from its parent",
		Long: "Fetch the message at <msg-hash-prefix>, open its content in $EDITOR, then\n" +
			"commit the edited text using fork_from to branch off the message's parent.\n\n" +
			"The new branch is named edit-<8hex> by default (derived from the edited\n" +
			"message hash) and HEAD is switched to it automatically. Use --branch to\n" +
			"override the name, --no-switch to stay on the current branch, and\n" +
			"-e/--editor to override $EDITOR.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			origResp, err := c.Sessions.GetMessage(ctx, sessions.SessionsGetMessageRequest{
				SessionID: args[0],
				MessageID: args[1],
			})
			if err != nil {
				return err
			}
			orig := origResp.Message

			edited, err := helpers.OpenInEditorWith(orig.Content, editor)
			if err != nil {
				return err
			}
			if strings.TrimSpace(edited) == "" {
				return fmt.Errorf("empty content; aborting")
			}
			if edited == orig.Content {
				return fmt.Errorf("no changes; aborting")
			}

			activeRef, err := streamSend(ctx, c, args[0], edited, false, false, false, "", args[1])
			if err != nil {
				return err
			}

			finalBranch := branch
			if finalBranch == "" && len(orig.Hash) >= 8 {
				finalBranch = "edit-" + orig.Hash[:8]
			}

			if finalBranch != "" && activeRef != "" && activeRef != finalBranch {
				if _, err := c.Refs.Rename(ctx, refs.RefsRenameRequest{
					SessionID: args[0],
					Ref:       activeRef,
					Name:      finalBranch,
				}); err != nil {
					return fmt.Errorf("edit succeeded but branch rename failed: %w", err)
				}
				activeRef = finalBranch
			}

			if !noSwitch && activeRef != "" {
				if _, err := c.Refs.Checkout(ctx, refs.RefsCheckoutRequest{
					SessionID: args[0],
					Branch:    activeRef,
				}); err != nil {
					return fmt.Errorf("edit succeeded but checkout failed: %w", err)
				}
				fmt.Printf("Switched to branch %s\n", activeRef)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "", "Name for the new edit branch (default: edit-<8hex>)")
	cmd.Flags().StringVarP(&editor, "editor", "e", "", "Editor to use (overrides $EDITOR)")
	cmd.Flags().BoolVar(&noSwitch, "no-switch", false, "Do not switch HEAD to the new branch after edit")
	return cmd
}
