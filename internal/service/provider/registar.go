package provider

import (
	"context"
	"fmt"
	"maps"
	"strings"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
)

type ProviderRegistar interface {
	Chat(ctx context.Context, providerName, modelName string, messages []sdkplugins.ChatMessage, tools []sdkplugins.ToolCall) (sdkplugins.ChatStream, error)
	Embed(ctx context.Context, providerName, modelName, content string) ([][]float32, error)

	ListAllModels(ctx context.Context) (map[string][]*sdkplugins.Model, []*ProviderModelTemplate, error)
	ListModels(ctx context.Context, providerName string) ([]*sdkplugins.Model, error)
	ListModelsByType(ctx context.Context, kind string) ([]*ProviderModelTemplate, error)
	GetModel(ctx context.Context, providerName, modelName string) (*sdkplugins.Model, error)
	ResolveEmbedModel(ctx context.Context, alias string) (providerName, modelName string, err error)
	CreateLocalModel(ctx context.Context, providerName string, tmpl *ProviderModelTemplate) (*sdkplugins.Model, error)
	DeleteModel(ctx context.Context, providerName, modelName string) (bool, error)
}

// modelType returns the declared type for a template, defaulting to chat.
func modelType(tmpl *ProviderModelTemplate) string {
	if tmpl == nil || tmpl.Type == "" {
		return ModelTypeChat
	}
	return tmpl.Type
}

func (s *ProviderService) getProvider(name string) (sdkplugins.ProviderPlugin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return p, nil
}

// resolveModel maps a forge-level model name to the plugin-level Model struct.
// It strips the provider prefix from base_model (e.g. "ollama/glm-5.1" → "glm-5.1").
// The system prompt is returned as a raw template; rendering with session-scoped
// variables happens in the pipeline layer before the prompt is sent to the LLM.
func (s *ProviderService) resolveModel(providerName, modelName string) *sdkplugins.Model {
	for _, tmpl := range s.configs.Models {
		if tmpl.Name == modelName {
			m := &sdkplugins.Model{
				ModelName:          strings.TrimPrefix(tmpl.BaseModel, providerName+"/"),
				System:             tmpl.System,
				CostPerInputToken:  tmpl.CostPerInputToken,
				CostPerOutputToken: tmpl.CostPerOutputToken,
			}
			if tmpl.Options != nil {
				if tmpl.Options.Temperature != nil {
					m.Temperature = *tmpl.Options.Temperature
				}
			}
			return m
		}
	}
	return &sdkplugins.Model{ModelName: modelName}
}

func (s *ProviderService) resolveAlias(modelName string) (string, *sdkplugins.Model, error) {
	return s.resolveAliasOfType(modelName, "")
}

// resolveAliasOfType resolves a forge/<alias> reference and (optionally) asserts
// that the alias is declared with the given model type. Pass an empty wantType
// to skip the assertion.
func (s *ProviderService) resolveAliasOfType(modelName, wantType string) (string, *sdkplugins.Model, error) {
	for _, tmpl := range s.configs.Models {
		if tmpl.Name != modelName {
			continue
		}
		got := modelType(tmpl)
		if wantType != "" && got != wantType {
			return "", nil, fmt.Errorf("model alias %q is declared type=%s, not usable for %s", modelName, got, wantType)
		}
		realProvider, _, ok := strings.Cut(tmpl.BaseModel, "/")
		if !ok {
			return "", nil, fmt.Errorf("invalid base_model format for alias %q: %q", modelName, tmpl.BaseModel)
		}
		return realProvider, s.resolveModel(realProvider, modelName), nil
	}
	return "", nil, fmt.Errorf("model alias %q not found", modelName)
}

func (s *ProviderService) Chat(ctx context.Context, providerName, modelName string, messages []sdkplugins.ChatMessage, tools []sdkplugins.ToolCall) (sdkplugins.ChatStream, error) {
	if providerName == "forge" {
		realProvider, model, err := s.resolveAliasOfType(modelName, ModelTypeChat)
		if err != nil {
			return nil, err
		}
		p, err := s.getProvider(realProvider)
		if err != nil {
			return nil, err
		}
		return p.Chat(ctx, messages, tools, model)
	}
	p, err := s.getProvider(providerName)
	if err != nil {
		return nil, err
	}
	return p.Chat(ctx, messages, tools, s.resolveModel(providerName, modelName))
}

func (s *ProviderService) Embed(ctx context.Context, providerName, modelName, content string) ([][]float32, error) {
	if providerName == "forge" {
		realProvider, model, err := s.resolveAliasOfType(modelName, ModelTypeEmbed)
		if err != nil {
			return nil, err
		}
		p, err := s.getProvider(realProvider)
		if err != nil {
			return nil, err
		}
		return p.Embed(ctx, content, model)
	}
	p, err := s.getProvider(providerName)
	if err != nil {
		return nil, err
	}
	return p.Embed(ctx, content, s.resolveModel(providerName, modelName))
}

func (s *ProviderService) ListAllModels(ctx context.Context) (map[string][]*sdkplugins.Model, []*ProviderModelTemplate, error) {
	s.mu.RLock()
	providers := make(map[string]sdkplugins.ProviderPlugin, len(s.providers))
	maps.Copy(providers, s.providers)
	s.mu.RUnlock()

	result := make(map[string][]*sdkplugins.Model, len(providers))
	for name, p := range providers {
		models, err := p.ListModels(ctx)
		if err != nil {
			s.logger.Warn("Failed to list models", "provider", name, "error", err)
			continue
		}
		result[name] = models
	}
	return result, s.configs.Models, nil
}

func (s *ProviderService) ListModels(ctx context.Context, providerName string) ([]*sdkplugins.Model, error) {
	if providerName == "forge" {
		models := make([]*sdkplugins.Model, 0, len(s.configs.Models))
		for _, tmpl := range s.configs.Models {
			realProvider, _, ok := strings.Cut(tmpl.BaseModel, "/")
			if !ok {
				continue
			}
			models = append(models, s.resolveModel(realProvider, tmpl.Name))
		}
		return models, nil
	}
	p, err := s.getProvider(providerName)
	if err != nil {
		return nil, err
	}
	return p.ListModels(ctx)
}

func (s *ProviderService) GetModel(ctx context.Context, providerName, modelName string) (*sdkplugins.Model, error) {
	if providerName == "forge" {
		_, model, err := s.resolveAlias(modelName)
		if err != nil {
			return nil, err
		}
		return model, nil
	}
	p, err := s.getProvider(providerName)
	if err != nil {
		return nil, err
	}
	return p.GetModel(ctx, modelName)
}

func (s *ProviderService) CreateLocalModel(ctx context.Context, providerName string, tmpl *ProviderModelTemplate) (*sdkplugins.Model, error) {
	p, err := s.getProvider(providerName)
	if err != nil {
		return nil, err
	}

	sdkTmpl := &sdkplugins.ModelTemplate{
		BaseModel: tmpl.BaseModel,
		System:    tmpl.System,
	}
	if tmpl.Options != nil {
		sdkTmpl.Parameters = make(map[string]any)
		if tmpl.Options.Temperature != nil {
			sdkTmpl.Parameters["temperature"] = *tmpl.Options.Temperature
		}
		if tmpl.Options.NumPredict != nil {
			sdkTmpl.Parameters["num_predict"] = *tmpl.Options.NumPredict
		}
		if tmpl.Options.TopP != nil {
			sdkTmpl.Parameters["top_p"] = *tmpl.Options.TopP
		}
		if tmpl.Options.TopK != nil {
			sdkTmpl.Parameters["top_k"] = *tmpl.Options.TopK
		}
	}

	return p.CreateModel(ctx, tmpl.Name, sdkTmpl)
}

func (s *ProviderService) DeleteModel(ctx context.Context, providerName, modelName string) (bool, error) {
	p, err := s.getProvider(providerName)
	if err != nil {
		return false, err
	}
	return p.DeleteModel(ctx, modelName)
}

func (s *ProviderService) ListModelsByType(ctx context.Context, kind string) ([]*ProviderModelTemplate, error) {
	if kind == "" {
		kind = ModelTypeChat
	}
	out := make([]*ProviderModelTemplate, 0, len(s.configs.Models))
	for _, tmpl := range s.configs.Models {
		if modelType(tmpl) == kind {
			out = append(out, tmpl)
		}
	}
	return out, nil
}

// ResolveEmbedModel validates that `alias` references a declared model of
// type=embed and returns the underlying provider and model name. Accepts a
// bare alias ("nomic") or the fully qualified form ("forge/nomic").
func (s *ProviderService) ResolveEmbedModel(ctx context.Context, alias string) (string, string, error) {
	if alias == "" {
		return "", "", fmt.Errorf("embed model alias is empty")
	}
	name := alias
	if p, rest, ok := strings.Cut(alias, "/"); ok {
		if p != "forge" {
			return "", "", fmt.Errorf("embed model %q must use the forge/ alias namespace", alias)
		}
		name = rest
	}
	provider, model, err := s.resolveAliasOfType(name, ModelTypeEmbed)
	if err != nil {
		return "", "", err
	}
	return provider, model.ModelName, nil
}

var _ ProviderRegistar = (*ProviderService)(nil)
