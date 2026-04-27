package metrics

import (
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type MetricsRegistar interface {
	Register(collectors ...prometheus.Collector) error
	MustRegister(collectors ...prometheus.Collector)
	Registerer() prometheus.Registerer
}

func (r *MetricsService) Register(collectors ...prometheus.Collector) error {
	for _, c := range collectors {
		if err := r.reg.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return fmt.Errorf("registering collector: %w", err)
			}
			// Already registered — treat it as success
		}
	}
	return nil
}

func (r *MetricsService) MustRegister(collectors ...prometheus.Collector) {
	if err := r.Register(collectors...); err != nil {
		panic(err)
	}
}

func (r *MetricsService) Registerer() prometheus.Registerer {
	return r.prom
}
