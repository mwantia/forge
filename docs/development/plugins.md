# Plugin Development Guide

Forge's plugin system lets you extend the agent with new capabilities — tools, LLM providers, memory backends, and communication channels — without modifying core code. Plugins run as isolated subprocesses and communicate over gRPC using [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin).

## Table of Contents

- [Architecture](#architecture)
- [Plugin Types](#plugin-types)
- [The Driver Interface](#the-driver-interface)
- [HCL Configuration](#hcl-configuration)
- [Implementing a Plugin in Go](#implementing-a-plugin-in-go)
  - [Embedded Plugin](#embedded-plugin)
  - [External Binary Plugin](#external-binary-plugin)
- [Plugin Types in Detail](#plugin-types-in-detail)
  - [Tools Plugin](#tools-plugin)
  - [Provider Plugin](#provider-plugin)
  - [Memory Plugin](#memory-plugin)
  - [Channel Plugin](#channel-plugin)
- [Implementing in Other Languages](#implementing-in-other-languages)
  - [Python](#python)
  - [Rust](#rust)
- [Tool Namespacing](#tool-namespacing)
- [Best Practices](#best-practices)

---

## Architecture

Every plugin is a **driver** — a subprocess that implements the `Driver` gRPC interface. A single driver can expose multiple plugin type capabilities simultaneously (e.g., both tools and a provider).

```
┌─────────────────────────────────────────────────────┐
│  forge agent                                        │
│                                                     │
│  PluginRegistry                                     │
│    ├─ driver "ollama"  ──gRPC──► ollama subprocess  │
│    ├─ driver "skills"  ──gRPC──► forge plugin skills│
│    └─ driver "search"  ──gRPC──► forge plugin searxng
└─────────────────────────────────────────────────────┘
```

### Embedded vs External

Forge supports two plugin delivery modes:

| Mode | When used | How |
|---|---|---|
| **External binary** | Binary `{plugin_dir}/{type}` exists | Forge spawns the binary directly |
| **Embedded** | No binary found | Forge runs `forge plugin {type}` on itself |

When a plugin block references type `"ollama"`, forge first checks `{plugin_dir}/ollama`. If that file does not exist, it falls back to calling its own binary as `forge plugin ollama`. Embedded plugins register themselves via Go's `init()` mechanism.

The subprocess communicates via gRPC. The handshake uses:

```
Magic cookie key:   FORGE_PLUGIN
Magic cookie value: forge
Protocol version:   2
```

---

## Plugin Types

A driver reports which types it supports through `GetCapabilities()`. The four available types are:

| Type constant | String | Purpose |
|---|---|---|
| `PluginTypeTools` | `"tools"` | Expose callable tools to the agent |
| `PluginTypeProvider` | `"provider"` | LLM chat and embedding endpoints |
| `PluginTypeMemory` | `"memory"` | Vector storage and session management |
| `PluginTypeChannel` | `"channel"` | Send/receive messages (Discord, Slack, …) |

A driver may support any combination of these types.

---

## The Driver Interface

Every plugin must implement `plugins.Driver`. This is the root interface that forge uses for lifecycle management.

```go
type Driver interface {
    // Metadata and capability discovery
    GetPluginInfo() PluginInfo
    ProbePlugin(ctx context.Context) (bool, error)
    GetCapabilities(ctx context.Context) (*DriverCapabilities, error)

    // Lifecycle
    OpenDriver(ctx context.Context) error
    CloseDriver(ctx context.Context) error
    ConfigDriver(ctx context.Context, config PluginConfig) error

    // Plugin type accessors — return ErrPluginNotSupported if not implemented
    GetProviderPlugin(ctx context.Context) (ProviderPlugin, error)
    GetMemoryPlugin(ctx context.Context)   (MemoryPlugin, error)
    GetChannelPlugin(ctx context.Context)  (ChannelPlugin, error)
    GetToolsPlugin(ctx context.Context)    (ToolsPlugin, error)
}
```

**Lifecycle order** for each plugin instance:

1. `OpenDriver` — establish connections, allocate resources
2. `ConfigDriver` — receive HCL config decoded as `map[string]any`
3. `GetCapabilities` — forge reads which types are supported
4. *(plugin serves requests)*
5. `CloseDriver` — release all resources

For unsupported plugin types, return `errors.ErrPluginNotSupported` from the corresponding accessor.

### DriverCapabilities

```go
type DriverCapabilities struct {
    Types    []string              // list of supported PluginType* constants
    Provider *ProviderCapabilities
    Memory   *MemoryCapabilities
    Channel  *ChannelCapabilities
    Tools    *ToolsCapabilities
}

type ToolsCapabilities struct {
    SupportsAsyncExecution bool
}

type ProviderCapabilities struct {
    SupportsStreaming bool
    SupportsVision   bool
}
```

Only populate the capability struct for types you advertise in `Types`.

---

## HCL Configuration

Plugins are declared in the agent config file. The block has two labels — **name** and **type** — followed by an optional `config` block:

```hcl
plugin "name" "type" {
    config {
        key   = "value"
        count = 10
        list  = ["a", "b"]
    }
}
```

- **name** — the unique instance name used for routing (e.g., `skills/list_files`)
- **type** — the plugin type, determines which binary or embedded plugin to load

Multiple instances of the same type are supported:

```hcl
plugin "home"   "filesystem" { config { home = "/home/user" } }
plugin "work"   "filesystem" { config { home = "/workspace" } }
plugin "search" "searxng"    { config { address = "https://search.example.com" } }
```

The config block contents are decoded into a `map[string]any` and passed to `ConfigDriver`. Use `mapstructure` to decode into a typed struct:

```go
type MyConfig struct {
    Address string   `mapstructure:"address"`
    Timeout int      `mapstructure:"timeout"`
    Tags    []string `mapstructure:"tags"`
}

func (d *MyDriver) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
    var cfg MyConfig
    if err := mapstructure.Decode(config.ConfigMap, &cfg); err != nil {
        return fmt.Errorf("failed to decode config: %w", err)
    }
    d.config = &cfg
    return nil
}
```

---

## Implementing a Plugin in Go

### Embedded Plugin

Embedded plugins live in `plugins/{name}/` and register themselves via `init()`. Forge's main binary imports them with a blank import.

#### 1. Register the driver

```go
// plugins/myplugin/driver.go
package myplugin

import (
    "github.com/hashicorp/go-hclog"
    "github.com/mwantia/forge/pkg/plugins"
)

const PluginName = "myplugin"

func init() {
    plugins.Register(PluginName, NewMyDriver)
}

type MyDriver struct {
    plugins.UnimplementedToolsPlugin
    log    hclog.Logger
    config *MyConfig
}

type MyConfig struct {
    Endpoint string   `mapstructure:"endpoint"`
    Tools    []string `mapstructure:"tools"`
}

func NewMyDriver(log hclog.Logger) plugins.Driver {
    return &MyDriver{log: log.Named(PluginName)}
}
```

#### 2. Implement the Driver interface

```go
func (d *MyDriver) GetPluginInfo() plugins.PluginInfo {
    return plugins.PluginInfo{Name: PluginName, Author: "you", Version: "0.1.0"}
}

func (d *MyDriver) ProbePlugin(_ context.Context) (bool, error) { return true, nil }

func (d *MyDriver) GetCapabilities(_ context.Context) (*plugins.DriverCapabilities, error) {
    return &plugins.DriverCapabilities{
        Types: []string{plugins.PluginTypeTools},
        Tools: &plugins.ToolsCapabilities{},
    }, nil
}

func (d *MyDriver) OpenDriver(_ context.Context) error  { return nil }
func (d *MyDriver) CloseDriver(_ context.Context) error { return nil }

func (d *MyDriver) ConfigDriver(_ context.Context, config plugins.PluginConfig) error {
    d.config = &MyConfig{Tools: []string{"greet"}}
    return mapstructure.Decode(config.ConfigMap, d.config)
}

func (d *MyDriver) GetProviderPlugin(_ context.Context) (plugins.ProviderPlugin, error) {
    return nil, errors.ErrPluginNotSupported
}
func (d *MyDriver) GetMemoryPlugin(_ context.Context) (plugins.MemoryPlugin, error) {
    return nil, errors.ErrPluginNotSupported
}
func (d *MyDriver) GetChannelPlugin(_ context.Context) (plugins.ChannelPlugin, error) {
    return nil, errors.ErrPluginNotSupported
}
func (d *MyDriver) GetToolsPlugin(_ context.Context) (plugins.ToolsPlugin, error) {
    return d, nil
}
```

#### 3. Implement the ToolsPlugin interface

```go
var toolDefs = map[string]plugins.ToolDefinition{
    "greet": {
        Name:        "greet",
        Description: "Returns a greeting for the given name",
        Tags:        []string{"demo"},
        Annotations: plugins.ToolAnnotations{ReadOnly: true, CostHint: "free"},
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "name": map[string]any{"type": "string", "description": "Name to greet"},
            },
            "required": []string{"name"},
        },
    },
}

func (d *MyDriver) GetLifecycle() plugins.Lifecycle { return d }

func (d *MyDriver) ListTools(_ context.Context, _ plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
    tools := make([]plugins.ToolDefinition, 0)
    for _, name := range d.config.Tools {
        if def, ok := toolDefs[name]; ok {
            tools = append(tools, def)
        }
    }
    return &plugins.ListToolsResponse{Tools: tools}, nil
}

func (d *MyDriver) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
    def, ok := toolDefs[name]
    if !ok {
        return nil, fmt.Errorf("tool %q not found", name)
    }
    return &def, nil
}

func (d *MyDriver) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
    if _, ok := req.Arguments["name"]; !ok {
        return &plugins.ValidateResponse{Valid: false, Errors: []string{`"name" is required`}}, nil
    }
    return &plugins.ValidateResponse{Valid: true}, nil
}

func (d *MyDriver) Execute(_ context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
    name, _ := req.Arguments["name"].(string)
    return &plugins.ExecuteResponse{
        Result: map[string]any{"message": "Hello, " + name + "!"},
    }, nil
}
```

#### 4. Import in main.go

```go
// cmd/forge/main.go
import (
    _ "github.com/mwantia/forge/plugins/myplugin"
)
```

### External Binary Plugin

An external plugin is a standalone binary placed in `plugin_dir`. It must serve itself over gRPC using the same protocol.

```go
// cmd/myplugin/main.go
package main

import (
    "github.com/hashicorp/go-hclog"
    "github.com/mwantia/forge/pkg/plugins/grpc"
    // import your driver package
    "github.com/you/myplugin"
)

func main() {
    grpc.Serve(myplugin.NewMyDriver)
}
```

Build and place the binary at `{plugin_dir}/{type}`:

```bash
go build -o ./plugins/myplugin ./cmd/myplugin/main.go
```

Config references it by type:

```hcl
plugin_dir = "./plugins"

plugin "myplugin" "myplugin" {
    config { endpoint = "http://localhost:9000" }
}
```

---

## Plugin Types in Detail

### Tools Plugin

Tools are callable functions exposed to the LLM agent. The agent decides when to call them based on their descriptions and the conversation context.

```go
type ToolsPlugin interface {
    BasePlugin
    ListTools(ctx context.Context, filter ListToolsFilter) (*ListToolsResponse, error)
    GetTool(ctx context.Context, name string) (*ToolDefinition, error)
    Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)
    ExecuteStream(ctx context.Context, req ExecuteRequest) (<-chan ExecuteChunk, error)
    Cancel(ctx context.Context, callID string) error
    Validate(ctx context.Context, req ExecuteRequest) (*ValidateResponse, error)
}
```

Embed `plugins.UnimplementedToolsPlugin` to get no-op defaults for `ExecuteStream` and `Cancel` if you don't need them.

**Tool definition** describes the tool to the LLM using JSON Schema for parameters:

```go
plugins.ToolDefinition{
    Name:        "search",
    Description: "Search the web and return results",
    Tags:        []string{"web", "search"},
    Annotations: plugins.ToolAnnotations{
        ReadOnly:   true,
        Idempotent: true,
        CostHint:   "cheap", // "free" | "cheap" | "expensive"
    },
    Parameters: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "query":       map[string]any{"type": "string"},
            "num_results": map[string]any{"type": "integer"},
        },
        "required": []string{"query"},
    },
}
```

**Tool annotations** guide agent behavior:

| Annotation | Effect |
|---|---|
| `ReadOnly` | Tool produces no side-effects |
| `Destructive` | Action cannot be undone |
| `Idempotent` | Safe to retry multiple times |
| `RequiresConfirmation` | Agent should ask the user before calling |
| `CostHint` | Hints at relative cost: `"free"`, `"cheap"`, or `"expensive"` |

**Execute** returns either a result or an error. Return `IsError: true` for tool-level errors that the agent should see, not Go errors:

```go
func (d *MyDriver) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
    // Tool-level error (agent sees this as a tool result)
    if req.Arguments["query"] == nil {
        return &plugins.ExecuteResponse{Result: "query is required", IsError: true}, nil
    }

    result, err := doWork(ctx, req.Arguments)
    if err != nil {
        // System error — agent retries or aborts
        return nil, fmt.Errorf("internal error: %w", err)
    }

    return &plugins.ExecuteResponse{
        Result: map[string]any{
            "data": result,
        },
    }, nil
}
```

**ListToolsFilter** allows clients to narrow the tool list:

```go
type ListToolsFilter struct {
    Tags       []string // only tools with at least one matching tag
    Deprecated bool     // include deprecated tools (default: false)
    Prefix     string   // only tools whose name starts with this prefix
}
```

---

### Provider Plugin

A provider exposes an LLM endpoint. Models are routed as `"provider/model"` (e.g., `"ollama/llama3"`).

```go
type ProviderPlugin interface {
    BasePlugin
    Chat(ctx context.Context, messages []ChatMessage, tools []ToolCall, model *Model) (ChatStream, error)
    Embed(ctx context.Context, content string, model *Model) ([][]float32, error)
    ListModels(ctx context.Context) ([]*Model, error)
    CreateModel(ctx context.Context, modelName string, template *ModelTemplate) (*Model, error)
    GetModel(ctx context.Context, name string) (*Model, error)
    DeleteModel(ctx context.Context, name string) (bool, error)
}
```

**Chat** returns a `ChatStream`. Implement it as a channel or struct with `Recv()` and `Close()`:

```go
type ChatStream interface {
    Recv() (*ChatChunk, error) // returns io.EOF when done
    Close() error
}

type ChatChunk struct {
    ID        string
    Role      string
    Delta     string         // incremental text
    Done      bool           // true on final chunk
    ToolCalls []ChatToolCall // populated on last chunk if tools were called
    Metadata  map[string]any
}
```

Use `plugins.CollectStream(stream)` to drain a `ChatStream` into a single `ChatResult`.

**Message roles** follow the standard convention: `"system"`, `"user"`, `"assistant"`, `"tool"`.

**Capabilities** for a provider:

```go
&plugins.DriverCapabilities{
    Types: []string{plugins.PluginTypeProvider},
    Provider: &plugins.ProviderCapabilities{
        SupportsStreaming: true,
        SupportsVision:   false,
    },
}
```

---

### Memory Plugin

A memory plugin stores and retrieves information, typically backed by a vector database.

```go
type MemoryPlugin interface {
    BasePlugin
    StoreResource(ctx context.Context, sessionID, content string, metadata map[string]any) (*MemoryResource, error)
    RetrieveResource(ctx context.Context, sessionID, query string, limit int, filter map[string]any) ([]*MemoryResource, error)

    CreateSession(ctx context.Context) (*MemorySession, error)
    GetSession(ctx context.Context, sessionID string) (*MemorySession, error)
    ListSessions(ctx context.Context) ([]*MemorySession, error)
    DeleteSession(ctx context.Context, sessionID string) (bool, error)
    CommitSession(ctx context.Context, sessionID string) (bool, error)

    AddMessage(ctx context.Context, sessionID, role, content string) (bool, error)
}
```

`RetrieveResource` should perform a semantic (vector) search and return results ranked by relevance. The `Score` field on `MemoryResource` represents similarity (higher is more relevant).

---

### Channel Plugin

A channel plugin sends and receives messages from an external service such as Discord or Slack.

```go
type ChannelPlugin interface {
    BasePlugin
    Send(ctx context.Context, channel, content string, metadata map[string]any) (string, error)
    Receive(ctx context.Context) (<-chan ChannelMessage, error)
}

type ChannelMessage struct {
    ID       string
    Channel  string
    Author   string
    Content  string
    Metadata map[string]any
}
```

`Receive` returns a channel that the caller reads from continuously. Close the channel when the context is cancelled.

---

## Implementing in Other Languages

External plugins in any language must:

1. Check the magic cookie environment variable or exit immediately
2. Start a gRPC server implementing the required proto services
3. Print the go-plugin handshake line to stdout and then block

The proto files are in `pkg/plugins/grpc/{driver,tools,provider,memory,channel}/proto/`.

### Handshake Protocol

go-plugin communicates the server address via stdout. Before serving, print exactly:

```
1|2|tcp|HOST:PORT|grpc\n
```

Where `1` is the core protocol version (always `1`), `2` is the app protocol version (always `2` for forge), and `HOST:PORT` is where your gRPC server is listening.

The parent process also sets the environment variable `FORGE_PLUGIN=forge`. Your plugin must verify this before starting. If it is absent or wrong, exit with a non-zero code.

### Python

Generate the gRPC stubs from the proto files:

```bash
python -m grpc_tools.protoc \
  -I. \
  --python_out=./stubs \
  --grpc_python_out=./stubs \
  pkg/plugins/grpc/driver/proto/driver.proto \
  pkg/plugins/grpc/driver/proto/common.proto \
  pkg/plugins/grpc/tools/proto/tools.proto
```

Implement and serve:

```python
# myplugin.py
import os
import sys
import socket
import threading
import concurrent.futures
from google.protobuf import struct_pb2, value_pb2

import grpc
from stubs import driver_pb2, driver_pb2_grpc, tools_pb2, tools_pb2_grpc

# --- Verify magic cookie ---
if os.environ.get("FORGE_PLUGIN") != "forge":
    print("This is a forge plugin binary and cannot be run directly.", file=sys.stderr)
    sys.exit(1)

# --- Implement services ---

class DriverServicer(driver_pb2_grpc.DriverServiceServicer):
    def GetPluginInfo(self, request, context):
        info = driver_pb2.PluginInfo(name="myplugin", author="you", version="0.1.0")
        return driver_pb2.GetPluginInfoResponse(info=info)

    def ProbePlugin(self, request, context):
        return driver_pb2.ProbeResponse(ok=True)

    def GetCapabilities(self, request, context):
        caps = driver_pb2.DriverCapabilities(
            types=["tools"],
            tools=driver_pb2.ToolsCapabilities(supports_async_execution=False),
        )
        return driver_pb2.CapabilitiesResponse(capabilities=caps)

    def OpenDriver(self, request, context):
        return driver_pb2.OpenResponse()

    def CloseDriver(self, request, context):
        return driver_pb2.CloseResponse()

    def ConfigDriver(self, request, context):
        # request.config is a google.protobuf.Struct
        cfg = dict(request.config)
        # parse your config here
        return driver_pb2.ConfigResponse()

    def GetToolsPlugin(self, request, context):
        return driver_pb2.GetPluginResponse(available=True)

    def GetProviderPlugin(self, request, context):
        return driver_pb2.GetPluginResponse(available=False)

    def GetMemoryPlugin(self, request, context):
        return driver_pb2.GetPluginResponse(available=False)

    def GetChannelPlugin(self, request, context):
        return driver_pb2.GetPluginResponse(available=False)


class ToolsServicer(tools_pb2_grpc.ToolsServiceServicer):
    def ListTools(self, request, context):
        params = struct_pb2.Struct()
        params.update({"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]})
        tool = tools_pb2.ToolDefinitionProto(
            name="greet",
            description="Greet a person by name",
            parameters=params,
            tags=["demo"],
        )
        return tools_pb2.ListToolsResponse(tools=[tool])

    def GetTool(self, request, context):
        # return a specific tool definition
        pass

    def Execute(self, request, context):
        args = dict(request.arguments)
        name = args.get("name", "World")
        result = struct_pb2.Value()
        result.string_value = f"Hello, {name}!"
        return tools_pb2.ExecuteResponse(result=result, is_error=False)

    def Validate(self, request, context):
        args = dict(request.arguments)
        errors = []
        if "name" not in args:
            errors.append('"name" is required')
        return tools_pb2.ValidateResponse(valid=len(errors) == 0, errors=errors)

    def Cancel(self, request, context):
        return tools_pb2.CancelResponse(ok=False)


# --- Start gRPC server ---

server = grpc.server(concurrent.futures.ThreadPoolExecutor(max_workers=10))
driver_pb2_grpc.add_DriverServiceServicer_to_server(DriverServicer(), server)
tools_pb2_grpc.add_ToolsServiceServicer_to_server(ToolsServicer(), server)

# Find a free port
sock = socket.socket()
sock.bind(("127.0.0.1", 0))
port = sock.getsockname()[1]
sock.close()

server.add_insecure_port(f"127.0.0.1:{port}")
server.start()

# --- Print handshake line (go-plugin protocol) ---
# Format: 1|{proto_version}|{network}|{address}|grpc
sys.stdout.write(f"1|2|tcp|127.0.0.1:{port}|grpc\n")
sys.stdout.flush()

# Block until interrupted
server.wait_for_termination()
```

Place the script (or a compiled binary) at `{plugin_dir}/myplugin`.

### Rust

Use the [`tonic`](https://github.com/hyperium/tonic) crate for gRPC. Generate stubs from the proto files with `tonic-build` in your `build.rs`.

```toml
# Cargo.toml
[dependencies]
tonic    = "0.12"
tokio    = { version = "1", features = ["full"] }
prost    = "0.13"

[build-dependencies]
tonic-build = "0.12"
```

```rust
// build.rs
fn main() {
    tonic_build::configure()
        .compile(
            &[
                "pkg/plugins/grpc/driver/proto/driver.proto",
                "pkg/plugins/grpc/driver/proto/common.proto",
                "pkg/plugins/grpc/tools/proto/tools.proto",
            ],
            &["."],
        )
        .unwrap();
}
```

```rust
// src/main.rs
use std::env;
use std::net::TcpListener;
use tonic::transport::Server;

// include generated stubs
mod driver { tonic::include_proto!("driver"); }
mod tools  { tonic::include_proto!("tools");  }

use driver::driver_service_server::{DriverService, DriverServiceServer};
use tools::tools_service_server::{ToolsService, ToolsServiceServer};

// --- Verify magic cookie ---
fn check_magic_cookie() {
    match env::var("FORGE_PLUGIN") {
        Ok(v) if v == "forge" => {}
        _ => {
            eprintln!("This is a forge plugin binary and cannot be run directly.");
            std::process::exit(1);
        }
    }
}

// --- Driver implementation ---

#[derive(Default)]
struct MyDriver;

#[tonic::async_trait]
impl DriverService for MyDriver {
    async fn get_plugin_info(&self, _: tonic::Request<driver::GetPluginInfoRequest>)
        -> Result<tonic::Response<driver::GetPluginInfoResponse>, tonic::Status>
    {
        let info = driver::PluginInfo {
            name: "myplugin".into(),
            author: "you".into(),
            version: "0.1.0".into(),
        };
        Ok(tonic::Response::new(driver::GetPluginInfoResponse { info: Some(info) }))
    }

    async fn probe_plugin(&self, _: tonic::Request<driver::ProbeRequest>)
        -> Result<tonic::Response<driver::ProbeResponse>, tonic::Status>
    {
        Ok(tonic::Response::new(driver::ProbeResponse { ok: true }))
    }

    async fn get_capabilities(&self, _: tonic::Request<driver::CapabilitiesRequest>)
        -> Result<tonic::Response<driver::CapabilitiesResponse>, tonic::Status>
    {
        let caps = driver::DriverCapabilities {
            types: vec!["tools".into()],
            tools: Some(driver::ToolsCapabilities { supports_async_execution: false }),
            ..Default::default()
        };
        Ok(tonic::Response::new(driver::CapabilitiesResponse { capabilities: Some(caps) }))
    }

    async fn open_driver(&self, _: tonic::Request<driver::OpenRequest>)
        -> Result<tonic::Response<driver::OpenResponse>, tonic::Status>
    {
        Ok(tonic::Response::new(driver::OpenResponse {}))
    }

    async fn close_driver(&self, _: tonic::Request<driver::CloseRequest>)
        -> Result<tonic::Response<driver::CloseResponse>, tonic::Status>
    {
        Ok(tonic::Response::new(driver::CloseResponse {}))
    }

    async fn config_driver(&self, _: tonic::Request<driver::ConfigRequest>)
        -> Result<tonic::Response<driver::ConfigResponse>, tonic::Status>
    {
        Ok(tonic::Response::new(driver::ConfigResponse {}))
    }

    async fn get_tools_plugin(&self, _: tonic::Request<driver::GetPluginRequest>)
        -> Result<tonic::Response<driver::GetPluginResponse>, tonic::Status>
    {
        Ok(tonic::Response::new(driver::GetPluginResponse { available: true }))
    }

    async fn get_provider_plugin(&self, _: tonic::Request<driver::GetPluginRequest>)
        -> Result<tonic::Response<driver::GetPluginResponse>, tonic::Status>
    {
        Ok(tonic::Response::new(driver::GetPluginResponse { available: false }))
    }

    async fn get_memory_plugin(&self, _: tonic::Request<driver::GetPluginRequest>)
        -> Result<tonic::Response<driver::GetPluginResponse>, tonic::Status>
    {
        Ok(tonic::Response::new(driver::GetPluginResponse { available: false }))
    }

    async fn get_channel_plugin(&self, _: tonic::Request<driver::GetPluginRequest>)
        -> Result<tonic::Response<driver::GetPluginResponse>, tonic::Status>
    {
        Ok(tonic::Response::new(driver::GetPluginResponse { available: false }))
    }
}

// --- Tools implementation ---

#[derive(Default)]
struct MyTools;

#[tonic::async_trait]
impl ToolsService for MyTools {
    async fn list_tools(&self, _: tonic::Request<tools::ListToolsRequest>)
        -> Result<tonic::Response<tools::ListToolsResponse>, tonic::Status>
    {
        let tool = tools::ToolDefinitionProto {
            name: "greet".into(),
            description: "Greet a person by name".into(),
            tags: vec!["demo".into()],
            ..Default::default()
        };
        Ok(tonic::Response::new(tools::ListToolsResponse { tools: vec![tool] }))
    }

    async fn execute(&self, req: tonic::Request<tools::ExecuteRequest>)
        -> Result<tonic::Response<tools::ExecuteResponse>, tonic::Status>
    {
        let args = req.into_inner().arguments.unwrap_or_default();
        let name = args.fields.get("name")
            .and_then(|v| v.kind.as_ref())
            .and_then(|k| if let prost_types::value::Kind::StringValue(s) = k { Some(s.as_str()) } else { None })
            .unwrap_or("World");

        let result = prost_types::Value {
            kind: Some(prost_types::value::Kind::StringValue(format!("Hello, {}!", name))),
        };
        Ok(tonic::Response::new(tools::ExecuteResponse { result: Some(result), is_error: false, metadata: None }))
    }

    // implement remaining methods (get_tool, validate, cancel, execute_stream) ...
}

#[tokio::main]
async fn main() {
    check_magic_cookie();

    // Bind to a random port
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let port = listener.local_addr().unwrap().port();
    drop(listener);

    let addr = format!("127.0.0.1:{}", port).parse().unwrap();

    // Print go-plugin handshake to stdout BEFORE serving
    println!("1|2|tcp|127.0.0.1:{}|grpc", port);
    std::io::Write::flush(&mut std::io::stdout()).unwrap();

    Server::builder()
        .add_service(DriverServiceServer::new(MyDriver))
        .add_service(ToolsServiceServer::new(MyTools))
        .serve(addr)
        .await
        .unwrap();
}
```

---

## Tool Namespacing

When forge registers tools from a plugin, it prefixes each tool name with the plugin instance name:

```
{plugin-name}/{tool-name}
```

For example, a plugin named `"search"` with tool `"web_search"` is exposed to the agent as `search/web_search`. The prefix is stripped before calling `Execute()`, so your plugin receives just `"web_search"` in `req.Tool`.

This means you can have multiple plugins with the same tool names without conflicts:

```hcl
plugin "home"   "filesystem" { config { home = "/home/user" } }
plugin "work"   "filesystem" { config { home = "/workspace" } }
```

→ Tools become `home/read`, `home/list`, `work/read`, `work/list`.

---

## Best Practices

**Return tool errors, not Go errors.** If arguments are invalid or the external service returns an error the agent should handle, return `&ExecuteResponse{IsError: true}`. Reserve Go errors for unexpected system failures.

**Validate early.** Implement `Validate()` to catch bad arguments before `Execute()` is called. The session pipeline may call `Validate` before `Execute` to skip clearly broken calls.

**Use config defaults.** Apply sensible defaults in `ConfigDriver` so plugins work without requiring every field:

```go
if cfg.Timeout <= 0 {
    cfg.Timeout = 30
}
```

**Report capabilities accurately.** Only list types in `GetCapabilities().Types` that you actually implement. Forge uses this to avoid calling unsupported accessors.

**Respect context cancellation.** Pass the `ctx` parameter to any HTTP calls, database queries, or blocking operations. The agent can cancel a tool call at any time.

**Annotate tools carefully.** The `CostHint`, `Destructive`, and `RequiresConfirmation` annotations change how the agent plans and presents tool calls. Use them honestly.

**Log with the provided logger.** Use the `hclog.Logger` passed to your factory function — it is already configured to write structured JSON to stderr and will not interfere with the go-plugin stdout handshake.
