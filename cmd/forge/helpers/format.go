package helpers

import (
	"fmt"
	"strings"
	"time"

	"github.com/mwantia/forge-sdk/pkg/api/v2/pipeline"
)

func FormatUsage(u pipeline.PreviewUsage) string {
	return fmt.Sprintf("%d bytes, %d runes, ~%d tokens", u.Bytes, u.Runes, u.EstTokens)
}

func FormatTime(t time.Time) string {
	d := time.Until(t)
	if d < 0 {
		return t.Local().Format(time.RFC3339) + " (expired)"
	}

	return fmt.Sprintf("%s (%s remaining)", t.Local().Format(time.RFC3339), d.Round(time.Second))
}

func FormatTokens(n int) string {
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

func FormatShortHash(h string) string {
	if h == "" {
		return "——"
	}

	if len(h) <= 12 {
		return h
	}

	return h[:12]
}
