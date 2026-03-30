package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge-sdk/pkg/metrics"
	"github.com/mwantia/forge/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	prom prometheus.Registerer
	reg  *prometheus.Registry
	mux  *http.ServeMux
	srv  *http.Server

	logger hclog.Logger          `fabric:"logger:metrics"`
	config *config.MetricsConfig `fabric:"config:metrics"`
}

func NewMetrics(cfg *config.AgentConfig, log hclog.Logger) (*Metrics, error) {
	mux := http.NewServeMux()
	reg := prometheus.NewRegistry()
	prom := prometheus.WrapRegistererWith(prometheus.Labels{}, reg)
	srv := &http.Server{
		Addr:    cfg.Metrics.Address,
		Handler: mux,
	}

	return &Metrics{
		reg:  reg,
		prom: prom,
		mux:  mux,
		srv:  srv,
	}, nil
}

func (m *Metrics) Setup() (func() error, error) {
	m.mux = http.NewServeMux()
	m.reg = prometheus.NewRegistry()
	m.prom = prometheus.WrapRegistererWith(prometheus.Labels{}, m.reg)
	m.srv = &http.Server{
		Addr:    m.config.Address,
		Handler: m.mux,
	}

	m.prom.MustRegister(collectors.NewGoCollector())
	m.prom.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m.prom.MustRegister(metrics.ServerHttpRequestsDurationSeconds)
	m.prom.MustRegister(metrics.ServerHttpRequestsTotal)

	m.mux.Handle("/metrics", promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{}))

	return func() error {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		m.logger.Debug("Performing metrics shutdown...")
		return m.srv.Shutdown(shutdown)
	}, nil
}

func (m *Metrics) Serve(ctx context.Context) error {
	m.logger.Info("Serving metrics server", "address", m.config.Address)
	if err := m.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
