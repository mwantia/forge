package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

func (p *OllamaProviderPlugin) Chat(ctx context.Context, messages []plugins.ChatMessage, tools []plugins.ToolCall, model *plugins.Model) (plugins.ChatStream, error) {
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

	req := OllamaRequest{
		Model:  modelName,
		Stream: true,
		Options: OllamaOptions{
			Temperature: temperature,
		},
	}

	for _, msg := range messages {
		ollamaMsg := OllamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.ToolCalls != nil {
			for _, tc := range msg.ToolCalls.ToolCalls {
				ollamaMsg.ToolCalls = append(ollamaMsg.ToolCalls, OllamaToolCall{
					Function: OllamaToolCallFunction{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
			}
		}
		req.Messages = append(req.Messages, ollamaMsg)
	}

	for _, tool := range tools {
		req.Tools = append(req.Tools, OllamaTool{
			Type: "function",
			Function: OllamaFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.driver.config.Address+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.driver.streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()

		p.driver.log.Warn("Ollama request failed", "request", string(body), "response", string(bodyBytes))
		return nil, fmt.Errorf("ollama returned status %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	return NewChatStream(p.driver.log, httpResp.Body), nil
}

// --- Embed ---

func (p *OllamaProviderPlugin) Embed(ctx context.Context, content string, model *plugins.Model) ([][]float32, error) {
	if p.driver.config == nil {
		return nil, fmt.Errorf("driver not configured")
	}

	modelName := p.driver.config.Model
	if model != nil && model.ModelName != "" {
		modelName = model.ModelName
	}

	req := OllamaEmbedRequest{
		Model: modelName,
		Input: content,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.driver.config.Address+"/api/embed", bytes.NewReader(body))
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

	var resp OllamaEmbedResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return resp.Embeddings, nil
}

// --- Models ---

func (p *OllamaProviderPlugin) ListModels(ctx context.Context) ([]*plugins.Model, error) {
	if p.driver.config == nil {
		return nil, fmt.Errorf("driver not configured")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		p.driver.config.Address+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpResp, err := p.driver.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var resp OllamaTagsResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]*plugins.Model, len(resp.Models))
	for i, m := range resp.Models {
		models[i] = &plugins.Model{
			ModelName: m.Name,
			Metadata: map[string]any{
				"size":        m.Size,
				"modified_at": m.ModifiedAt,
			},
		}
	}
	return models, nil
}

func (p *OllamaProviderPlugin) CreateModel(ctx context.Context, modelName string, template *plugins.ModelTemplate) (*plugins.Model, error) {
	if p.driver.config == nil {
		return nil, fmt.Errorf("driver not configured")
	}

	modelfile := ""
	if template != nil {
		var sb strings.Builder
		if template.BaseModel != "" {
			sb.WriteString("FROM " + template.BaseModel + "\n")
		}
		if template.System != "" {
			sb.WriteString("SYSTEM " + template.System + "\n")
		}
		if template.PromptTemplate != "" {
			sb.WriteString("TEMPLATE " + template.PromptTemplate + "\n")
		}
		for k, v := range template.Parameters {
			sb.WriteString(fmt.Sprintf("PARAMETER %s %v\n", k, v))
		}
		modelfile = sb.String()
	}

	req := OllamaCreateRequest{
		Name:      modelName,
		Modelfile: modelfile,
		Stream:    false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.driver.config.Address+"/api/create", bytes.NewReader(body))
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

	return &plugins.Model{ModelName: modelName}, nil
}

func (p *OllamaProviderPlugin) GetModel(ctx context.Context, name string) (*plugins.Model, error) {
	if p.driver.config == nil {
		return nil, fmt.Errorf("driver not configured")
	}

	req := OllamaShowRequest{Name: name}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.driver.config.Address+"/api/show", bytes.NewReader(body))
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

	var resp OllamaShowResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &plugins.Model{
		ModelName: name,
		Metadata: map[string]any{
			"family":             resp.Details.Family,
			"parameter_size":     resp.Details.ParameterSize,
			"quantization_level": resp.Details.QuantizationLevel,
			"format":             resp.Details.Format,
		},
	}, nil
}

func (p *OllamaProviderPlugin) DeleteModel(ctx context.Context, name string) (bool, error) {
	if p.driver.config == nil {
		return false, fmt.Errorf("driver not configured")
	}

	req := OllamaDeleteRequest{Name: name}
	body, err := json.Marshal(req)
	if err != nil {
		return false, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		p.driver.config.Address+"/api/delete", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.driver.client.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	return httpResp.StatusCode == http.StatusOK, nil
}
