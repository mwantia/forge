package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/mwantia/forge/pkg/plugins"
)

// OllamaProviderPlugin implements ProviderPlugin for Ollama.
// Unimplemented capabilities fall back to UnimplementedProviderPlugin.
type OllamaProviderPlugin struct {
	plugins.UnimplementedProviderPlugin
	driver *OllamaDriver
}

func (p *OllamaProviderPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *OllamaProviderPlugin) Chat(ctx context.Context, messages []plugins.ChatMessage, tools []plugins.ToolCall, model *plugins.Model) (*plugins.ChatResult, error) {
	if p.driver.config == nil {
		return nil, fmt.Errorf("driver not configured")
	}

	modelName := p.driver.config.Model
	temperature := 0.0
	if model != nil {
		if model.ModelName != "" {
			modelName = model.ModelName
		}
		temperature = model.Temperature
	}

	ollamaReq := OllamaRequest{
		Model:  modelName,
		Stream: false,
		Options: OllamaOptions{
			Temperature: temperature,
		},
	}

	for _, msg := range messages {
		ollamaReq.Messages = append(ollamaReq.Messages, OllamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	for _, tool := range tools {
		ollamaReq.Tools = append(ollamaReq.Tools, OllamaTool{
			Type: "function",
			Function: OllamaFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.driver.config.Address+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.driver.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var ollamaResp OllamaChatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := &plugins.ChatResult{
		ID:      uuid.New().String(),
		Content: ollamaResp.Message.Content,
		Role:    ollamaResp.Message.Role,
		Metadata: map[string]any{
			"model": ollamaResp.Model,
		},
	}

	for _, tc := range ollamaResp.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, plugins.ChatToolCall{
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return result, nil
}
