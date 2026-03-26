package ollama

// OllamaConfig holds the configuration for the Ollama driver.
type OllamaConfig struct {
	Address string                          `mapstructure:"address"`
	Timeout string                          `mapstructure:"timeout"` // Timeout in seconds
	Models  map[string]*OllamaModelTemplate `mapstructure:"model"`
}

// OllamaModelTemplate defines a named model alias within forge.
//
// When `create = false` (default), the model is resolved entirely in memory:
// forge sets `base_model` as the Ollama model, prepends `system` as a system
// message, and merges `options` into every chat request for this alias.
//
// When `create = true`, forge additionally provisions the model in Ollama via
// /api/create on startup, comparing the generated Modelfile hash against the
type OllamaModelTemplate struct {
	// BaseModel is the underlying Ollama model to use (e.g. "llama3.2", "glm-4:9b").
	BaseModel string `mapstructure:"base_model"`

	// Reasoning controls whether thinking/reasoning tokens are included in the
	// response stream. When false, <think>...</think> blocks are stripped.
	Reasoning bool `mapstructure:"reasoning"`

	// System is prepended as a system message on every request using this alias.
	System string `mapstructure:"system"`

	// Options overrides generation parameters for this model alias.
	Options *OllamaModelOptions `mapstructure:"options"`
}

// OllamaModelOptions maps to Ollama generation parameters.
// All fields are pointers so that zero values (e.g. temperature=0) can be
// distinguished from "not set".
type OllamaModelOptions struct {
	Temperature *float64 `mapstructure:"temperature"`
	NumPredict  *int     `mapstructure:"num_predict"`
	TopP        *float64 `mapstructure:"top_p"`
	TopK        *int     `mapstructure:"top_k"`
}

// DefaultConfig returns the default configuration for Ollama.
func DefaultConfig() *OllamaConfig {
	return &OllamaConfig{
		Address: "http://localhost:11434",
		Timeout: "60s",
	}
}

// OllamaChatRequest represents a request to the Ollama /api/chat endpoint.
type OllamaChatRequest struct {
	Model    string            `json:"model"`
	Messages []OllamaMessage   `json:"messages,omitempty"`
	Stream   bool              `json:"stream"`
	Options  OllamaChatOptions `json:"options,omitempty"`
	Tools    []OllamaTool      `json:"tools,omitempty"`
}

// OllamaMessage represents a message in the chat API.
type OllamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"`
}

// OllamaChatOptions represents additional options for generation.
type OllamaChatOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
}

// OllamaChatResponse is a single streamed chunk from /api/chat.
type OllamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// OllamaTool represents a tool definition in a request.
type OllamaTool struct {
	Type     string         `json:"type"`
	Function OllamaFunction `json:"function"`
}

// OllamaFunction describes the function to call.
type OllamaFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// OllamaToolCall represents a tool call in a response message.
type OllamaToolCall struct {
	Function OllamaToolCallFunction `json:"function"`
}

// OllamaToolCallFunction contains the name and arguments of the call.
type OllamaToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// OllamaEmbedRequest is the request body for /api/embed.
type OllamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// OllamaEmbedResponse is the response from /api/embed.
type OllamaEmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

// OllamaTagsResponse is the response from /api/tags (list models).
type OllamaTagsResponse struct {
	Models []OllamaModelEntry `json:"models"`
}

// OllamaModelEntry describes a single model in the tags list.
type OllamaModelEntry struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

// OllamaCreateRequest is the request body for /api/create.
type OllamaCreateRequest struct {
	Name      string `json:"name"`
	Modelfile string `json:"modelfile"`
	Stream    bool   `json:"stream"`
}

// OllamaCreateResponse is a streamed chunk from /api/create.
type OllamaCreateResponse struct {
	Status string `json:"status"`
}

// OllamaShowRequest is the request body for /api/show.
type OllamaShowRequest struct {
	Name string `json:"name"`
}

// OllamaShowResponse is the response from /api/show.
type OllamaShowResponse struct {
	Modelfile  string            `json:"modelfile"`
	Parameters string            `json:"parameters"`
	Template   string            `json:"template"`
	Details    OllamaModelDetail `json:"details"`
}

// OllamaModelDetail holds details from /api/show.
type OllamaModelDetail struct {
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// OllamaDeleteRequest is the request body for /api/delete.
type OllamaDeleteRequest struct {
	Name string `json:"name"`
}
