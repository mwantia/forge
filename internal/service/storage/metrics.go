package storage

import (
	"github.com/mwantia/forge/internal/service/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	StorageMetrics = metrics.NewSubSubsystemMetrics("storage")

	// OperationsTotal counts every storage call partitioned by operation name and
	// result status ("success" or "error").
	OperationsTotal = StorageMetrics.CounterWithLabels(
		"operations_total",
		"Total number of storage operations partitioned by operation type and result status",
		[]string{"operation", "status"},
	)

	// OperationDuration records the latency of each storage call in seconds.
	OperationDuration = StorageMetrics.Histogram(
		"operation_duration_seconds",
		"Latency of storage operations in seconds",
		[]string{"operation"},
		prometheus.DefBuckets,
	)

	// BytesTotal counts raw bytes transferred, labelled by operation and direction
	// ("read" or "write"). Only populated for GetRaw and PutRaw where the byte
	// count is available at this layer.
	BytesTotal = StorageMetrics.CounterWithLabels(
		"bytes_total",
		"Total bytes transferred by raw storage operations",
		[]string{"operation", "direction"},
	)
)
