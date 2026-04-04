# Forge

A pluggable AI agent framework built in Go. Forge provides a plugin-based architecture for integrating LLM providers (like Ollama) and tools/skills, exposed through a REST API with persistent session management.

## Features

- **Session Management**: Create and manage persistent chat sessions with per-session model, tool, and system prompt configuration
- **Tool Call Pipeline**: Automatic tool call loop with intermediate streaming — clients see assistant reasoning before tool results arrive
- **Plugin System**: Extensible driver-based plugin architecture using gRPC for isolated plugin execution
- **LLM Provider Support**: Built-in Ollama provider plugin with full streaming support
- **Tools/Skills Plugins**: Loadable tool plugins (filesystem, skills) with namespaced tool names
- **CLI Client**: `forge sessions` commands for interacting with a running agent from the terminal
- **HTTP REST API**: Full session, model, tool, and embedding endpoints with optional bearer token auth
- **Prometheus Metrics**: Built-in metrics endpoint for observability
- **HCL Configuration**: Flexible HashiCorp Configuration Language for agent setup
- **File-Based Persistence**: Sessions and message history stored to disk under `data_dir`

## Installation

### Prerequisites

- Go 1.24.3 or later
- Task build tool (optional, for convenience commands)

### Build

Forge uses a code generation step to produce the plugin import file (`cmd/forge/plugins.go`) from a `plugins.yaml` manifest. Copy the example manifest before building:

```bash
cp plugins.example.yaml plugins.yaml
```

Using Task:

```bash
task setup      # Download and tidy dependencies
task generate   # Generate cmd/forge/plugins.go from plugins.yaml
task build      # generate + compile static binary to ./build/forge
```

Using Go directly:

```bash
go mod download && go mod tidy
go run ./tools/plugins -manifest plugins.yaml -out cmd/forge
go build -tags all -a -ldflags '-s -w -extldflags "-static"' -o ./build/forge ./cmd/forge/main.go
```

You can also trigger generation via the `go:generate` directive in `cmd/forge/generate.go`:

```bash
go generate ./cmd/forge/...
```

## Usage

### Running the Agent

```bash
# Run with default configuration
./build/forge agent

# Run with a single config file
./build/forge agent --config config.hcl

# Run with a config directory (all *.hcl files merged alphabetically)
./build/forge agent --config ./config/

# Run once and exit (for testing)
./build/forge agent --once
```

### Managing Sessions

Use the `forge sessions` subcommand to interact with a running agent. Authentication and address are resolved from flags or environment variables:

| Flag | Environment Variable | Default |
|------|---------------------|---------|
| `--http-addr` | `FORGE_HTTP_ADDR` | `http://127.0.0.1:9280` |
| `--http-token` | `FORGE_HTTP_TOKEN` | (none) |

```bash
# List all sessions
forge sessions list

# Create a session
forge sessions create --model ollama/llama3.2
forge sessions create --model ollama/llama3.2 --tools skills,filesystem
forge sessions create --model ollama/llama3.2 --system-prompt "You are a helpful assistant."

# Get session details
forge sessions get <id>

# Send a message (blocking, returns full response)
forge sessions send <id> "List the files in my home directory."

# Send a message with streaming output
forge sessions send <id> "Explain this code." --stream

# List message history
forge sessions messages <id>

# Delete a session
forge sessions delete <id>
```

### Serving a Plugin

```bash
./build/forge plugin ollama
./build/forge plugin skills
./build/forge plugin filesystem
```

## Configuration

Forge uses HCL (HashiCorp Configuration Language) for configuration. Pass either a single file or a directory to `--config`:

```bash
# Single file
./build/forge agent --config config.hcl

# Directory — all *.hcl files are merged alphabetically
./build/forge agent --config ./config/
```

### Single file example

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

plugin "ollama" "ollama" {
  config {
    address = "http://localhost:11434"
  }
}

plugin "skills" "skills" {}

plugin "filesystem" "filesystem" {}
```

### Split directory example

As the number of plugins grows it can be useful to split configuration across multiple files:

```
config/
  00_base.hcl       # log_level, data_dir, server {}, metrics {}
  10_ollama.hcl     # plugin "ollama" ...
  10_skills.hcl     # plugin "skills" ...
  10_filesystem.hcl # plugin "filesystem" ...
```

Files are merged in alphabetical order. Scalar attributes follow last-writer-wins; blocks (like `plugin`) are appended from all files. Numeric prefixes (`00_`, `10_`) are a convenient way to control ordering and group related config.

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `log_level` | Logging level (`DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`) | `INFO` |
| `data_dir` | Directory for session and message persistence | `./data` |
| `plugin_dir` | Directory containing plugin binaries | (empty) |
| `server.address` | HTTP server bind address | `127.0.0.1:9280` |
| `server.token` | Optional bearer token for API authentication | (empty) |
| `metrics.address` | Prometheus metrics endpoint address | `127.0.0.1:9500` |

## API

All routes except `GET /v1/health` require `Authorization: Bearer <token>` when `server.token` is set.

### Sessions

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/sessions` | List sessions (`?limit=20&offset=0`) |
| `POST` | `/v1/sessions` | Create a session |
| `GET` | `/v1/sessions/:id` | Get session details |
| `DELETE` | `/v1/sessions/:id` | Delete a session |
| `GET` | `/v1/sessions/:id/messages` | List message history (`?limit=50&offset=0`) |
| `POST` | `/v1/sessions/:id/messages` | Send a message; set `"stream": true` for SSE |

**Create session body:**
```json
{
  "model": "ollama/llama3.2",
  "tools": ["skills", "filesystem"],
  "system_prompt": "You are a helpful assistant.",
  "memory": "",
  "max_tool_iterations": 10
}
```

**Send message body:**
```json
{ "content": "What files are in my home directory?", "stream": false }
```

When `stream: true`, the response is Server-Sent Events with `data: {json}` lines ending in `data: [DONE]`. Intermediate assistant text (before tool calls execute) is streamed immediately.

### Models

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/models` | List all models across providers |
| `GET` | `/v1/models/:provider` | List models for a provider |
| `POST` | `/v1/models/:provider` | Create/pull a model |
| `DELETE` | `/v1/models/:provider/:model` | Delete a model |

### Tools

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/tools` | List all tools across plugins |
| `GET` | `/v1/tools/:driver` | List tools for a driver |
| `GET` | `/v1/tools/:driver/:tool` | Get tool definition |
| `POST` | `/v1/tools/:driver/:tool/validate` | Validate tool arguments |
| `POST` | `/v1/tools/:driver/:tool/execute` | Execute a tool |
| `DELETE` | `/v1/tools/:driver/cancel/:call_id` | Cancel a running tool call |

### Other

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/health` | Health check |
| `GET` | `/v1/plugins` | List loaded plugins |
| `GET` | `/v1/plugins/:name` | Get plugin details |
| `POST` | `/v1/embeddings` | Generate embeddings |

## Architecture

```
cmd/forge/
  main.go              # Entry point (Cobra CLI)
  generate.go          # go:generate directive for plugin code generation
  plugins.go           # Generated blank imports (do not edit manually)
  client/              # CLI client commands
    client.go          # ForgeClient HTTP wrapper
    sessions.go        # forge sessions subcommands
  server/              # Server-side CLI commands
    agent.go           # forge agent command
    plugin.go          # forge plugin command
tools/
  plugins/             # Code generator: reads plugins.yaml, writes cmd/forge/plugins.go
internal/
  agent/               # Core agent orchestration
  config/              # HCL configuration parsing
  registry/            # Plugin registry and provider routing
  session/             # Session management and pipeline
    session.go         # Session, Message types
    manager.go         # CRUD, Dispatch, tool resolution
    pipeline.go        # Tool call loop, streaming
    store.go           # File-based persistence
  server/              # Gin HTTP server and middleware
    api/               # Route handlers
  metrics/             # Prometheus metrics server
pkg/
  errors/              # Thread-safe multi-error accumulation
  log/                 # Structured logging helpers
  metrics/             # Prometheus metric definitions
  plugins/             # Plugin interface definitions and gRPC transport
plugins/
  ollama/              # Ollama LLM provider plugin
  filesystem/          # Filesystem tools plugin
  skills/              # Skills/tools plugin
```

## Plugins

All plugins — regardless of how they are loaded — run as isolated subprocesses and communicate with the main agent over gRPC using the [go-plugin](https://github.com/hashicorp/go-plugin) protocol. Forge supports two ways to load plugins:

### 1. Built-in (compile-time)

Plugin code is compiled into the `forge` binary. At runtime, forge spawns the plugin as a subprocess from the same binary and communicates with it over gRPC.

The `plugins.yaml` manifest controls which plugins are included. `task generate` reads the manifest and writes `cmd/forge/plugins.go`; `task build` runs this automatically.

Copy the example manifest and edit it:

```bash
cp plugins.example.yaml plugins.yaml
```

```yaml
plugins:
  - name: ollama
    module: github.com/mwantia/forge-plugin-ollama
    import: github.com/mwantia/forge-plugin-ollama/plugin

  - name: skills
    module: github.com/mwantia/forge-plugin-skills
    import: github.com/mwantia/forge-plugin-skills/plugin
    local: ../plugins/skills   # optional: redirect to a local checkout
```

| Field | Description |
|-------|-------------|
| `name` | Plugin identifier (used in log output) |
| `module` | Go module path (used for `go mod edit -replace` when `local` is set) |
| `import` | Package import path whose `init()` registers the plugin |
| `local` | Optional relative path; triggers `go mod edit -replace` or `go work use` automatically |

### 2. External gRPC plugins (runtime)

Pre-built plugin binaries placed in `plugin_dir` (default: `./plugins`) are discovered and launched at startup. Forge spawns each binary as a subprocess and connects over gRPC.

Declare external plugins in `config.hcl`:

```hcl
plugin_dir = "./plugins"

plugin "ollama" "my-ollama" {
  config {
    address = "http://127.0.0.1:11434"
  }
}
```

This approach requires no recompilation — drop a new binary into `plugin_dir` and restart the agent.

### Plugin Types

| Type | Description |
|------|-------------|
| `Provider` | LLM providers (chat completion, embeddings, model management) |
| `Tools` | Tool discovery and execution for agentic workflows |
| `Memory` | Session storage and RAG (interface defined, optional) |
| `Channel` | Communication gateways (interface defined, future) |

See the [shared SDK README](../shared/README.md) and `plugins/skills/` or `plugins/filesystem/` for reference implementations.

## License

MIT License
