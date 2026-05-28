package storage

import (
	"github.com/mwantia/forge/internal/infrastructure/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	StorageMetrics = metrics.NewSubSubsystemMetrics("storage")

	OperationsTotal = StorageMetrics.CounterWithLabels(
		"operations_total",
		"Total number of storage operations partitioned by operation type and result status",
		[]string{"operation", "status"},
	)

	OperationDuration = StorageMetrics.Histogram(
		"operation_duration_seconds",
		"Latency of storage operations in seconds",
		[]string{"operation"},
		prometheus.DefBuckets,
	)

	BytesTotal = StorageMetrics.CounterWithLabels(
		"bytes_total",
		"Total bytes transferred by raw storage operations",
		[]string{"operation", "direction"},
	)
)
