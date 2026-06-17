package tmplsessions

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appsession "github.com/mwantia/forge/internal/application/session"
)

// sessionDisplayName returns the title when set, otherwise the name.
func sessionDisplayName(meta *appsession.SessionMetadata) string {
	if meta.Title != "" {
		return meta.Title
	}
	return meta.Name
}

func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

func fmtCreatedAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("Jan 2 15:04")
}

func millisTime(ms int64) string {
	d := time.Duration(ms) * time.Millisecond

	h := d / time.Hour
	d %= time.Hour

	m := d / time.Minute
	d %= time.Minute

	s := d / time.Second

	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if s > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}

	return strings.Join(parts, " ")
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2, 2006")
	}
}

func prettifyJSON(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return s
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return s
	}
	return string(b)
}

func formatArgs(args map[string]any) string {
	b, err := json.MarshalIndent(args, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func roleInitial(role string) string {
	if len(role) == 0 {
		return "?"
	}
	return strings.ToUpper(role[:1])
}

func pathRoleColor(role string) string {
	switch role {
	case "user":
		return "text-accent"
	case "assistant":
		return "text-ok"
	case "tool":
		return "text-ink-3"
	default:
		return "text-ink-4"
	}
}
