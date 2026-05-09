package helpers

import (
	"fmt"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/api"
)

func FormatUsage(u api.PreviewUsage) string {
	return fmt.Sprintf("%d bytes, %d runes, ~%d tokens", u.Bytes, u.Runes, u.EstTokens)
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
