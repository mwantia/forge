package provider

import (
	"strconv"
	"strings"
)

const (
	ModelTypeChat  = "chat"
	ModelTypeEmbed = "embed"
)

// ProviderConfig holds all declared model aliases.
type ProviderConfig struct {
	Models []*ProviderModelTemplate `hcl:"model,block"`
}

// ProviderModelTemplate defines a named model alias within forge.
type ProviderModelTemplate struct {
	Name               string                `hcl:"name,label"`
	Type               string                `hcl:"type,optional"`
	BaseModel          string                `hcl:"base_model"`
	System             string                `hcl:"system,optional"`
	Options            *ProviderModelOptions `hcl:"options,block"`
	CostPerInputToken  float64               `hcl:"cost_per_input_token,optional"`
	CostPerOutputToken float64               `hcl:"cost_per_output_token,optional"`
	// ContextWindowSize accepts bare integers ("128000") or suffixed forms ("128k", "1m").
	ContextWindowSize string `hcl:"context_window_size,optional"`
}

// ParseContextWindowSize parses the ContextWindowSize string into a token count.
// Supports bare integers and k/K (×1 000) and m/M (×1 000 000) suffixes.
// Returns 0 for empty or unparseable input.
func (t *ProviderModelTemplate) ParseContextWindowSize() int {
	return parseTokenCount(t.ContextWindowSize)
}

// parseTokenCount converts strings like "128k", "976K", "1m", "198000" to ints.
func parseTokenCount(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	multiplier := 1
	lower := strings.ToLower(s)
	if strings.HasSuffix(lower, "m") {
		multiplier = 1_000_000
		s = s[:len(s)-1]
	} else if strings.HasSuffix(lower, "k") {
		multiplier = 1_000
		s = s[:len(s)-1]
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return int(v * float64(multiplier))
}

// ProviderModelOptions maps to model generation parameters.
// All fields are pointers so zero values can be distinguished from "not set".
type ProviderModelOptions struct {
	Temperature *float64 `hcl:"temperature,optional"`
	NumPredict  *int     `hcl:"num_predict,optional"`
	TopP        *float64 `hcl:"top_p,optional"`
	TopK        *int     `hcl:"top_k,optional"`
}
