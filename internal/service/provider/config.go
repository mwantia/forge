package provider

const (
	ModelTypeChat  = "chat"
	ModelTypeEmbed = "embed"
)

type ProviderConfig struct {
	Models []*ProviderModelTemplate `hcl:"model,block"`
}

// ProviderModelTemplate defines a named model alias within forge.
//
// forge sets `base_model` as the base model, prepends `system` as a system
// message, and merges `options` into every chat request for this alias.
type ProviderModelTemplate struct {
	// Name is the forge name used to declare this model.
	// The full qualified model name is combined with the provider: <providername>/<modelname>.
	Name string `hcl:"name,label"`

	// Type declares whether this alias is for chat completion or embeddings.
	// Empty string defaults to "chat". Use ModelTypeChat / ModelTypeEmbed.
	Type string `hcl:"type,optional"`

	// BaseModel is the underlying model to use (e.g. "llama3.2", "glm-4:9b").
	BaseModel string `hcl:"base_model"`

	// System is prepended as a system message on every request using this alias.
	System string `hcl:"system,optional"`

	// Options overrides generation parameters for this model alias.
	Options *ProviderModelOptions `hcl:"options,block"`

	// CostPerInputToken is the cost in USD per input (prompt) token. Zero means no cost tracking.
	CostPerInputToken float64 `hcl:"cost_per_input_token,optional"`

	// CostPerOutputToken is the cost in USD per output (completion) token.
	CostPerOutputToken float64 `hcl:"cost_per_output_token,optional"`
}

// ProviderModelOptions maps to model generation parameters.
// All fields are pointers so that zero values (e.g. temperature=0) can be distinguished from "not set".
type ProviderModelOptions struct {
	Temperature *float64 `hcl:"temperature,optional"`
	NumPredict  *int     `hcl:"num_predict,optional"`
	TopP        *float64 `hcl:"top_p,optional"`
	TopK        *int     `hcl:"top_k,optional"`
}
