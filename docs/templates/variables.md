## Variables

### `session.*`

Available in any message within a session commit. Reflects the session state at render time.

| Variable | Type | Description |
|---|---|---|
| `.session.id` | string | Session UUID |
| `.session.name` | string | Human-readable unique name |
| `.session.title` | string | Optional display title |
| `.session.description` | string | Optional description |
| `.session.parent` | string | Parent session ID (empty for root sessions) |
| `.session.model` | string | Active model reference, e.g. `ollama/llama3.2` |
| `.session.created_at` | string | RFC3339 creation timestamp |
| `.session.updated_at` | string | RFC3339 last-updated timestamp |

### `runtime.*`

Captured once at agent startup. Reflect the host and process.

| Variable | Type | Description |
|---|---|---|
| `.runtime.version` | string | Go runtime version, e.g. `go1.25.0` |
| `.runtime.pid` | int | Agent process ID |
| `.runtime.uid` | int | Effective user ID |
| `.runtime.cwd` | string | Working directory at startup |
| `.runtime.node.hostname` | string | Machine hostname |
| `.runtime.node.arch` | string | CPU architecture, e.g. `amd64`, `arm64` |
| `.runtime.node.cpu_count` | int | Logical CPU count |
| `.runtime.node.ipv4` | string | Primary outbound IPv4 address |
| `.runtime.node.os.name` | string | OS identifier, e.g. `linux`, `darwin` |
| `.runtime.node.os.version` | string | OS pretty-name from `/etc/os-release` |

### `model.*`

The active model's configuration, resolved from the `provider { model "..." { ... } }` block for the session's current model. Available at every commit.

| Variable | Type | Description |
|---|---|---|
| `.model.name` | string | Model alias name, e.g. `prometheus` |
| `.model.provider` | string | Provider name, e.g. `ollama` |
| `.model.system` | string | Raw system prose from the model config (may itself contain template expressions — use `render`) |
| `.model.temperature` | float | Temperature from model config, or `0` if unset |

### `tools.*`

Reflects the live tool registry at the moment of each commit. Always current — a plugin registered after session creation appears on the next commit without any session changes.

```
.tools.namespaces                    — map[name → namespace entry]
.tools.namespaces.<ns>.name          — namespace name (same as map key)
.tools.namespaces.<ns>.version       — plugin version string
.tools.namespaces.<ns>.description   — short namespace description
.tools.namespaces.<ns>.system        — plugin-level system prose
.tools.namespaces.<ns>.builtin       — true for built-in namespaces (sessions, pipeline, resource)
.tools.namespaces.<ns>.definitions   — list of tool definitions

.tools.namespaces.<ns>.definitions[i].name           — bare tool name (without namespace prefix)
.tools.namespaces.<ns>.definitions[i].description    — tool description
.tools.namespaces.<ns>.definitions[i].annotations.system               — extended system prose for this tool
.tools.namespaces.<ns>.definitions[i].annotations.read_only            — bool
.tools.namespaces.<ns>.definitions[i].annotations.idempotent           — bool
.tools.namespaces.<ns>.definitions[i].annotations.destructive          — bool
.tools.namespaces.<ns>.definitions[i].annotations.requires_confirmation — bool
```

Tool calls are addressed as `namespace__name` (double underscore). Construct the full name in a template with `{{ $ns }}__{{ $def.name }}`.