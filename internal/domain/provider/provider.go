package provider

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
}

// ProviderModelOptions maps to model generation parameters.
// All fields are pointers so zero values can be distinguished from "not set".
type ProviderModelOptions struct {
	Temperature *float64 `hcl:"temperature,optional"`
	NumPredict  *int     `hcl:"num_predict,optional"`
	TopP        *float64 `hcl:"top_p,optional"`
	TopK        *int     `hcl:"top_k,optional"`
}
