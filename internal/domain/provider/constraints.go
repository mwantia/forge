package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type constraintContextKey struct{}

// ConstraintContext carries the per-turn dispatch state used to evaluate
// candidate constraints inside an agent. Mode and Turns are injected by the
// pipeline before RunSessionPipeline; Size is computed from the live message
// list at the moment Chat is called.
type ConstraintContext struct {
	Size  int    // estimated token count of messages for this turn (chars/4)
	Mode  string // session mode: chat, coding, planning, research
	Turns int    // number of non-system messages in the current ref history
}

// WithConstraintContext stores cc in ctx so ProviderService.Chat can read it.
func WithConstraintContext(ctx context.Context, cc ConstraintContext) context.Context {
	return context.WithValue(ctx, constraintContextKey{}, cc)
}

// ConstraintContextFrom extracts the ConstraintContext stored by WithConstraintContext.
// Returns a zero-value ConstraintContext if none was set.
func ConstraintContextFrom(ctx context.Context) ConstraintContext {
	cc, _ := ctx.Value(constraintContextKey{}).(ConstraintContext)
	return cc
}

// resolveAttribute maps a plain dotted path (e.g. "context.size") to the
// corresponding field in cc. Returns an error for unknown paths.
func resolveAttribute(attr string, cc ConstraintContext) (any, error) {
	switch attr {
	case "context.size":
		return cc.Size, nil
	case "context.mode":
		return cc.Mode, nil
	case "context.turns":
		return cc.Turns, nil
	default:
		return nil, fmt.Errorf("unknown constraint attribute %q", attr)
	}
}

// evaluateConstraint checks a single constraint against cc.
// Unknown attributes and malformed values are treated as a failure (false)
// so the candidate is skipped rather than unexpectedly selected.
func evaluateConstraint(c *AgentConstraint, cc ConstraintContext) bool {
	val, err := resolveAttribute(c.Attribute, cc)
	if err != nil {
		return false
	}

	switch c.Operator {
	case "==":
		return fmt.Sprintf("%v", val) == c.Value
	case "!=":
		return fmt.Sprintf("%v", val) != c.Value

	case "contains":
		return strings.Contains(fmt.Sprintf("%v", val), c.Value)

	case "in":
		str := fmt.Sprintf("%v", val)
		for _, part := range strings.Split(c.Value, ",") {
			if strings.TrimSpace(part) == str {
				return true
			}
		}
		return false

	case "<", "<=", ">", ">=":
		intVal, ok := toIntValue(val)
		if !ok {
			return false
		}
		threshold, ok := parseThreshold(c.Value)
		if !ok {
			return false
		}
		switch c.Operator {
		case "<":
			return intVal < threshold
		case "<=":
			return intVal <= threshold
		case ">":
			return intVal > threshold
		case ">=":
			return intVal >= threshold
		}
	}

	return false
}

// EvaluateConstraints returns true only when every constraint in the slice
// passes. An empty slice (unconditional candidate) always returns true.
func EvaluateConstraints(constraints []*AgentConstraint, cc ConstraintContext) bool {
	for _, c := range constraints {
		if !evaluateConstraint(c, cc) {
			return false
		}
	}
	return true
}

// toIntValue coerces val to int when the underlying type is int or float64.
func toIntValue(val any) (int, bool) {
	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	}
	return 0, false
}

// parseThreshold parses a value string as either a size shorthand ("180k",
// "1m") or a plain integer. Returns (0, false) on parse failure.
func parseThreshold(s string) (int, bool) {
	if n := parseTokenCount(s); n > 0 {
		return n, true
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	return n, true
}
