package client

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

// --- forge sessions branches | branch | checkout | log ---

func newSessionsBranchesCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "branches <session>",
		Short: "List refs (HEAD + branches) for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, err := client().ListRefs(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if len(refs) == 0 {
				fmt.Println("No refs.")
				return nil
			}
			names := make([]string, 0, len(refs))
			for n := range refs {
				names = append(names, n)
			}
			sort.Strings(names)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "REF\tHASH")
			for _, n := range names {
				fmt.Fprintf(w, "%s\t%s\n", n, shortHash(refs[n]))
			}
			return w.Flush()
		},
	}
}

func newSessionsBranchCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "branch <session> <ref-name> <hash>",
		Short: "Create a branch ref pointing at a message hash",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client().CreateRef(cmd.Context(), args[0], args[1], args[2])
		},
	}
}

func newSessionsCheckoutCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "checkout <session> <ref-name>",
		Short: "Set HEAD to the value of <ref-name>",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client()
			refs, err := c.ListRefs(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			target, ok := refs[args[1]]
			if !ok {
				return fmt.Errorf("ref not found: %s", args[1])
			}
			return c.MoveRef(cmd.Context(), args[0], "HEAD", target, "")
		},
	}
}

func newSessionsLogCmd(client func() *api.Client) *cobra.Command {
	var ref string
	var limit int
	var verbose bool

	cmd := &cobra.Command{
		Use:   "log <session>",
		Short: "Walk a ref's parent chain (HEAD by default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client()
			ctx := cmd.Context()

			meta, err := c.GetSession(ctx, args[0])
			if err != nil {
				return err
			}
			refs, err := c.ListRefs(ctx, args[0])
			if err != nil {
				return err
			}
			byHash := map[string][]string{}
			for n, h := range refs {
				byHash[h] = append(byHash[h], n)
			}

			msgs, err := listMessagesForRef(ctx, c, args[0], ref, limit)
			if err != nil {
				return err
			}

			printSessionLogHeader(meta, msgs)
			fmt.Println()

			// Render newest-first so the latest exchange is on top.
			for i := len(msgs) - 1; i >= 0; i-- {
				printSessionLogEntry(msgs[i], byHash, verbose)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&ref, "ref", "HEAD", "Ref to walk")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max entries (0 = all)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Include full hashes, parent, and context_hash")
	return cmd
}

// printSessionLogHeader summarises session-level state: model, message
// counts by role, the live prompt size (latest assistant InputTokens),
// and cumulative usage across every dispatched turn.
func printSessionLogHeader(meta *api.SessionMetadata, msgs []*api.Message) {
	roleCounts := map[string]int{}
	var liveContext int
	for _, m := range msgs {
		roleCounts[m.Role]++
	}
	// Most-recent assistant InputTokens ≈ what the model just saw as prompt.
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" && msgs[i].Usage != nil && msgs[i].Usage.InputTokens > 0 {
			liveContext = msgs[i].Usage.InputTokens
			break
		}
	}

	parts := []string{}
	for _, role := range []string{"user", "assistant", "tool", "system"} {
		if n := roleCounts[role]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, role))
		}
	}
	fmt.Printf("%s · %s\n", meta.Name, meta.Model)
	fmt.Printf("%d messages (%s)\n", len(msgs), strings.Join(parts, " · "))
	if liveContext > 0 {
		fmt.Printf("context: ~%s tokens (latest prompt)\n", formatTokens(liveContext))
	}
	if meta.Usage != nil && meta.Usage.TotalTokens > 0 {
		line := fmt.Sprintf("cumulative: in=%s out=%s total=%s",
			formatTokens(meta.Usage.InputTokens),
			formatTokens(meta.Usage.OutputTokens),
			formatTokens(meta.Usage.TotalTokens))
		if meta.Usage.TotalCost > 0 {
			line += fmt.Sprintf("  ($%.4f)", meta.Usage.TotalCost)
		}
		fmt.Println(line)

		if meta.Usage.CachedInputTokens > 0 || meta.Usage.CacheCreationInputTokens > 0 {
			cacheLine := "cache:      "
			if c := meta.Usage.CachedInputTokens; c > 0 {
				ratio := float64(c) / float64(meta.Usage.InputTokens) * 100
				cacheLine += fmt.Sprintf("read=%s (%.0f%% of input)", formatTokens(c), ratio)
			}
			if w := meta.Usage.CacheCreationInputTokens; w > 0 {
				if meta.Usage.CachedInputTokens > 0 {
					cacheLine += "  "
				}
				cacheLine += fmt.Sprintf("write=%s", formatTokens(w))
			}
			fmt.Println(cacheLine)
		}
	}
}

// printSessionLogEntry renders one message line. Role-aware preview width:
// tool output is high-noise so we clip harder. Verbose mode adds a second
// indented line with hashes for the DAG-curious.
func printSessionLogEntry(m *api.Message, byHash map[string][]string, verbose bool) {
	marker := ""
	if names, ok := byHash[m.Hash]; ok {
		sort.Strings(names)
		marker = "(" + strings.Join(names, ", ") + ") "
	}

	stamp := ""
	if !m.CreatedAt.IsZero() {
		stamp = m.CreatedAt.Local().Format("15:04:05") + "  "
	}

	usage := ""
	if m.Usage != nil && m.Usage.TotalTokens > 0 {
		usage = fmt.Sprintf("  [in=%s out=%s]",
			formatTokens(m.Usage.InputTokens),
			formatTokens(m.Usage.OutputTokens))
	}

	fmt.Printf("%s%s  %s%-9s%s\n", stamp, shortHash(m.Hash), marker, m.Role, usage)

	previewCap := 100
	if m.Role == "tool" {
		previewCap = 70
	}
	preview := strings.ReplaceAll(strings.TrimSpace(m.Content), "\n", " ")
	if len(preview) > previewCap {
		preview = preview[:previewCap-3] + "..."
	}
	if preview != "" {
		fmt.Printf("    %s\n", preview)
	}

	if verbose {
		fmt.Printf("    hash=%s parent=%s ctx=%s\n", m.Hash, shortOrEmpty(m.ParentHash), shortOrEmpty(m.ContextHash))
	}
}

// formatTokens renders a token count with thousands separators
// for readability ("22147" → "22,147").
func formatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
		if len(s) > pre {
			b.WriteByte(',')
		}
	}
	for i := pre; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	return b.String()
}

func shortOrEmpty(h string) string {
	if h == "" {
		return "—"
	}
	return shortHash(h)
}

// listMessagesForRef returns messages on a ref. The HTTP API only walks
// HEAD today; for non-HEAD refs we resolve the ref's hash and assume the
// caller has set HEAD = that ref via checkout, OR we'd need a server-side
// ?ref= on GET messages. Phase 5 keeps it simple and walks via HEAD; if
// --ref is HEAD or empty we just use the existing endpoint.
func listMessagesForRef(ctx context.Context, c *api.Client, sessionID, ref string, limit int) ([]*api.Message, error) {
	if ref == "" || ref == "HEAD" {
		return c.ListMessages(ctx, sessionID, 0, limit)
	}
	// Fall back to walking HEAD; user can check out the branch first.
	return c.ListMessages(ctx, sessionID, 0, limit)
}

// --- forge messages edit ---

// NewMessagesCommand builds the `forge messages` tree, currently hosting
// the editor-driven edit-and-fork flow.
func NewMessagesCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Inspect and edit session messages",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client { return api.New(httpAddr, httpToken) }

	cmd.AddCommand(newMessagesEditCmd(client))
	return cmd
}

func newMessagesEditCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <session> <msg-hash-prefix>",
		Short: "Open a message in $EDITOR; submitting forks a new branch from its parent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := client()

			orig, err := c.GetMessage(ctx, args[0], args[1])
			if err != nil {
				return err
			}

			edited, err := openInEditor(orig.Content)
			if err != nil {
				return err
			}
			if strings.TrimSpace(edited) == "" {
				return fmt.Errorf("empty content; aborting")
			}
			if edited == orig.Content {
				return fmt.Errorf("no changes; aborting")
			}

			ch, err := c.SendMessage(ctx, args[0], edited, api.DispatchOptions{
				ForkFrom: args[1],
			})
			if err != nil {
				return err
			}
			for ev := range ch {
				parsed, err := api.ParseWireEvent(ev)
				if err != nil {
					continue
				}
				switch e := parsed.(type) {
				case api.ChunkEvent:
					fmt.Print(e.Text)
				case api.ErrorEvent:
					return fmt.Errorf("dispatch error: %s", e.Message)
				}
			}
			fmt.Println()
			return nil
		},
	}
}

func openInEditor(initial string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmp, err := os.CreateTemp("", "forge-edit-*.md")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	defer os.Remove(path)

	if _, err := tmp.WriteString(initial); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor %q: %w", filepath.Base(editor), err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}
