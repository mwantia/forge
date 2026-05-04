package client

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

// newSessionsBranchCmd builds the `forge sessions branch` subgroup.
func newSessionsBranchCmd(client func() *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Manage session branches",
	}
	cmd.AddCommand(newBranchListCmd(client))
	cmd.AddCommand(newBranchCreateCmd(client))
	cmd.AddCommand(newBranchCheckoutCmd(client))
	cmd.AddCommand(newBranchDeleteCmd(client))
	cmd.AddCommand(newBranchRenameCmd(client))
	return cmd
}

func newBranchListCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list <session>",
		Short: "List refs (HEAD + branches) for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, symrefs, err := client().ListBranchesWithSymrefs(cmd.Context(), args[0])
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
				label := n
				if target, ok := symrefs[n]; ok {
					label = n + " → " + target
				}
				fmt.Fprintf(w, "%s\t%s\n", label, shortHash(refs[n]))
			}
			return w.Flush()
		},
	}
}

func newBranchCreateCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "create <session> <name> <hash>",
		Short: "Create a branch ref pointing at a message hash",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client().CreateBranch(cmd.Context(), args[0], args[1], args[2])
		},
	}
}

func newBranchCheckoutCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "checkout <session> <branch>",
		Short: "Switch HEAD to a named branch (e.g. main, fork-abc)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client().CheckoutBranch(cmd.Context(), args[0], args[1])
		},
	}
}

func newBranchDeleteCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <session> <name>",
		Short: "Delete a branch ref",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client().DeleteBranch(cmd.Context(), args[0], args[1])
		},
	}
}

func newBranchRenameCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <session> <old-name> <new-name>",
		Short: "Rename a branch ref",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client().RenameBranch(cmd.Context(), args[0], args[1], args[2])
		},
	}
}

// newSessionsLogCmd builds `forge sessions log`.
func newSessionsLogCmd(client func() *api.Client) *cobra.Command {
	var ref string
	var limit, offset int
	var verbose, table bool

	cmd := &cobra.Command{
		Use:   "log <session>",
		Short: "Walk a ref's parent chain (HEAD by default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client()
			ctx := cmd.Context()

			msgs, err := listMessagesForRef(ctx, c, args[0], ref, limit)
			if err != nil {
				return err
			}

			if table {
				return printSessionLogTable(msgs[offset:])
			}

			meta, err := c.GetSession(ctx, args[0])
			if err != nil {
				return err
			}
			refs, err := c.ListBranches(ctx, args[0])
			if err != nil {
				return err
			}
			byHash := map[string][]string{}
			for n, h := range refs {
				byHash[h] = append(byHash[h], n)
			}

			printSessionLogHeader(meta, msgs)
			fmt.Println()

			for i := len(msgs) - 1; i >= 0; i-- {
				printSessionLogEntry(msgs[i], byHash, verbose)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&ref, "ref", "HEAD", "Ref to walk")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max entries (0 = all)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of messages to skip (table mode only)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Include full hashes, parent, and context_hash")
	cmd.Flags().BoolVar(&table, "table", false, "Render as a flat table instead of DAG view")
	return cmd
}

// printSessionLogHeader summarises session-level state.
func printSessionLogHeader(meta *api.SessionMetadata, msgs []*api.Message) {
	roleCounts := map[string]int{}
	var liveContext int
	for _, m := range msgs {
		roleCounts[m.Role]++
	}
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

func printSessionLogTable(msgs []*api.Message) error {
	if len(msgs) == 0 {
		fmt.Println("No messages found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "HASH\tCREATED\tROLE\tTOKENS\tCONTENT")
	for _, m := range msgs {
		tokens := ""
		if m.Usage != nil && m.Usage.TotalTokens > 0 {
			tokens = fmt.Sprintf("in=%s out=%s",
				formatTokens(m.Usage.InputTokens),
				formatTokens(m.Usage.OutputTokens))
		}
		content := strings.ReplaceAll(strings.TrimSpace(m.Content), "\n", " ")
		if len(content) > 80 {
			content = content[:77] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			shortHash(m.Hash),
			m.CreatedAt.Local().Format("2006-01-02 15:04:05"),
			m.Role,
			tokens,
			content,
		)
	}
	return w.Flush()
}

func listMessagesForRef(ctx context.Context, c *api.Client, sessionID, ref string, limit int) ([]*api.Message, error) {
	if ref == "" || ref == "HEAD" {
		return c.ListMessages(ctx, sessionID, 0, limit)
	}
	return c.ListMessages(ctx, sessionID, 0, limit)
}

func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}
