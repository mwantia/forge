package pipeline

import "github.com/mwantia/forge/internal/service/metrics"

var (
	PipelineMetrics = metrics.NewSubSubsystemMetrics("pipeline")

	PipelineMessagesTotal = PipelineMetrics.CounterWithLabels(
		"messages_total",
		"Total number of messages dispatched by status",
		[]string{"status"},
	)

	PipelineToolCallsTotal = PipelineMetrics.CounterWithLabels(
		"tool_calls_total",
		"Total number of tool calls made during pipeline execution by namespace, name, and status",
		[]string{"namespace", "name", "status"},
	)
)
