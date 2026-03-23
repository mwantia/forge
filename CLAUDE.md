# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

Uses [Task](https://taskfile.dev/) as the build tool. Key commands:

```bash
task setup          # Download and tidy dependencies
task build          # Build static binary to ./build/forge
task run            # Run agent with test config
task release        # Build and push multi-arch Docker image
```

Direct Go commands:
```bash
go mod download && go mod tidy
go build -o ./build/forge ./cmd/forge/main.go
go run ./cmd/forge/main.go agent --path <config.hcl>
```

## Architecture

```
cmd/forge/main.go          # Entry point (Cobra CLI)
internal/
  agent/                   # Core agent orchestration
    agent.go               # Agent struct, serves runners
    runner.go              # Runner interface (Setup, Serve)
  config/                  # HCL configuration parsing
    agent.go               # AgentConfig, Parse(), NewDefault()
    server.go              # ServerConfig
    metrics.go             # MetricsConfig
  plugins/                 # Plugin registry system
    registry.go            # PluginRegistry, driver management
    serve.go               # Plugin serving logic
  sandbox/                 # Sandbox testing environment
    sandbox.go             # Sandbox struct, plugin execution
    flags.go               # Sandbox CLI flags
  server/                  # Gin HTTP server
    server.go              # Server struct, routes setup
    api/health.go          # Health endpoint
    recovery.go, logger.go # Middleware
  metrics/                 # Prometheus metrics server
    metrics.go             # Separate metrics HTTP endpoint
pkg/
  errors/                  # Multi-error handling utilities
    errors.go              # Errors struct with thread-safe accumulation
  log/                     # Custom structured logging
  metrics/                 # Prometheus metric definitions
  plugins/                 # Plugin interface definitions
    plugin.go              # Core plugin interfaces (Driver, Provider, etc.)
    driver.go              # Driver implementation helpers
    grpc.go                # gRPC client implementation
    serve.go               # Plugin server setup
    const.go               # Shared constants
    proto/                 # Protocol buffer definitions
      driver.proto         # gRPC service definitions
      driver.pb.go         # Generated protobuf code
      driver_grpc.pb.go    # Generated gRPC code
plugins/
  ollama/                  # Ollama LLM provider plugin
  skills/                   # Skills/tools plugin
  stub/                    # Stub plugin for testing
```

## Key Patterns

**Runner Interface**: Both `Server` and `Metrics` implement the `Runner` interface (`internal/agent/runner.go:8-12`). The Agent manages runners via `serveRunner()` which calls `Setup()`, spawns a goroutine for `Serve()`, and tracks cleanup functions.

**Plugin Registry**: The `PluginRegistry` (`internal/plugins/registry.go`) manages plugin lifecycle. It loads plugins via `ServePlugins()`, stores drivers, and provides access to `GetProviderPlugin()` and `GetToolsPlugin()`. Cleanup via `CleanupDrivers()`.

**Configuration**: HCL-based config parsed by `config.Parse()`. Returns defaults if no path provided. Config structure is defined in `internal/config/agent.go:11-16`.

**Cleanup Chain**: Each runner's `Setup()` returns a cleanup function. Agent stores these and calls all on shutdown via `Cleanup()`.

**Errors Package**: Thread-safe multi-error handling in `pkg/errors/errors.go`. Used for accumulating errors during cleanup operations.

**Default Ports**:
- Server: `127.0.0.1:9280`
- Metrics: `127.0.0.1:9500`

## Configuration File Format

HCL format (example config.hcl):
```hcl
log_level = "DEBUG"

server {
  address = "0.0.0.0:9280"
  token   = "optional-auth-token"
}

metrics {
  address = "0.0.0.0:9500"
}
```

Log levels: `DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`.