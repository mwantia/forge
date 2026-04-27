package provider

import "github.com/mwantia/forge/internal/service/metrics"

var (
	ProviderMetrics = metrics.NewSubSubsystemMetrics("provider")

	ProvidersTotal = ProviderMetrics.Counter(
		"providers_total",
		"Total number of providers registered.",
	)
)
