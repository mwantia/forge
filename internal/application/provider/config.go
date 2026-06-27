package provider

import domprovider "github.com/mwantia/forge/internal/domain/provider"

const (
	ModelTypeAgent = domprovider.ModelTypeAgent
	ModelTypeModel = domprovider.ModelTypeModel
)

type (
	ProviderConfig       = domprovider.ProviderConfig
	AgentConfig          = domprovider.AgentConfig
	AgentModelCandidate  = domprovider.AgentModelCandidate
	AgentConstraint      = domprovider.AgentConstraint
	ProviderModelOptions = domprovider.ProviderModelOptions
	ListModelsQuery      = domprovider.ListModelsQuery
)
