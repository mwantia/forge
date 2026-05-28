package provider

import domprovider "github.com/mwantia/forge/internal/domain/provider"

const (
	ModelTypeChat  = domprovider.ModelTypeChat
	ModelTypeEmbed = domprovider.ModelTypeEmbed
)

type (
	ProviderConfig        = domprovider.ProviderConfig
	ProviderModelTemplate = domprovider.ProviderModelTemplate
	ProviderModelOptions  = domprovider.ProviderModelOptions
)
