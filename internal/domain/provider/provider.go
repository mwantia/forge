package provider

import (
	"strconv"
	"strings"

	sdkprovider "github.com/mwantia/forge-sdk/pkg/plugin/provider"
)

const (
	ModelTypeAgent = "agent"
	ModelTypeModel = "model"
)

// ProviderConfig holds all declared agents.
type ProviderConfig struct {
	Agents []*AgentConfig `hcl:"agent,block"`
}

// AgentConfig defines a named forge agent within the provider {} HCL block.
type AgentConfig struct {
	Name    string                 `hcl:"name,label"`
	System  string                 `hcl:"system,optional"`
	Options *ProviderModelOptions  `hcl:"options,block"`
	Cost    *ModelCost             `hcl:"cost,block"`
	Context *ModelContext          `hcl:"context,block"`
	Models  []*AgentModelCandidate `hcl:"model,block"`
}

// AgentModelCandidate is a candidate provider/model within an agent block.
// Ref is the "provider/model" label (e.g. "ollama/glm-5.2:cloud").
// A candidate with zero Constraints is unconditional and serves as the fallback.
type AgentModelCandidate struct {
	Ref         string             `hcl:"ref,label"`
	System      string             `hcl:"system,optional"`
	Constraints []*AgentConstraint `hcl:"constraint,block"`
}

// AgentConstraint is a single dispatch constraint on a candidate.
type AgentConstraint struct {
	Attribute string `hcl:"attribute"`
	Operator  string `hcl:"operator"`
	Value     string `hcl:"value"`
}

// ProviderModelOptions maps to model generation parameters.
// All fields are pointers so zero values can be distinguished from "not set".
type ProviderModelOptions struct {
	Temperature *float64 `hcl:"temperature,optional" json:"temperature,omitempty"`
	NumPredict  *int     `hcl:"num_predict,optional" json:"num_predict,omitempty"`
	TopP        *float64 `hcl:"top_p,optional"       json:"top_p,omitempty"`
	TopK        *int     `hcl:"top_k,optional"       json:"top_k,omitempty"`
}

// ModelCost holds per-token pricing. Used as both an HCL sub-block inside
// agent {} and as a JSON field in ModelInfo.
type ModelCost struct {
	InputToken  float64 `hcl:"input_token,optional"  json:"input_token,omitempty"`
	OutputToken float64 `hcl:"output_token,optional" json:"output_token,omitempty"`
}

// ModelContext holds context-window configuration. Used as both an HCL
// sub-block inside agent {} and as a JSON field in ModelInfo.
type ModelContext struct {
	WindowSize string `hcl:"window_size,optional" json:"window_size,omitempty"`
}

// ParseWindowSize parses the WindowSize string into a token count.
// Supports bare integers and k/K (×1 000) and m/M (×1 000 000) suffixes.
// Returns 0 for empty or unparseable input.
func (c *ModelContext) ParseWindowSize() int {
	if c == nil {
		return 0
	}
	return parseTokenCount(c.WindowSize)
}

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

// ListModelsQuery filters and sorts the result of ListAllModels.
type ListModelsQuery struct {
	Type     string // "agent", "model", or "" for both
	Provider string // filter raw models by provider prefix; no effect on agents
	Name     string // case-insensitive substring match on the name field
	Sort     string // "name" (default), "size", "modified_at"
	Order    string // "asc" (default), "desc"
}

// ModelInfo is the unified model/agent representation returned by
// GET /v1/provider/models. The Type field ("agent" or "model") is the
// discriminator. Agent entries populate the agent-specific fields;
// raw provider model entries populate the discovery fields.
// All fields are omitempty so each kind only serialises what it has.
type ModelInfo struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`

	// Agent fields (populated when Type == "agent")
	System  string                `json:"system,omitempty"`
	Options *ProviderModelOptions `json:"options,omitempty"`
	Cost    *ModelCost            `json:"cost,omitempty"`
	Context *ModelContext         `json:"context,omitempty"`

	// Provider model fields (populated when Type == "model")
	ModifiedAt string                    `json:"modified_at,omitempty"`
	Size       int64                     `json:"size,omitempty"`
	Digest     string                    `json:"digest,omitempty"`
	Details    *sdkprovider.ModelDetails `json:"details,omitempty"`
	Metadata   map[string]any            `json:"metadata,omitempty"`
}
