package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func NewSessionsCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage forge sessions",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client { return api.New(httpAddr, httpToken) }

	cmd.AddCommand(newSessionsListCmd(client))
	cmd.AddCommand(newSessionsCreateCmd(client))
	cmd.AddCommand(newSessionsGetCmd(client))
	cmd.AddCommand(newSessionsDeleteCmd(client))
	cmd.AddCommand(newSessionsMessagesCmd(client))
	cmd.AddCommand(newSessionsBranchesCmd(client))
	cmd.AddCommand(newSessionsBranchCmd(client))
	cmd.AddCommand(newSessionsCheckoutCmd(client))
	cmd.AddCommand(newSessionsLogCmd(client))

	return cmd
}

// NewPipelineCommand builds the `forge pipeline` command tree, currently
// hosting `dispatch` for sending a user message and streaming the response.
func NewPipelineCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Interact with the forge pipeline",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client { return api.New(httpAddr, httpToken) }

	cmd.AddCommand(newPipelineDispatchCmd(client))
	cmd.AddCommand(newPipelinePreviewCmd(client))

	return cmd
}

func newPipelinePreviewCmd(client func() *api.Client) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "preview <session-id> [content]",
		Short: "Preview the assembled prompt and chat history without calling the LLM",
		Long: "Returns the exact system prompt and message slice that would be " +
			"sent to the provider for the given session. The optional [content] " +
			"argument is appended as a tentative user message — it is NOT " +
			"persisted to the session.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := ""
			if len(args) == 2 {
				content = args[1]
			}
			resp, err := client().PreviewPipeline(cmd.Context(), args[0], content)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}

			accuracy := resp.EstAccuracy
			if accuracy == "" {
				accuracy = "±20%"
			}
			fmt.Printf("Session:    %s\n", resp.SessionID)
			fmt.Printf("Tool count: %d\n", resp.ToolCount)
			fmt.Printf("Total:      %d bytes, %d runes, ~%d tokens (%s)\n",
				resp.Total.Bytes, resp.Total.Runes, resp.Total.EstTokens, accuracy)
			fmt.Println()
			fmt.Printf("=== SYSTEM (%s) ===\n", formatUsage(resp.SystemUsage))
			if resp.System == "" {
				fmt.Println("(empty)")
			} else {
				fmt.Println(resp.System)
			}
			for _, m := range resp.Messages {
				fmt.Println()
				fmt.Printf("=== %s (%s) ===\n", strings.ToUpper(m.Role), formatUsage(m.Usage))
				if m.Content == "" {
					fmt.Println("(empty)")
				} else {
					fmt.Println(m.Content)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Print raw JSON response instead of human-readable layout")

	return cmd
}

func newPipelineDispatchCmd(client func() *api.Client) *cobra.Command {
	var raw, noRender, noStore bool

	cmd := &cobra.Command{
		Use:   "dispatch <session-id> <content>",
		Short: "Dispatch a user message to a session and stream the response",
		Long: "Dispatch a user message to a session and stream the response. " +
			"Pass `-` as <content> to read the message body from stdin " +
			"(e.g. `echo \"Hello\" | forge pipeline dispatch test-session -`).",
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
			return streamSend(cmd.Context(), client(), args[0], content, raw, noRender, noStore)
		},
	}

	cmd.Flags().BoolVar(&raw, "raw", false, "Bypass server chunking/pacing; print deltas as they arrive, no markdown rendering")
	cmd.Flags().BoolVar(&noRender, "no-render", false, "Print raw output without markdown rendering")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "Do not persist the generated messages to storage")

	return cmd
}

// --- sessions list ---

func newSessionsListCmd(client func() *api.Client) *cobra.Command {
	var limit, offset int
	var parent string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := client().ListSessions(cmd.Context(), parent, offset, limit)
			if err != nil {
				return err
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tMODEL\tCREATED")
			for _, s := range sessions {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					s.ID,
					s.Name,
					s.Model,
					s.CreatedAt.Format(time.DateTime),
				)
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of sessions to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of sessions to skip")
	cmd.Flags().StringVar(&parent, "parent", "", "Filter by parent session ID")

	return cmd
}

// --- sessions create ---

func newSessionsCreateCmd(client func() *api.Client) *cobra.Command {
	var (
		name              string
		model             string
		systemPrompt      string
		maxToolIterations int
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new session",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := api.CreateSessionRequest{
				Name:              name,
				Model:             model,
				MaxToolIterations: maxToolIterations,
			}
			if systemPrompt != "" {
				req.SystemPrompts = []api.SystemPromptEntry{{Name: "main", Content: systemPrompt}}
			}
			meta, err := client().CreateSession(cmd.Context(), req)
			if err != nil {
				return err
			}
			printSession(meta)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Session name (auto-generated if not set)")
	cmd.Flags().StringVar(&model, "model", "", "Model to use (format: provider/model)")
	cmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "System prompt for the session")
	cmd.Flags().IntVar(&maxToolIterations, "max-tool-iterations", 0, "Maximum tool call iterations (0 = default)")
	cmd.MarkFlagRequired("model")

	return cmd
}

// --- sessions get ---

func newSessionsGetCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get session details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := client().GetSession(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printSession(meta)
			return nil
		},
	}
}

// --- sessions delete ---

func newSessionsDeleteCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := client().DeleteSession(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Session %s deleted.\n", args[0])
			return nil
		},
	}
}

// --- sessions messages ---

func newSessionsMessagesCmd(client func() *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Manage messages in a session",
	}
	cmd.AddCommand(newSessionsMessagesListCmd(client))
	cmd.AddCommand(newSessionsMessagesViewCmd(client))
	cmd.AddCommand(newSessionsMessagesCompactCmd(client))
	return cmd
}

func newSessionsMessagesListCmd(client func() *api.Client) *cobra.Command {
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List messages in a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			messages, err := client().ListMessages(cmd.Context(), args[0], offset, limit)
			if err != nil {
				return err
			}

			if len(messages) == 0 {
				fmt.Println("No messages found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "HASH\tCREATED\tROLE\tCONTENT")
			for _, m := range messages {
				content := m.Content
				if len(content) > 80 {
					content = content[:77] + "..."
				}
				content = strings.ReplaceAll(content, "\n", " ")
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					shortHash(m.Hash),
					m.CreatedAt.Format(time.DateTime),
					m.Role,
					content,
				)
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of messages to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of messages to skip")

	return cmd
}

func newSessionsMessagesViewCmd(client func() *api.Client) *cobra.Command {
	var noRender bool

	cmd := &cobra.Command{
		Use:   "view <session-id> <message-id>",
		Short: "View the content of a single message",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			msg, err := client().GetMessage(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}

			var sb strings.Builder
			sb.WriteString("---\n")
			fmt.Fprintf(&sb, "Hash:    %s\n", msg.Hash)
			fmt.Fprintf(&sb, "Role:    %s\n", msg.Role)
			fmt.Fprintf(&sb, "Created: %s\n", msg.CreatedAt.Format(time.DateTime))
			if len(msg.ToolCalls) > 0 {
				fmt.Fprintf(&sb, "Tools:   %d\n", len(msg.ToolCalls))
			}
			sb.WriteString("---\n")
			if msg.Content != "" {
				sb.WriteString("\n")
				sb.WriteString(msg.Content)
			}
			fmt.Println(renderMarkdown(sb.String(), noRender))
			return nil
		},
	}

	cmd.Flags().BoolVar(&noRender, "no-render", false, "Print raw content without markdown rendering")
	return cmd
}

func newSessionsMessagesCompactCmd(client func() *api.Client) *cobra.Command {
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

// streamSend consumes the NDJSON ChunkEvent stream. Each chunk's Boundary
// dictates how to display it:
//   - token    → print raw (no rendering possible on partial tokens)
//   - sentence → print raw
//   - block    → render with glamour (self-contained markdown unit)
//   - final    → render with glamour (tail / whole response in final mode)
//
// --raw forces token-boundary streaming from the server and disables all
// rendering, matching pipe/programmatic use. --no-render keeps the server's
// chunking but skips glamour locally.
func streamSend(ctx context.Context, c *api.Client, sessionID, content string, raw, noRender, noStore bool) error {
	ch, err := c.SendMessage(ctx, sessionID, content, api.DispatchOptions{NoStore: noStore, Raw: raw})
	if err != nil {
		return err
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
					fmt.Print(renderMarkdown(e.Text, false))
				} else {
					fmt.Print(e.Text)
				}
			default: // token | sentence
				fmt.Print(e.Text)
			}
			printed = true
		case api.ErrorEvent:
			return fmt.Errorf("pipeline error: %s", e.Message)
		}
	}

	if printed {
		fmt.Println()
	}
	return nil
}

// --- helpers ---

func printSession(s *api.SessionMetadata) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID:\t%s\n", s.ID)
	fmt.Fprintf(w, "Name:\t%s\n", s.Name)
	fmt.Fprintf(w, "Model:\t%s\n", s.Model)
	for _, p := range s.SystemPrompts {
		label := p.Name
		if label == "" {
			label = "system"
		}
		preview := p.Content
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
		fmt.Fprintf(w, "Prompt [%s]:\t%s\n", label, preview)
	}
	fmt.Fprintf(w, "Created:\t%s\n", s.CreatedAt.Format(time.DateTime))
	fmt.Fprintf(w, "Updated:\t%s\n", s.UpdatedAt.Format(time.DateTime))
	w.Flush()
}

func formatUsage(u api.PreviewUsage) string {
	return fmt.Sprintf("%d bytes, %d runes, ~%d tokens", u.Bytes, u.Runes, u.EstTokens)
}

func renderMarkdown(content string, noRender bool) string {
	if noRender {
		return content
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	return out
}
