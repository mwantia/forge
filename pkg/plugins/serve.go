package plugins

import (
	"context"
	"os"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

// DriverContextFactory creates a Driver with context support (external plugins only).
type DriverContextFactory func(ctx func() context.Context, log hclog.Logger) Driver

// Serve starts the plugin process and serves the Driver over gRPC.
// A single Driver can support multiple plugin types (provider, memory, channel, tools).
func Serve(df DriverFactory) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:       "plugin",
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	})
	driver := df(logger)

	serveDriver(driver, logger)
}

// ServeContext starts the plugin with context support for cancellation.
func ServeContext(dcf DriverContextFactory) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:       "plugin",
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	ctx := func() context.Context {
		return context.Background()
	}

	serveDriver(dcf(ctx, logger), logger)
}

func serveDriver(driver Driver, logger hclog.Logger) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]goplugin.Plugin{
			"driver": &DriverPlugin{Impl: driver},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     logger,
	})
}

// ServeWithLogger starts the plugin with a custom logger.
func ServeWithLogger(df DriverFactory, logger hclog.Logger) {
	serveDriver(df(logger), logger)
}

// ServeContextWithLogger starts the plugin with context support and a custom logger.
func ServeContextWithLogger(dcf DriverContextFactory, logger hclog.Logger) {
	ctx := func() context.Context {
		return context.Background()
	}
	serveDriver(dcf(ctx, logger), logger)
}
