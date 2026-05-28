package provider

import inframetrics "github.com/mwantia/forge/internal/infrastructure/metrics"

var (
	ProviderMetrics = inframetrics.NewSubSubsystemMetrics("provider")

	ProvidersTotal = ProviderMetrics.Counter(
		"providers_total",
		"Total number of providers registered.",
	)
)
