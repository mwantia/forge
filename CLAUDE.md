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
go run ./cmd/forge/main.go agent --config <config.hcl>
```

## Architecture

```
cmd/forge/
  main.go              # Entry point (Cobra CLI)
  client/              # HTTP client CLI commands (forge sessions ...)
    client.go          # ForgeClient, resolveClient (env/flag fallback)
    sessions.go        # forge sessions {list,create,get,delete,messages,send}
  server/              # Server-side CLI commands
    agent.go           # forge agent command
    plugin.go          # forge plugin command
internal/
  agent/               # Core agent orchestration
    agent.go           # Agent struct, serves runners
    runner.go          # Runner interface (Setup, Serve)
  config/              # HCL configuration parsing
    agent.go           # AgentConfig (includes DataDir), Parse(), NewDefault()
    server.go          # ServerConfig
    metrics.go         # MetricsConfig
  registry/            # Plugin registry system
    registry.go        # PluginRegistry, driver management
    provider.go        # PluginProviderNamespace, Chat routing by "provider/model"
    serve.go           # Plugin serving logic
  server/              # Gin HTTP server
    server.go          # Server struct, routes setup
    auth.go            # Bearer token middleware
    logger.go          # Request logging middleware
    recovery.go        # Panic recovery middleware
    api/               # HTTP handler functions
      health.go        # GET /v1/health
      sessions.go      # Session CRUD + message dispatch handlers
      stream.go        # streamChat SSE helper
      plugins.go       # Plugin listing handlers
      models.go        # Model CRUD handlers
      tools.go         # Tool listing/execution handlers
      embeddings.go    # Embedding handler
  session/             # Session management
    session.go         # Session, Message, ToolCallEntry types
    manager.go         # Manager: CRUD + Dispatch, CreateOptions, resolveTools
    pipeline.go        # runPipeline, pipelineStream, emitContent, replayAsStream
    store.go           # FileStore: file-based session/message persistence
  metrics/             # Prometheus metrics server
    metrics.go         # Separate metrics HTTP endpoint
pkg/
  errors/              # Multi-error handling utilities
    errors.go          # Errors struct with thread-safe accumulation
  log/                 # Custom structured logging
  metrics/             # Prometheus metric definitions
  plugins/             # Plugin interface definitions
    provider.go        # ChatMessage, ChatChunk, ChatResult, CollectStream
    plugin.go          # Core plugin interfaces (Driver, Provider, Tools, etc.)
    driver.go          # Driver implementation helpers
    grpc.go            # gRPC client implementation
    serve.go           # Plugin server setup
    const.go           # Shared constants
    grpc/              # gRPC implementations per plugin type
      provider/        # Provider gRPC client/server + proto
      tools/           # Tools gRPC client/server + proto
      memory/          # Memory gRPC client/server + proto
      channel/         # Channel gRPC client/server + proto
      driver/          # Driver gRPC client/server + proto
plugins/
  ollama/              # Ollama LLM provider plugin
  filesystem/          # Filesystem tools plugin
  skills/              # Skills/tools plugin
```

## Key Patterns

**Runner Interface**: Both `Server` and `Metrics` implement the `Runner` interface (`internal/agent/runner.go`). The Agent manages runners via `serveRunner()` which calls `Setup()`, spawns a goroutine for `Serve()`, and tracks cleanup functions.

**Plugin Registry**: The `PluginRegistry` (`internal/registry/registry.go`) manages plugin lifecycle. It loads plugins via `ServePlugins()`, stores drivers, and provides access via `GetProviderPlugin()` and `GetToolsPlugin()`. Model routing lives in `internal/registry/provider.go` — the `"provider/model"` string is split to look up the right driver.

**Session Management**: `internal/session/manager.go` coordinates session CRUD and message dispatch. `Dispatch()` saves the user message, launches `runPipeline` as a goroutine, and returns a `pipelineStream` immediately. The pipeline buffers each LLM response via `CollectStream`, executes tool calls if any, and replays the final response as chunks. Intermediate assistant text (before tool calls) is emitted via `emitContent` so clients see it in real time.

**Tool Name Namespacing**: Tools are prefixed as `pluginName/toolName` when registered (e.g. `skills/list_files`). The prefix is stripped before calling `tp.Execute()` so plugins receive the bare tool name.

**File-Based Persistence**: `internal/session/store.go` stores sessions as `{dataDir}/{sessionID}/session.json` and messages as `{dataDir}/{sessionID}/messages/{20-digit-unix-nano}_{id}.json`. The zero-padded timestamp prefix gives chronological order from a plain directory listing.

**Configuration**: HCL-based config parsed by `config.Parse()`. Returns defaults if no path provided. The full `*AgentConfig` is injected into `Server` (not just `*ServerConfig`) so it can access `DataDir`.

**Cleanup Chain**: Each runner's `Setup()` returns a cleanup function. Agent stores these and calls all on shutdown via `Cleanup()`.

**Errors Package**: Thread-safe multi-error handling in `pkg/errors/errors.go`. Used for accumulating errors during cleanup operations.

**Default Ports**:
- Server: `127.0.0.1:9280`
- Metrics: `127.0.0.1:9500`

## Configuration File Format

HCL format (example config.hcl):
```hcl
log_level = "DEBUG"
data_dir  = "./data"

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
}

plugin "skills" {}
```

Log levels: `DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`.

## API Routes

All routes except `/v1/health` require a `Authorization: Bearer <token>` header when `server.token` is set.

```
GET    /v1/health

GET    /v1/plugins
GET    /v1/plugins/:name

GET    /v1/models
GET    /v1/models/:provider
POST   /v1/models/:provider
DELETE /v1/models/:provider/:model

POST   /v1/embeddings

GET    /v1/tools
GET    /v1/tools/:driver
GET    /v1/tools/:driver/:tool
POST   /v1/tools/:driver/:tool/validate
POST   /v1/tools/:driver/:tool/execute
DELETE /v1/tools/:driver/cancel/:call_id

GET    /v1/sessions
POST   /v1/sessions
GET    /v1/sessions/:id
DELETE /v1/sessions/:id
GET    /v1/sessions/:id/messages
POST   /v1/sessions/:id/messages
```
