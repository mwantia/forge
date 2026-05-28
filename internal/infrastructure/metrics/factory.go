package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type SubSubsystemMetrics struct {
	Namespace string
	Subsystem string
}

func NewSubSubsystemMetrics(subsystem string) *SubSubsystemMetrics {
	return &SubSubsystemMetrics{
		Namespace: "forge",
		Subsystem: strings.TrimSpace(strings.ToLower(subsystem)),
	}
}

func (s *SubSubsystemMetrics) CounterWithLabels(name, help string, labels []string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: s.Namespace,
		Subsystem: s.Subsystem,
		Name:      name,
		Help:      help,
	}, labels)
}

func (s *SubSubsystemMetrics) Counter(name, help string) *prometheus.CounterVec {
	return s.CounterWithLabels(name, help, make([]string, 0))
}

func (s *SubSubsystemMetrics) HistogramWithLabels(name, help string, labels []string, buckets []float64) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: s.Namespace,
		Subsystem: s.Subsystem,
		Name:      name,
		Help:      help,
		Buckets:   buckets,
	}, labels)
}

func (s *SubSubsystemMetrics) Histogram(name, help string, labels []string, buckets []float64) *prometheus.HistogramVec {
	return s.HistogramWithLabels(name, help, labels, buckets)
}

func (s *SubSubsystemMetrics) GaugeWithLabels(name, help string, labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: s.Namespace,
		Subsystem: s.Subsystem,
		Name:      name,
		Help:      help,
	}, labels)
}

func (s *SubSubsystemMetrics) Gauge(name, help string, labels []string) *prometheus.GaugeVec {
	return s.GaugeWithLabels(name, help, labels)
}
