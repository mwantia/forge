package pipeline

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	appsession "github.com/mwantia/forge/internal/application/session"
	domresource "github.com/mwantia/forge/internal/domain/resource"
)

// buildRecallHint runs the budget-limited, recent-first recall pipeline over
// history and returns a rendered Markdown section to append to the system
// prompt. Returns "" when nothing surfaces above threshold or budget is 0.
//
// history is the persisted slice as of the current commit, excluding the new
// user message. The system message (history[0]) is skipped by the role filter.
func (s *PipelineService) buildRecallHint(ctx context.Context, history []*appsession.Message) string {
	cfg := s.config.Recall

	// budget=0 means explicitly disabled.
	budget := cfg.GetBudget()
	if budget == 0 {
		return ""
	}

	threshold := cfg.GetThreshold()
	minLength := cfg.GetMinLength()

	type hit struct {
		msgHashPrefix string
		resource      *domresource.Resource
	}

	var hits []hit
	seen := make(map[string]struct{})

	// Walk recent-first: highest index → lowest.
	for i := len(history) - 1; i >= 0 && budget > 0; i-- {
		msg := history[i]

		// Only user messages carry query value.
		if msg.Role != "user" {
			continue
		}

		// Pre-filter: skip short, non-question filler.
		runes := utf8.RuneCountInString(msg.Content)
		if runes < minLength && !strings.Contains(msg.Content, "?") {
			continue
		}

		results, err := s.resources.Recall(ctx, domresource.RecallQuery{
			Query: msg.Content,
			Limit: budget * 2,
		})
		if err != nil {
			s.logger.Warn("recall hint query failed", "msg_hash", msg.Hash, "error", err)
			continue
		}

		var above []*domresource.Resource
		for _, r := range results {
			if threshold <= 0 || r.Score >= threshold {
				above = append(above, r)
			}
		}

		if len(above) == 0 {
			continue
		}

		// This message consumed budget.
		budget--

		prefix := msg.Hash
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}

		for _, r := range above {
			if _, dup := seen[r.ID]; dup {
				continue
			}
			seen[r.ID] = struct{}{}
			hits = append(hits, hit{msgHashPrefix: prefix, resource: r})
		}
	}

	if len(hits) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Relevant Resources (recall)\n")
	for _, h := range hits {
		fmt.Fprintf(&sb, "\n<!-- from: %s score: %.2f -->\n## %s\n\n%s\n",
			h.msgHashPrefix, h.resource.Score, h.resource.ID, h.resource.Content)
	}
	return sb.String()
}
