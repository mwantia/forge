package memory

import "github.com/mwantia/forge/internal/service/metrics"

var (
	MemoryMetrics = metrics.NewSubSubsystemMetrics("memory")

	MemoryOperationsTotal = MemoryMetrics.CounterWithLabels(
		"operations_total",
		"Total number of memory operations processed.",
		[]string{"op", "status"},
	)

	MemoryOperationDuration = MemoryMetrics.HistogramWithLabels(
		"operation_duration_seconds",
		"Memory operation latency in seconds.",
		[]string{"op"},
		nil,
	)
)
