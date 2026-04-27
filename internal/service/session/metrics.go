package session

import "github.com/mwantia/forge/internal/service/metrics"

var (
	SessionMetrics = metrics.NewSubSubsystemMetrics("session")

	SessionsTotal = SessionMetrics.CounterWithLabels(
		"operations_total",
		"Total number of session operations by type",
		[]string{"operation"},
	)
)
