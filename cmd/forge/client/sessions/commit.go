package sessions

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsCommitCmd(client func() *api.Client) *cobra.Command {
	var raw, noRender, noStore bool
	var ref, forkFrom, branch string

	cmd := &cobra.Command{
		Use:   "commit <session-id> <content>",
		Short: "Commit a user message to a session and stream the response",
		Long: "Commit a user message to a session and stream the response. " +
			"Pass `-` as <content> to read the message body from stdin.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := args[1]
			if content == "-" {
				buf, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				content = strings.TrimRight(string(buf), "\n")
				if content == "" {
					return fmt.Errorf("stdin produced empty message content")
				}
			}
			activeRef, err := streamSend(cmd.Context(), client(), args[0], content, raw, noRender, noStore, ref, forkFrom)
			if err != nil {
				return err
			}
			if branch != "" && activeRef != "" && activeRef != branch {
				if err := client().RenameBranch(cmd.Context(), args[0], activeRef, branch); err != nil {
					return fmt.Errorf("commit succeeded but branch rename failed: %w", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&raw, "raw", false, "Bypass server chunking/pacing; print deltas as they arrive, no markdown rendering")
	cmd.Flags().BoolVar(&noRender, "no-render", false, "Print raw output without markdown rendering")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "Do not persist the generated messages to storage")
	cmd.Flags().StringVar(&ref, "ref", "", "Commit against a named branch")
	cmd.Flags().StringVar(&forkFrom, "fork-from", "", "Fork off a message hash before committing")
	cmd.Flags().StringVar(&branch, "branch", "", "Rename the auto-created fork-* ref to this name after commit")

	return cmd
}

func streamSend(ctx context.Context, c *api.Client, sessionID, content string, raw, noRender, noStore bool, ref, forkFrom string) (string, error) {
	activeRef, ch, err := c.SendMessage(ctx, sessionID, content, api.CommitOptions{
		NoStore:  noStore,
		Raw:      raw,
		Ref:      ref,
		ForkFrom: forkFrom,
	})
	if err != nil {
		return "", err
	}

	render := !raw && !noRender
	printed := false

	for ev := range ch {
		parsed, err := api.ParseWireEvent(ev)
		if err != nil {
			continue
		}
		switch e := parsed.(type) {
		case api.ChunkEvent:
			if e.Text == "" {
				continue
			}
			switch e.Boundary {
			case api.ChunkBoundaryBlock, api.ChunkBoundaryFinal:
				if render {
					fmt.Print(helpers.RenderMarkdown(e.Text, false))
				} else {
					fmt.Print(e.Text)
				}
			default:
				fmt.Print(e.Text)
			}
			printed = true
		case api.ErrorEvent:
			return "", fmt.Errorf("pipeline error: %s", e.Message)
		}
	}

	if printed {
		fmt.Println()
	}
	return activeRef, nil
}
