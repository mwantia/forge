package plugins

import (
	"context"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

// Handshake is the plugin handshake configuration.
// Plugins and hosts must use the same values to communicate.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  2,
	MagicCookieKey:   "FORGE_PLUGIN",
	MagicCookieValue: "forge",
}

// DriverContextFactory creates a Driver with context support (external plugins only).
type DriverContextFactory func(ctx func() context.Context, log hclog.Logger) Driver

// Serve starts the plugin process and serves the Driver over gRPC.
// A single Driver can support multiple plugin types (provider, memory, channel, tools).
func Serve(df DriverFactory) {
	logger := hclog.Default().Named("plugin")
	driver := df(logger)

	serveDriver(driver)
}

// ServeContext starts the plugin with context support for cancellation.
func ServeContext(dcf DriverContextFactory) {
	logger := hclog.Default().Named("plugin")

	ctx := func() context.Context {
		return context.Background()
	}

	serveDriver(dcf(ctx, logger))
}

func serveDriver(driver Driver) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]goplugin.Plugin{
			"driver": &DriverPlugin{Impl: driver},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger: hclog.New(&hclog.LoggerOptions{
			Name:       "plugin",
			Level:      hclog.Debug,
			JSONFormat: true,
		}),
	})
}

// ServeWithLogger starts the plugin with a custom logger.
func ServeWithLogger(df DriverFactory, logger hclog.Logger) {
	serveDriver(df(logger))
}

// ServeContextWithLogger starts the plugin with context support and a custom logger.
func ServeContextWithLogger(dcf DriverContextFactory, logger hclog.Logger) {
	ctx := func() context.Context {
		return context.Background()
	}
	serveDriver(dcf(ctx, logger))
}
