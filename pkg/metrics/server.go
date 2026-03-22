package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	ServerHttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "forge",
			Subsystem: "server",
			Name:      "http_requests_total",
			Help:      "Count the amount of http requests",
		},
		[]string{"method", "addr", "path", "status"},
	)
	ServerHttpRequestsDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "forge",
			Subsystem: "server",
			Name:      "http_requests_duration_seconds",
			Help:      "Histogram of the time (in seconds) each http request took",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 10),
		},
		[]string{"method", "addr", "path", "status"},
	)
)
