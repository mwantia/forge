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
type OllamaProviderPlugin struct {
	driver *OllamaDriver
}

func (p *OllamaProviderPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *OllamaProviderPlugin) GetPluginInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Type:    plugins.PluginTypeProvider,
		Name:    "ollama-provider",
		Author:  "forge",
		Version: "0.1.0",
	}
}

func (p *OllamaProviderPlugin) Generate(ctx context.Context, req plugins.GenerateRequest) (*plugins.GenerateResponse, error) {
	if p.driver.config == nil {
		return nil, fmt.Errorf("driver not configured")
	}

	// Use the model from request or fall back to config default
	model := req.Model
	if model == "" {
		model = p.driver.config.Model
	}

	// Build Ollama chat request
	ollamaReq := OllamaRequest{
		Model:  model,
		Stream: false,
		Options: OllamaOptions{
			Temperature: req.Temperature,
		},
	}

	// Convert messages
	if len(req.Messages) > 0 {
		ollamaReq.Messages = make([]OllamaMessage, len(req.Messages))
		for i, msg := range req.Messages {
			ollamaMsg := OllamaMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}

			// Handle assistant messages with tool calls
			if len(msg.ToolCalls) > 0 {
				ollamaMsg.ToolCalls = make([]OllamaToolCall, len(msg.ToolCalls))
				for j, tc := range msg.ToolCalls {
					ollamaMsg.ToolCalls[j] = OllamaToolCall{
						Function: OllamaToolCallFunction{
							Name:      tc.Name,
							Arguments: tc.Arguments,
						},
					}
				}
			}

			// Handle tool result messages
			if msg.ToolCallID != "" {
				ollamaMsg.ToolCallID = msg.ToolCallID
			}

			ollamaReq.Messages[i] = ollamaMsg
		}
	}

	// Convert tools to Ollama format
	if len(req.Tools) > 0 {
		ollamaReq.Tools = make([]OllamaTool, len(req.Tools))
		for i, t := range req.Tools {
			ollamaTool := OllamaTool{
				Type: "function",
				Function: OllamaFunction{
					Name:        t.Name,
					Description: t.Description,
				},
			}
			// Only include parameters if not nil/empty
			if t.Parameters != nil {
				ollamaTool.Function.Parameters = t.Parameters
			}
			ollamaReq.Tools[i] = ollamaTool
		}
	}

	// Debug: log that we're sending tools
	p.driver.log.Debug("Sending to Ollama", "model", model, "tools_count", len(ollamaReq.Tools), "messages_count", len(ollamaReq.Messages))

	// Set max tokens if specified
	if req.MaxTokens > 0 {
		ollamaReq.Options.NumPredict = req.MaxTokens
	}

	// Send request to Ollama
	resp, err := p.doRequest(ctx, "/api/chat", ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode ollama response: %w", err)
	}

	fmt.Println(ollamaResp)

	// Convert tool calls from Ollama format
	var toolCalls []plugins.ToolCall
	if len(ollamaResp.Message.ToolCalls) > 0 {
		toolCalls = make([]plugins.ToolCall, len(ollamaResp.Message.ToolCalls))
		for i, tc := range ollamaResp.Message.ToolCalls {
			// Generate ID if not provided
			id := tc.Function.Name // Use function name as ID for Ollama
			if id == "" {
				id = uuid.New().String()[:8]
			}
			toolCalls[i] = plugins.ToolCall{
				ID:        id,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	return &plugins.GenerateResponse{
		ID:        ollamaResp.CreatedAt,
		Content:   ollamaResp.Message.Content,
		Role:      ollamaResp.Message.Role,
		Model:     ollamaResp.Model,
		ToolCalls: toolCalls,
	}, nil
}

// doRequest sends a request to the Ollama API.
func (p *OllamaProviderPlugin) doRequest(ctx context.Context, endpoint string, req any) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Debug: log the request body
	p.driver.log.Debug("Ollama request body", "body", string(body))

	url := p.driver.config.Address + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	return p.driver.client.Do(httpReq)
}

// GenerateWithPrompt generates a response using the /api/generate endpoint (non-chat models).
func (p *OllamaProviderPlugin) GenerateWithPrompt(ctx context.Context, prompt string, model string) (string, error) {
	if model == "" {
		model = p.driver.config.Model
	}

	ollamaReq := OllamaRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	resp, err := p.doRequest(ctx, "/api/generate", ollamaReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return ollamaResp.Response, nil
}

// ListModels returns available models from Ollama.
func (p *OllamaProviderPlugin) ListModels(ctx context.Context) ([]string, error) {
	url := p.driver.config.Address + "/api/tags"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.driver.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var modelsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	models := make([]string, len(modelsResp.Models))
	for i, m := range modelsResp.Models {
		models[i] = m.Name
	}

	return models, nil
}
