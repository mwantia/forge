package provider

import (
	"strconv"
	"strings"

	sdkprovider "github.com/mwantia/forge-sdk/pkg/plugin/provider"
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
	Temperature *float64 `hcl:"temperature,optional" json:"temperature,omitempty"`
	NumPredict  *int     `hcl:"num_predict,optional" json:"num_predict,omitempty"`
	TopP        *float64 `hcl:"top_p,optional"       json:"top_p,omitempty"`
	TopK        *int     `hcl:"top_k,optional"       json:"top_k,omitempty"`
}

// ModelCost holds per-token pricing for a model.
type ModelCost struct {
	InputToken  float64 `json:"input_token,omitempty"`
	OutputToken float64 `json:"output_token,omitempty"`
}

// ModelContext holds context-window metadata for a model.
type ModelContext struct {
	WindowSize string `json:"window_size,omitempty"`
}

// ModelInfo is the unified model representation returned by GET /v1/provider/models.
// Forge-alias fields are populated for locally configured models; Name carries the
// bare alias without the "forge/" prefix. Provider models use "provider/model" as
// Name; alias fields are absent and provider-supplied fields are populated instead.
type ModelInfo struct {
	Name      string                `json:"name"`
	Type      string                `json:"type,omitempty"`
	BaseModel string                `json:"base_model,omitempty"`
	System    string                `json:"system,omitempty"`
	Options   *ProviderModelOptions `json:"options,omitempty"`
	Cost      *ModelCost            `json:"cost,omitempty"`
	Context   *ModelContext         `json:"context,omitempty"`

	ModifiedAt string                  `json:"modified_at,omitempty"`
	Size       int64                   `json:"size,omitempty"`
	Digest     string                  `json:"digest,omitempty"`
	Details    *sdkprovider.ModelDetails `json:"details,omitempty"`
	Metadata   map[string]any          `json:"metadata,omitempty"`
}
