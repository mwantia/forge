package resource

import "github.com/mwantia/forge/internal/service/metrics"

var (
	ResourceMetrics = metrics.NewSubSubsystemMetrics("resource")

	ResourceOperationsTotal = ResourceMetrics.CounterWithLabels(
		"operations_total",
		"Total number of resource operations processed.",
		[]string{"op", "status"},
	)

	ResourceOperationDuration = ResourceMetrics.HistogramWithLabels(
		"operation_duration_seconds",
		"Resource operation latency in seconds.",
		[]string{"op"},
		nil,
	)
)
