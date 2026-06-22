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

// formatTokens converts a raw token count to a compact human-readable string.
// 4218 → "4.2k", 200000 → "200k", 1500000 → "1.5M".
func formatTokens(n int) string {
	if n == 0 {
		return "0"
	}

	switch {
	case n >= 1_000_000:
		v := float64(n) / 1_000_000
		if v == float64(int(v)) {
			return fmt.Sprintf("%dM", int(v))
		}
		
		return fmt.Sprintf("%.1fM", v)
	
	case n >= 1_000:
		v := float64(n) / 1_000
		if v == float64(int(v)) {
			return fmt.Sprintf("%dk", int(v))
		}

		return fmt.Sprintf("%.1fk", v)
	
	default:
		return fmt.Sprintf("%d", n)
	}
}

// pressureClass returns the Tailwind text-colour token for context window pressure.
// <70% → text-ok (green), 70–90% → text-accent (amber), >90% → text-rem (red).
func pressureClass(used, limit int) string {
	if limit <= 0 {
		return "text-ink-2"
	}
	pct := used * 100 / limit
	switch {
	case pct >= 90:
		return "text-rem"
	case pct >= 70:
		return "text-accent"
	default:
		return "text-ok"
	}
}

// pressureBgClass returns the Tailwind bg-colour token matching pressureClass.
func pressureBgClass(used, limit int) string {
	if limit <= 0 {
		return "bg-ink-4"
	}
	pct := used * 100 / limit
	switch {
	case pct >= 90:
		return "bg-rem"
	case pct >= 70:
		return "bg-accent"
	default:
		return "bg-ok"
	}
}

// formatCost renders a float64 cost value to a compact dollar string with
// enough precision to show non-zero values (e.g. "$0.0002", "$1.23").
func formatCost(f float64) string {
	if f <= 0 {
		return "$0.00"
	}
	// Find first non-zero decimal place and show 2 more digits.
	for digits := 2; digits <= 8; digits++ {
		s := fmt.Sprintf("%.*f", digits, f)
		if s[len(s)-1] != '0' {
			return "$" + s
		}
	}
	return fmt.Sprintf("$%.8f", f)
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

// modePickerData returns the Alpine.js x-data expression for the mode selector.
// initialMode is the session's current effective mode; it pre-selects the matching pill.
func modePickerData(initialMode string) string {
	b, _ := json.Marshal(initialMode)
	return "{ activeMode: " + string(b) + " }"
}
