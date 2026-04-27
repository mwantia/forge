package plugins

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/mwantia/forge/internal/service/metrics"
)

var (
	PluginsMetrics = metrics.NewSubSubsystemMetrics("plugins")

	// PluginsLoaded tracks the number of plugin drivers currently loaded, by type.
	PluginsLoaded = PluginsMetrics.GaugeWithLabels(
		"loaded",
		"Number of plugin drivers currently loaded by type",
		[]string{"type"},
	)

	// PluginsServeTotal counts plugin startup attempts, by type and outcome.
	PluginsServeTotal = PluginsMetrics.CounterWithLabels(
		"serve_total",
		"Total number of plugin serve attempts by type and status",
		[]string{"type", "status"},
	)

	// PluginsServeDuration records how long plugin startup takes, by type.
	PluginsServeDuration = PluginsMetrics.HistogramWithLabels(
		"serve_duration_seconds",
		"Duration of plugin driver startup in seconds",
		[]string{"type"},
		prometheus.DefBuckets,
	)

)
