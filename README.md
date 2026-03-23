# Forge

A pluggable AI agent framework built in Go. Forge provides a plugin-based architecture for integrating LLM providers (like Ollama) and tools/skills through a unified interface.

## Features

- **Plugin System**: Extensible driver-based plugin architecture using gRPC for isolated plugin execution
- **LLM Provider Support**: Built-in Ollama provider plugin with streaming support
- **Tools/Skills Plugin**: Loadable skills/tools for enhanced agent capabilities
- **HTTP Server**: RESTful API server with health endpoints
- **Prometheus Metrics**: Built-in metrics endpoint for observability
- **HCL Configuration**: Flexible HashiCorp Configuration Language for agent setup
- **Sandbox Mode**: Test plugins without running the full agent

## Installation

### Prerequisites

- Go 1.24.3 or later
- Task build tool (optional, for convenience commands)

### Build

Using Go directly:

```bash
go mod download && go mod tidy
go build -o ./build/forge ./cmd/forge/main.go
```

Using Task:

```bash
task setup    # Download and tidy dependencies
task build    # Build static binary to ./build/forge
```

## Usage

### Running the Agent

```bash
# Run with default configuration
./build/forge agent

# Run with custom configuration
./build/forge agent --path config.hcl

# Run once and exit (for testing)
./build/forge agent --once
```

### Running a Plugin

```bash
# Serve a specific plugin
./build/forge plugin ollama
./build/forge plugin skills
./build/forge plugin stub
```

### Sandbox Mode

Test plugins without running the full agent:

```bash
# Quick test with Ollama
./build/forge sandbox --model ollama/llama2 "What is the capital of France?"

# Test with tools
./build/forge sandbox --model ollama/llama2 --tools skills "Help me with this task"
```

## Configuration

Forge uses HCL (HashiCorp Configuration Language) for configuration. Create a `config.hcl` file:

```hcl
log_level = "DEBUG"

server {
  address = "0.0.0.0:9280"
  token   = "optional-auth-token"
}

metrics {
  address = "0.0.0.0:9500"
}

plugin_dir = "./plugins"

plugin "ollama" {
  address = "http://localhost:11434"
  model   = "llama2"
  timeout = 60
}

plugin "skills" {
  path = "./skills"
}
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `log_level` | Logging level (DEBUG, INFO, WARN, ERROR, FATAL) | `INFO` |
| `server.address` | HTTP server bind address | `127.0.0.1:9280` |
| `server.token` | Optional authentication token | (empty) |
| `metrics.address` | Prometheus metrics endpoint address | `127.0.0.1:9500` |
| `plugin_dir` | Directory containing plugin binaries | (empty) |

## Architecture

```
cmd/forge/main.go          # Entry point (Cobra CLI)
internal/
  agent/                   # Core agent orchestration
    agent.go               # Agent struct, serves runners
    runner.go              # Runner interface (Setup, Serve)
  config/                  # HCL configuration parsing
  plugins/                 # Plugin registry system
    registry.go            # PluginRegistry, driver management
    serve.go               # Plugin serving logic
  sandbox/                 # Sandbox testing environment
  server/                  # Gin HTTP server
    server.go              # Server struct, routes setup
    api/health.go          # Health endpoint
  metrics/                 # Prometheus metrics server
pkg/
  errors/                  # Multi-error handling utilities
  log/                     # Custom structured logging
  plugins/                 # Plugin interface definitions
    plugin.go              # Core plugin interfaces
    driver.go              # Driver implementation helpers
    grpc.go                # gRPC client implementation
    proto/                 # Protocol buffer definitions
plugins/
  ollama/                  # Ollama LLM provider plugin
  skills/                  # Skills/tools plugin
  stub/                    # Stub plugin for testing
```

## Plugin Development

### Plugin Types

Forge supports multiple plugin types:

- **Provider**: LLM providers (Ollama, Anthropic, OpenAI, etc.)
- **Tools**: Skills and tools for enhanced capabilities
- **Memory**: Conversation memory management
- **Channel**: Communication channels

### Creating a Plugin

1. Implement the `Driver` interface from `pkg/plugins/driver.go`
2. Register your plugin using `plugins.Register(name, factory)`
3. Build as a separate binary and configure in `config.hcl`

Example plugin structure:

```go
package myplugin

import (
    "github.com/hashicorp/go-hclog"
    "github.com/mwantia/forge/pkg/plugins"
)

const PluginName = "myplugin"

func init() {
    plugins.Register(PluginName, NewMyDriver)
}

func NewMyDriver(log hclog.Logger) plugins.Driver {
    return &MyDriver{log: log.Named(PluginName)}
}

type MyDriver struct {
    log hclog.Logger
}

// Implement Driver interface methods...
```

### Plugin Capabilities

Each driver reports its capabilities through `GetCapabilities()`:

```go
func (d *MyDriver) GetCapabilities(ctx context.Context) (*proto.DriverCapabilities, error) {
    return &proto.DriverCapabilities{
        Types: []string{plugins.PluginTypeProvider},
        Provider: &proto.ProviderCaps{
            SupportsStreaming: true,
            SupportsVision:    false,
        },
    }, nil
}
```

## API Endpoints

### Health Check

```
GET /health
```

Returns server health status.

## Development

### Project Structure

- `cmd/` - Main applications
- `internal/` - Private application code
- `pkg/` - Public library code (can be imported by external projects)
- `plugins/` - Built-in plugins

### Key Patterns

**Runner Interface**: Both `Server` and `Metrics` implement the `Runner` interface. The Agent manages runners via `serveRunner()` which calls `Setup()`, spawns a goroutine for `Serve()`, and tracks cleanup functions.

**Cleanup Chain**: Each runner's `Setup()` returns a cleanup function. Agent stores these and calls all on shutdown via `Cleanup()`.

**Plugin Registry**: The `PluginRegistry` manages plugin lifecycle, loads plugins via `ServePlugins()`, stores drivers, and provides access through `GetProviderPlugin()` and `GetToolsPlugin()`.

## License

MIT License