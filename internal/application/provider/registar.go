package provider

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/plugin/provider"
	domprovider "github.com/mwantia/forge/internal/domain/provider"
)

type ProviderRegistar = domprovider.ProviderRegistar

func (s *ProviderService) getProvider(name string) (provider.ProviderPlugin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return p, nil
}

// resolveAgent looks up an agent by bare name in the declared agent configs.
func (s *ProviderService) resolveAgent(agentName string) (*AgentConfig, error) {
	for _, a := range s.configs.Agents {
		if a.Name == agentName {
			return a, nil
		}
	}
	return nil, fmt.Errorf("agent %q not found", agentName)
}

// resolveAgentCandidate picks the first candidate whose constraints all pass
// against cc (first-match, top-to-bottom). An unconditional candidate (zero
// constraints) always passes and acts as the fallback.
// Returns the selected candidate and a human-readable reason for logging.
func resolveAgentCandidate(agent *AgentConfig, cc domprovider.ConstraintContext) (*AgentModelCandidate, string, error) {
	if len(agent.Models) == 1 {
		return agent.Models[0], "single candidate", nil
	}
	for i, c := range agent.Models {
		if len(c.Constraints) == 0 {
			return c, fmt.Sprintf("unconditional fallback (candidate %d of %d)", i+1, len(agent.Models)), nil
		}
		if domprovider.EvaluateConstraints(c.Constraints, cc) {
			return c, fmt.Sprintf("constraints matched (candidate %d of %d, size=%d mode=%s)", i+1, len(agent.Models), cc.Size, cc.Mode), nil
		}
	}
	return nil, "", fmt.Errorf("agent %q: no candidate matched constraints (size=%d mode=%s)", agent.Name, cc.Size, cc.Mode)
}

// agentToSDKModel builds a provider.Model from an agent and its selected candidate.
// The model name is the bare model portion of candidate.Ref (after the "/").
func agentToSDKModel(agent *AgentConfig, candidate *AgentModelCandidate) *provider.Model {
	_, modelName, _ := strings.Cut(candidate.Ref, "/")

	system := agent.System
	if candidate.System != "" {
		if system != "" {
			system += "\n\n"
		}
		system += candidate.System
	}

	m := &provider.Model{
		ModelName: modelName,
		System:    system,
	}
	if agent.Options != nil && agent.Options.Temperature != nil {
		m.Temperature = *agent.Options.Temperature
	}
	if agent.Context != nil {
		m.ContextWindowSize = agent.Context.ParseWindowSize()
	}
	if agent.Cost != nil {
		m.CostPerInputToken = agent.Cost.InputToken
		m.CostPerOutputToken = agent.Cost.OutputToken
	}
	return m
}

// ResolveAgent resolves a bare agent name to its provider name and SDK Model,
// evaluating candidate constraints against cc.
func (s *ProviderService) ResolveAgent(agentName string, cc domprovider.ConstraintContext) (string, *provider.Model, error) {
	agent, err := s.resolveAgent(agentName)
	if err != nil {
		return "", nil, err
	}
	candidate, reason, err := resolveAgentCandidate(agent, cc)
	if err != nil {
		return "", nil, err
	}
	providerName, _, ok := strings.Cut(candidate.Ref, "/")
	if !ok {
		return "", nil, fmt.Errorf("agent %q candidate %q is not in provider/model format", agentName, candidate.Ref)
	}
	model := agentToSDKModel(agent, candidate)
	s.logger.Debug("Agent resolved",
		"agent", agentName,
		"candidate", candidate.Ref,
		"provider", providerName,
		"model", model.ModelName,
		"reason", reason,
		"context_size", cc.Size,
		"context_mode", cc.Mode,
		"context_turns", cc.Turns,
		"context_window", model.ContextWindowSize,
		"candidates_total", len(agent.Models),
	)
	return providerName, model, nil
}

func (s *ProviderService) Chat(ctx context.Context, providerName, modelName string, messages []provider.ChatMessage, tools []provider.ToolCall) (provider.ChatStream, error) {
	if providerName == "" {
		cc := domprovider.ConstraintContextFrom(ctx)
		cc.Size = estimateTokens(messages)
		realProvider, model, err := s.ResolveAgent(modelName, cc)
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
	return p.Chat(ctx, messages, tools, &provider.Model{ModelName: modelName})
}

func (s *ProviderService) Embed(ctx context.Context, providerName, modelName, content string) ([][]float32, error) {
	if providerName == "" {
		cc := domprovider.ConstraintContextFrom(ctx)
		cc.Size = len(content) / 4
		realProvider, model, err := s.ResolveAgent(modelName, cc)
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
	return p.Embed(ctx, content, &provider.Model{ModelName: modelName})
}

// estimateTokens returns a rough token count from the message list (chars / 4).
func estimateTokens(messages []provider.ChatMessage) int {
	n := 0
	for _, m := range messages {
		n += len(m.Content)
		if m.ToolCalls != nil {
			for _, tc := range m.ToolCalls.ToolCalls {
				n += len(tc.Arguments)
			}
		}
	}
	return n / 4
}

func (s *ProviderService) ListAllModels(ctx context.Context, q domprovider.ListModelsQuery) ([]*domprovider.ModelInfo, error) {
	s.mu.RLock()
	providers := make(map[string]provider.ProviderPlugin, len(s.providers))
	maps.Copy(providers, s.providers)
	s.mu.RUnlock()

	var result []*domprovider.ModelInfo

	if q.Type == "" || q.Type == domprovider.ModelTypeAgent {
		for _, agent := range s.configs.Agents {
			if q.Name != "" && !strings.Contains(strings.ToLower(agent.Name), strings.ToLower(q.Name)) {
				continue
			}
			info := &domprovider.ModelInfo{
				Name:    agent.Name,
				Type:    domprovider.ModelTypeAgent,
				System:  agent.System,
				Options: agent.Options,
				Cost:    agent.Cost,
				Context: agent.Context,
			}
			result = append(result, info)
		}
	}

	if q.Type == "" || q.Type == domprovider.ModelTypeModel {
		for name, p := range providers {
			if q.Provider != "" && !strings.EqualFold(name, q.Provider) {
				continue
			}
			models, err := p.ListModels(ctx)
			if err != nil {
				s.logger.Warn("Failed to list models", "provider", name, "error", err)
				continue
			}
			for _, m := range models {
				fullName := name + "/" + m.ModelName
				if q.Name != "" && !strings.Contains(strings.ToLower(fullName), strings.ToLower(q.Name)) {
					continue
				}
				result = append(result, &domprovider.ModelInfo{
					Name:       fullName,
					Type:       domprovider.ModelTypeModel,
					ModifiedAt: m.ModifiedAt,
					Size:       m.Size,
					Digest:     m.Digest,
					Details:    m.Details,
					Metadata:   m.Metadata,
				})
			}
		}
	}

	sortModels(result, q.Sort, q.Order)
	return result, nil
}

func sortModels(models []*domprovider.ModelInfo, by, order string) {
	desc := strings.EqualFold(order, "desc")
	sort.SliceStable(models, func(i, j int) bool {
		var less bool
		switch by {
		case "size":
			less = models[i].Size < models[j].Size
		case "modified_at":
			less = models[i].ModifiedAt < models[j].ModifiedAt
		default:
			less = models[i].Name < models[j].Name
		}
		if desc {
			return !less
		}
		return less
	})
}

func (s *ProviderService) ListModels(ctx context.Context, providerName string) ([]*provider.Model, error) {
	p, err := s.getProvider(providerName)
	if err != nil {
		return nil, err
	}
	return p.ListModels(ctx)
}

func (s *ProviderService) GetModel(ctx context.Context, providerName, modelName string) (*provider.Model, error) {
	if providerName == "" {
		// bare agent name
		agent, err := s.resolveAgent(modelName)
		if err != nil {
			return nil, err
		}
		candidate, _, err := resolveAgentCandidate(agent, domprovider.ConstraintContextFrom(ctx))
		if err != nil {
			return nil, err
		}
		return agentToSDKModel(agent, candidate), nil
	}
	p, err := s.getProvider(providerName)
	if err != nil {
		return nil, err
	}
	return p.GetModel(ctx, modelName)
}

func (s *ProviderService) DeleteModel(ctx context.Context, providerName, modelName string) (bool, error) {
	p, err := s.getProvider(providerName)
	if err != nil {
		return false, err
	}
	return p.DeleteModel(ctx, modelName)
}

// ResolveEmbedModel splits a "provider/model" reference and returns the two
// parts. Accepts bare provider/model strings only — no agent names.
func (s *ProviderService) ResolveEmbedModel(_ context.Context, modelRef string) (string, string, error) {
	if modelRef == "" {
		return "", "", fmt.Errorf("embed model reference is empty")
	}
	providerName, modelName, ok := strings.Cut(modelRef, "/")
	if !ok {
		return "", "", fmt.Errorf("embed model %q must be in provider/model format", modelRef)
	}
	return providerName, modelName, nil
}

var _ ProviderRegistar = (*ProviderService)(nil)
