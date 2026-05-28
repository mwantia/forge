package session

import inframetrics "github.com/mwantia/forge/internal/infrastructure/metrics"

var (
	SessionMetrics = inframetrics.NewSubSubsystemMetrics("session")

	SessionsTotal = SessionMetrics.CounterWithLabels(
		"operations_total",
		"Total number of session operations by type",
		[]string{"operation"},
	)
)
