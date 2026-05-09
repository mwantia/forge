package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mwantia/forge-sdk/pkg/api"
)

func PrintSession(s *api.SessionMetadata) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\t= %s\n", s.ID)
	fmt.Fprintf(w, "Name\t= %s\n", s.Name)
	fmt.Fprintf(w, "Model\t= %s\n", s.Model)
	fmt.Fprintf(w, "Created\t= %s\n", s.CreatedAt.Format(time.DateTime))
	fmt.Fprintf(w, "Updated\t= %s\n", s.UpdatedAt.Format(time.DateTime))
	w.Flush()
}

func PrintSessionLogTable(msgs []*api.Message) error {
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

		time := m.CreatedAt.Local().Format("2006-01-02 15:04:05")
		hash := m.Hash
		if len(hash) > 12 {
			hash = hash[:12]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", hash, time, role, tokens, content)
	}
	return w.Flush()
}

func PrintSessionLogHeader(meta *api.SessionMetadata, msgs []*api.Message) {
	roleCounts := map[string]int{}
	var liveContext int
	for _, m := range msgs {
		roleCounts[displayRole(m)]++
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" && msgs[i].Usage != nil && msgs[i].Usage.InputTokens > 0 {
			liveContext = msgs[i].Usage.InputTokens
			break
		}
	}

	parts := []string{}
	for _, role := range []string{"user", "assistant", "tool_call", "tool_result", "system"} {
		if n := roleCounts[role]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, role))
		}
	}
	fmt.Printf("%s · %s\n", meta.Name, meta.Model)
	fmt.Printf("%d messages (%s)\n", len(msgs), strings.Join(parts, " · "))
	if liveContext > 0 {
		fmt.Printf("context: ~%s tokens (latest prompt)\n", FormatTokens(liveContext))
	}
	if meta.Usage != nil && meta.Usage.TotalTokens > 0 {
		line := fmt.Sprintf("cumulative: in=%s out=%s total=%s",
			FormatTokens(meta.Usage.InputTokens),
			FormatTokens(meta.Usage.OutputTokens),
			FormatTokens(meta.Usage.TotalTokens))
		if meta.Usage.TotalCost > 0 {
			line += fmt.Sprintf("  ($%.4f)", meta.Usage.TotalCost)
		}
		fmt.Println(line)

		if meta.Usage.CachedInputTokens > 0 || meta.Usage.CacheCreationInputTokens > 0 {
			cacheLine := "cache:      "
			if c := meta.Usage.CachedInputTokens; c > 0 {
				ratio := float64(c) / float64(meta.Usage.InputTokens) * 100
				cacheLine += fmt.Sprintf("read=%s (%.0f%% of input)", FormatTokens(c), ratio)
			}
			if w := meta.Usage.CacheCreationInputTokens; w > 0 {
				if meta.Usage.CachedInputTokens > 0 {
					cacheLine += "  "
				}
				cacheLine += fmt.Sprintf("write=%s", FormatTokens(w))
			}
			fmt.Println(cacheLine)
		}
	}
}

func PrintSessionLogEntry(m *api.Message, byHash map[string][]string, verbose, detailed bool) {
	marker := ""
	if names, ok := byHash[m.Hash]; ok {
		sort.Strings(names)
		marker = "(" + strings.Join(names, ", ") + ") "
	}

	stamp := ""
	if !m.CreatedAt.IsZero() {
		stamp = m.CreatedAt.Local().Format("15:04:05") + "  "
	}

	hash := FormatShortHash(m.Hash)
	if detailed {
		hash = m.Hash
	}

	usage := ""
	if m.Usage != nil && m.Usage.TotalTokens > 0 {
		usage = fmt.Sprintf("  [in=%s out=%s]", FormatTokens(m.Usage.InputTokens), FormatTokens(m.Usage.OutputTokens))
	}

	role := displayRole(m)
	fmt.Printf("%s%s  %s%-12s%s\n", stamp, hash, marker, role, usage)

	switch role {
	case "tool_call":
		for _, tc := range m.ToolCalls {
			if detailed {
				fmt.Printf("    → %s\n", tc.Name)
				if len(tc.Arguments) > 0 {
					b, _ := json.MarshalIndent(tc.Arguments, "      ", "  ")
					fmt.Printf("      %s\n", b)
				}
			} else {
				line := "→ " + tc.Name
				if len(tc.Arguments) > 0 {
					line += formatArgPreview(tc.Arguments, 80-len(line))
				}
				fmt.Printf("    %s\n", line)
			}
		}
	case "tool_result":
		prefix := ""
		if len(m.ToolCalls) > 0 {
			prefix = "← " + m.ToolCalls[0].Name + "  "
		}
		if detailed {
			fmt.Printf("    %s\n", prefix)
			if m.Content != "" {
				for _, line := range strings.Split(strings.TrimSpace(m.Content), "\n") {
					fmt.Printf("    %s\n", line)
				}
			}
		} else {
			preview := strings.ReplaceAll(strings.TrimSpace(m.Content), "\n", " ")
			cap := 80 - len(prefix)
			if cap < 20 {
				cap = 20
			}
			if len(preview) > cap {
				preview = preview[:cap-3] + "..."
			}
			if preview != "" || prefix != "" {
				fmt.Printf("    %s%s\n", prefix, preview)
			}
		}
	default:
		if detailed {
			if m.Content != "" {
				for _, line := range strings.Split(strings.TrimSpace(m.Content), "\n") {
					fmt.Printf("    %s\n", line)
				}
			}
		} else {
			preview := strings.ReplaceAll(strings.TrimSpace(m.Content), "\n", " ")
			if len(preview) > 100 {
				preview = preview[:97] + "..."
			}
			if preview != "" {
				fmt.Printf("    %s\n", preview)
			}
		}
	}

	if verbose {
		fmt.Printf("    hash=%s parent=%s ctx=%s\n", m.Hash, FormatShortHash(m.ParentHash), FormatShortHash(m.ContextHash))
	}
	fmt.Println()
}

// displayRole maps stored roles to their display label.
// An assistant message that carries only tool calls (no text content) is shown
// as "tool_call"; tool-result messages are shown as "tool_result".
func displayRole(m *api.Message) string {
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

// formatArgPreview renders tool call arguments as a compact JSON snippet,
// capped at maxLen characters.
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
