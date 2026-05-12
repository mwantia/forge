package log

import "github.com/hashicorp/go-hclog"

// RegisterSink attaches a SinkAdapter to the global intercept logger.
func RegisterSink(sink hclog.SinkAdapter) {
	if globInterceptor != nil {
		globInterceptor.RegisterSink(sink)
	}
}

// DeregisterSink removes a previously registered SinkAdapter.
func DeregisterSink(sink hclog.SinkAdapter) {
	if globInterceptor != nil {
		globInterceptor.DeregisterSink(sink)
	}
}
