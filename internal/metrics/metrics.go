package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/pkg/log"
	"github.com/mwantia/forge/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	log log.Logger
	cfg config.MetricsConfig

	reg  *prometheus.Registry
	prom prometheus.Registerer
	mux  *http.ServeMux
	srv  *http.Server
}

func NewMetrics(cfg config.AgentConfig) (*Metrics, error) {
	mux := http.NewServeMux()
	reg := prometheus.NewRegistry()
	prom := prometheus.WrapRegistererWith(prometheus.Labels{}, reg)
	srv := &http.Server{
		Addr:    cfg.Metrics.Address,
		Handler: mux,
	}

	return &Metrics{
		log:  log.Named("metrics"),
		cfg:  *cfg.Metrics,
		reg:  reg,
		prom: prom,
		mux:  mux,
		srv:  srv,
	}, nil
}

func (impl *Metrics) Setup() (func() error, error) {
	impl.prom.MustRegister(collectors.NewGoCollector())
	impl.prom.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	impl.prom.MustRegister(metrics.ServerHttpRequestsDurationSeconds)
	impl.prom.MustRegister(metrics.ServerHttpRequestsTotal)

	impl.mux.Handle("/metrics", promhttp.HandlerFor(impl.reg, promhttp.HandlerOpts{}))

	return func() error {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		impl.log.Debug("Performing metrics shutdown...")
		return impl.srv.Shutdown(shutdown)
	}, nil
}

func (impl *Metrics) Serve(ctx context.Context) error {
	impl.log.Info("Serving metrics server", "address", impl.cfg.Address)
	if err := impl.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
