package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/mwantia/forge/internal/service/metrics"
)

var (
	ServerMetrics = metrics.NewSubSubsystemMetrics("server")

	ServerHttpRequestsTotal = ServerMetrics.CounterWithLabels(
		"http_requests_total",
		"Total number of HTTP requests by method, path, and status",
		[]string{"method", "path", "status"},
	)

	ServerHttpRequestsDuration = ServerMetrics.HistogramWithLabels(
		"http_requests_duration_seconds",
		"Duration of HTTP requests in seconds by method and path",
		[]string{"method", "path"},
		prometheus.ExponentialBuckets(0.0001, 2, 10),
	)
)
