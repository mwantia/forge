package ollama

// OllamaConfig holds the configuration for the Ollama driver.
type OllamaConfig struct {
	Address string `mapstructure:"address"`
	Model   string `mapstructure:"model"`
	Timeout int    `mapstructure:"timeout"` // Timeout in seconds
}

// DefaultConfig returns the default configuration for Ollama.
func DefaultConfig() *OllamaConfig {
	return &OllamaConfig{
		Address: "http://localhost:11434",
		Model:   "llama2",
		Timeout: 60,
	}
}

// OllamaRequest represents a request to the Ollama API.
type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages,omitempty"`
	Prompt   string          `json:"prompt,omitempty"`
	Stream   bool            `json:"stream"`
	Options  OllamaOptions   `json:"options,omitempty"`
	Tools    []OllamaTool   `json:"tools,omitempty"`
}

// OllamaMessage represents a message in the chat API.
type OllamaMessage struct {
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	ToolCalls []OllamaToolCall  `json:"tool_calls,omitempty"`
}

// OllamaOptions represents additional options for generation.
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
}

// OllamaResponse represents a response from the Ollama API.
type OllamaResponse struct {
	Model     string         `json:"model"`
	CreatedAt string         `json:"created_at"`
	Message   *OllamaMessage `json:"message"`
	Response  string         `json:"response"`
	Done      bool           `json:"done"`
	Context   []int          `json:"context"`
}

// OllamaGenerateResponse is the response from /api/generate endpoint.
type OllamaGenerateResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

// OllamaChatResponse is the response from /api/chat endpoint.
type OllamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// OllamaModelInfo represents model information from /api/show endpoint.
type OllamaModelInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	QuantType string `json:"quant_type"`
}

// OllamaTool represents a tool in a request.
type OllamaTool struct {
	Type     string          `json:"type"`
	Function OllamaFunction `json:"function"`
}

// OllamaFunction describes the function to call.
type OllamaFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// OllamaToolCall represents a tool call in a response.
type OllamaToolCall struct {
	Function OllamaToolCallFunction `json:"function"`
}

// OllamaToolCallFunction contains name and arguments.
type OllamaToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}
