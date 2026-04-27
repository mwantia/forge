package tools

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/mwantia/forge/internal/service/metrics"
)

var (
	ToolsMetrics = metrics.NewSubSubsystemMetrics("tools")

	ToolsTotal = ToolsMetrics.CounterWithLabels(
		"tools_total",
		"Total number of tools registered per namespace",
		[]string{"namespace"},
	)

	ToolsExecutionsTotal = ToolsMetrics.CounterWithLabels(
		"executions_total",
		"Total number of tool executions by namespace, name, and status",
		[]string{"namespace", "name", "status"},
	)

	ToolsExecutionDuration = ToolsMetrics.Histogram(
		"execution_duration_seconds",
		"Duration of tool executions in seconds",
		[]string{"namespace", "name"},
		prometheus.DefBuckets,
	)
)
