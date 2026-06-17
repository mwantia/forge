package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mwantia/forge-sdk/pkg/api/v2/events"
	"github.com/mwantia/forge-sdk/pkg/api/v2/sessions"
)

const (
	colorReset  = "\033[0m"
	colorWhite  = "\033[97m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
)

func roleColor(role string) string {
	switch role {
	case "system":
		return colorBlue
	case "assistant":
		return colorCyan
	case "user":
		return colorGreen
	case "tool_result", "tool_call":
		return colorYellow
	default:
		return ""
	}
}

func PrintEventStatus(ev events.EventStatus) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\t= %s\n", ev.ID)
	fmt.Fprintf(w, "State\t= %s\n", ev.State)
	if ev.LastBranch != "" {
		fmt.Fprintf(w, "Last Branch\t= %s\n", ev.LastBranch)
	}
	if ev.Description != "" {
		fmt.Fprintf(w, "Description\t= %s\n", ev.Description)
	}

	return w.Flush()
}

func PrintEventOptions(ev events.EventStatus) error {
	if ev.Options == nil {
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options")
	if ev.Options.Timespan != "" {
		fmt.Fprintf(w, "  Timespan\t= %s\n", ev.Options.Timespan)
	}
	if ev.Options.MaxQueue > 0 {
		fmt.Fprintf(w, "  Max Queue\t= %d\n", ev.Options.MaxQueue)
	}

	return w.Flush()
}

func PrintEventQueue(ev events.EventStatus) error {
	if ev.Queue == nil {
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Queue")
	fmt.Fprintf(w, "  Size\t= %d\n", ev.Queue.Size)
	if ev.Queue.WindowExpiresAt != nil {
		fmt.Fprintf(w, "  Window Expires\t= %s\n", FormatTime(*ev.Queue.WindowExpiresAt))
	}

	return w.Flush()
}

func PrintEventBranches(branches []events.EventBranch) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if len(branches) > 0 {
		fmt.Fprintf(w, "%-55s\t%-21s\t%s\n", "NAME", "FIRED AT", "HASH")
		for _, b := range branches {
			hash := b.Hash
			if len(hash) > 8 {
				hash = hash[:8]
			}

			fmt.Fprintf(w, "%-55s\t%-21s\t%s\n", b.Name, b.FiredAt.Local().Format(time.DateTime), hash)
		}
	} else {
		fmt.Fprintln(w, "  No branches allocated")
	}

	return w.Flush()
}

func PrintSession(meta sessions.SessionMetadata, skipEmpty bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	plugins := strings.Join(meta.Plugins, ",")
	if plugins == "" {
		plugins = "all"
	}

	fmt.Fprintf(w, "ID\t= %s\n", meta.ID)
	fmt.Fprintf(w, "Name\t= %s\n", meta.Name)
	if skipEmpty && meta.Title != "" {
		fmt.Fprintf(w, "Title\t= %s\n", meta.Title)
	}
	if skipEmpty && meta.Description != "" {
		fmt.Fprintf(w, "Description\t= %s\n", meta.Description)
	}
	fmt.Fprintf(w, "Plugins\t= %s\n", plugins)
	fmt.Fprintf(w, "Model\t= %s\n", meta.Model)
	if skipEmpty && meta.Parent != "" {
		fmt.Fprintf(w, "Parent\t= %s\n", meta.Parent)
	}
	fmt.Fprintf(w, "Created\t= %s\n", meta.CreatedAt.Format(time.DateTime))
	fmt.Fprintf(w, "Updated\t= %s\n", meta.UpdatedAt.Format(time.DateTime))

	if meta.CurrentContextTokens > 0 {
		line := fmt.Sprintf("  Context\t= %s tokens", FormatTokens(meta.CurrentContextTokens))
		if meta.ContextWindowSize > 0 {
			pct := meta.CurrentContextTokens * 100 / meta.ContextWindowSize
			line += fmt.Sprintf(" / %s (%d%%)", FormatTokens(meta.ContextWindowSize), pct)
		}
		fmt.Fprintln(w, line)
	}

	return w.Flush()
}

func PrintSessionLogTable(msgs []sessions.Message) error {
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
				FormatTokens(m.Usage.InputTokens),
				FormatTokens(m.Usage.OutputTokens))
		}
		role := displayRole(m)
		content := strings.ReplaceAll(strings.TrimSpace(m.Content), "\n", " ")
		if role == "tool_call" && content == "" && len(m.ToolCalls) > 0 {
			names := make([]string, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				names = append(names, tc.Name)
			}
			content = "→ " + strings.Join(names, ", ")
		}
		if len(content) > 80 {
			content = content[:77] + "..."
		}

		t := m.CreatedAt.Local().Format("2006-01-02 15:04:05")
		hash := m.Hash
		if len(hash) > 12 {
			hash = hash[:12]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", hash, t, role, tokens, content)
	}
	return w.Flush()
}

func PrintSessionLogHeader(meta sessions.SessionMetadata, msgs []sessions.Message) {
	roleCounts := map[string]int{}
	for _, m := range msgs {
		roleCounts[displayRole(m)]++
	}

	parts := []string{}
	for _, role := range []string{"user", "assistant", "tool_call", "tool_result", "system"} {
		if n := roleCounts[role]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, role))
		}
	}
	fmt.Printf("%s · %s\n", meta.Name, meta.Model)
	fmt.Printf("%d messages (%s)\n", len(msgs), strings.Join(parts, " · "))
	if meta.CurrentContextTokens > 0 {
		line := fmt.Sprintf("context:    %s tokens", FormatTokens(meta.CurrentContextTokens))
		if meta.ContextWindowSize > 0 {
			pct := meta.CurrentContextTokens * 100 / meta.ContextWindowSize
			line += fmt.Sprintf(" / %s (%d%%)", FormatTokens(meta.ContextWindowSize), pct)
		}
		fmt.Println(line)
	}
}

func PrintSessionLogEntry(m sessions.Message, byHash map[string][]string, verbose, detailed bool) {
	role := displayRole(m)

	hash := FormatShortHash(m.Hash)
	if detailed || verbose {
		hash = m.Hash
	}
	branchStr := ""
	if names, ok := byHash[m.Hash]; ok && len(names) > 0 {
		sort.Strings(names)
		branchStr = fmt.Sprintf(" (%s%s%s)", colorGreen, strings.Join(names, ", "), colorReset)
	}

	fmt.Printf("%smessage %s%s%s\n", colorWhite, hash, colorReset, branchStr)

	fmt.Printf("Role:   %s%s%s\n", roleColor(role), role, colorReset)
	if !m.CreatedAt.IsZero() {
		fmt.Printf("Date:   %s\n", m.CreatedAt.Local().Format("Mon Jan 02 15:04:05 2006 -0700"))
	}

	if m.Usage != nil && m.Usage.TotalTokens > 0 {
		fmt.Printf("Tokens: in=%s out=%s\n", FormatTokens(m.Usage.InputTokens), FormatTokens(m.Usage.OutputTokens))
	}

	if verbose {
		fmt.Printf("Parent: %s\n", m.ParentHash)
		if m.ContextHash != "" {
			fmt.Printf("Ctx:    %s\n", m.ContextHash)
		}
	}

	fmt.Println()

	var lines []string
	switch role {
	case "tool_call":
		for _, tc := range m.ToolCalls {
			line := "→ " + tc.Name
			if len(tc.Arguments) > 0 {
				line += formatArgPreview(tc.Arguments, 80-len(line))
			}

			lines = append(lines, line)
			if detailed && len(tc.Arguments) > 0 {
				b, _ := json.MarshalIndent(tc.Arguments, "  ", "  ")
				for _, al := range strings.Split(string(b), "\n") {
					lines = append(lines, "  "+al)
				}
			}
		}

	case "tool_result":
		if m.Content != "" {
			var buf bytes.Buffer
			if err := json.Compact(&buf, []byte(m.Content)); err == nil {
				s := buf.String()
				if !detailed && len(s) > 160 {
					lines = []string{s[:157] + fmt.Sprintf("... (%d more characters)", len(s)-157)}
				} else {
					lines = []string{s}
				}
			} else {
				lines = strings.Split(strings.TrimSpace(m.Content), "\n")
			}
		}

	default:
		if m.Content != "" {
			lines = strings.Split(strings.TrimSpace(m.Content), "\n")
		}
	}

	const maxLines = 6
	shown := lines
	truncated := 0
	if !detailed && len(lines) > maxLines {
		shown = lines[:maxLines]
		truncated = len(lines) - maxLines
	}
	for _, l := range shown {
		fmt.Printf("    %s\n", l)
	}
	if truncated > 0 {
		fmt.Printf("    ... (%d more lines)\n", truncated)
	}

	fmt.Println()
}

func displayRole(m sessions.Message) string {
	switch m.Role {
	case "assistant":
		if m.Content == "" && len(m.ToolCalls) > 0 {
			return "tool_call"
		}
		return "assistant"
	case "tool":
		return "tool_result"
	default:
		return m.Role
	}
}

func formatArgPreview(args map[string]any, maxLen int) string {
	if len(args) == 0 {
		return "()"
	}
	parts := make([]string, 0, len(args))
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(parts)
	s := "(" + strings.Join(parts, ", ") + ")"
	if maxLen > 0 && len(s) > maxLen {
		s = s[:maxLen-1] + "…"
	}
	return s
}
