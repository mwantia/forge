package metrics

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsService struct {
	service.Service
	MetricsRegistar

	mu   sync.RWMutex
	prom prometheus.Registerer
	reg  *prometheus.Registry
	mux  *http.ServeMux
	srv  *http.Server

	logger hclog.Logger  `fabric:"logger:metrics"`
	config MetricsConfig `fabric:"config:metrics"`
}

func init() {
	if err := container.Register[*MetricsService](
		container.AsSingleton(),
		container.With[MetricsRegistar](),
	); err != nil {
		panic(err)
	}
}

func (r *MetricsService) Init(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.config.Address == "" {
		r.config.Address = "127.0.0.1:9500"
	}

	r.mux = http.NewServeMux()
	r.reg = prometheus.NewRegistry()
	r.prom = prometheus.WrapRegistererWith(prometheus.Labels{}, r.reg)
	r.srv = &http.Server{
		Addr:    r.config.Address,
		Handler: r.mux,
	}

	r.prom.MustRegister(collectors.NewGoCollector())
	r.prom.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	metricsHandler := promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{})
	if r.config.Token != "" {
		metricsHandler = r.authMiddleware(metricsHandler)
	}
	r.mux.Handle("/metrics", metricsHandler)

	return nil
}

func (r *MetricsService) Cleanup(ctx context.Context) error {
	shutdown, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	r.logger.Debug("Performing metrics shutdown...")
	return r.srv.Shutdown(shutdown)
}

func (r *MetricsService) Serve(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Info("Serving metrics server", "address", r.config.Address)
	if err := r.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

var _ service.Service = (*MetricsService)(nil)
var _ MetricsRegistar = (*MetricsService)(nil)
