## Examples

### Date and session context

```
You are assisting {{ .session.name }}.
Today is {{ date "Monday, 2 January 2006" now }}.
Session created: {{ date "2006-01-02" .session.created_at }}.
```

### Session header — title, description, parent lineage

```
**Session:** {{ if .session.title }}{{ .session.title }}{{ else }}{{ .session.name }}{{ end }}{{ if .session.description }} — {{ .session.description }}{{ end }}
{{ if .session.parent -}}
**Derived from:** {{ .session.parent }}
{{ end -}}
**Model:** {{ .model.provider }}/{{ .model.name }}
```

### Runtime and node context

```
Running on {{ .runtime.node.hostname }} ({{ .runtime.node.os.name }} {{ .runtime.node.os.version }}, {{ .runtime.node.arch }}, {{ .runtime.node.cpu_count }} CPUs).
Agent PID: {{ .runtime.pid }}. Working directory: {{ .runtime.cwd }}.
```

### Conditional parent context

```
You are a Forge agent.
{{ if .session.parent }}
This is a sub-session. Parent session: {{ .session.parent }}.
{{ end }}
```

### Model system prose with recursive render

```
{{ if .model.system -}}
{{ render .model.system }}

{{ end -}}
You are a Forge agent. Model: {{ .model.provider }}/{{ .model.name }}.
```

### Selective tool inclusion — only show non-builtin plugins

```
You are a Forge agent.

Available tools:
{{ range $key, $val := .tools.namespaces -}}
{{ if not $val.builtin }}
## {{ $key }} — {{ $val.description }}

{{ $val.system }}
{{ range $def := $val.definitions }}
- **{{ $key }}__{{ $def.name }}**: {{ $def.description }}
{{ end }}
{{ end }}
{{- end }}
```

### Tool listing with safety annotations

```
{{ range $key, $val := .tools.namespaces -}}
## {{ $key }}{{ if $val.description }} — {{ $val.description }}{{ end }}

{{ range $def := $val.definitions -}}
### {{ $key }}__{{ $def.name }}
{{ if $def.annotations.system -}}
{{ $def.annotations.system }}
{{- else -}}
{{ $def.description }}
{{- end }}
{{ if $def.annotations.destructive -}}
> **Destructive.** Confirm before use.
{{ end -}}
{{ if $def.annotations.requires_confirmation -}}
> Requires explicit user confirmation.
{{ end -}}
{{ end -}}
{{ end -}}
```

### Load system prose from a file

```
{{ file "/etc/forge/agent-persona.txt" }}

Available tools:
{{ range $key, $val := .tools.namespaces }}
## {{ $key }}
{{ $val.system }}
{{ end }}
```

### Environment-aware behaviour

```
You are a Forge agent operating in the {{ env "FORGE_ENV" }} environment.
{{ if eq (env "FORGE_ENV") "production" }}
Be conservative. Prefer read-only tools and confirm before any destructive action.
{{ end }}
```

### Custom tool layout — compact list style

```
You are a Forge agent.

The following tools are available. Use the exact `namespace__name` form when calling them.

{{ range $key, $val := .tools.namespaces -}}
**{{ $key }}** — {{ $val.description }}
{{ range $def := $val.definitions }}  - `{{ $key }}__{{ $def.name }}`: {{ $def.description }}
{{ end }}
{{ end }}
```

### Full production-style header

Combines date, session context, node info, and model prose into the preamble used by the built-in default:

```
{{ if .model.system }}{{ render .model.system }}

{{ end -}}
You are a Forge agent — an LLM-driven assistant that orchestrates work through a curated set of tools provided by loaded plugins.

**Date:** {{ date "Monday, 2 January 2006" now }}
**Session:** {{ if .session.title }}{{ .session.title }}{{ else }}{{ .session.name }}{{ end }}{{ if .session.description }} — {{ .session.description }}{{ end }}
**Node:** {{ .runtime.node.hostname }} · {{ .runtime.node.os.name }} {{ .runtime.node.os.version }} · {{ .runtime.node.arch }}
{{ if .session.parent -}}
**Derived from:** {{ .session.parent }}
{{ end }}
```

## Example Dataset

```json
{
    "session": {
        "id": "6cec0a724fb55e226bfdc8055affa0a2",
        "name": "test-session",
        "title": "Templating Test Session - Full Render Test",
        "description": "Test session created to ",
        "parent": "5f95edafe49ee982adb29ed11c245f0f",
        "model": "ollama/glm-5.1:cloud",
        "mode": "research",
        "created_at": "2026-06-08T12:04:12.670681882+02:00",
        "updated_at": "2026-06-09T02:10:59.506715243+02:00"
    },
    "runtime": {
        "version": "go1.25.0",
        "pid": 1000,
        "uid": 1000,
        "cwd": "/etc/forge.d",
        "node": {
            "hostname": "forge",
            "arch": "amd64",
            "cpu_count": 4,
            "ipv4": "127.0.0.1",
            "os": {
                "name": "ubuntu",
                "version": "24.04"
            }
        }
    },
    "language": {
        "code": "en",
        "name": "English"
    },
    "model": {
        "name": "glm-5.1:cloud",
        "provider": "ollama",
        "system": "",
        "temperature": 0
    },
    "tools": {
        "namespaces": {
            "dummy": {
                "name": "Dummy Namespace",
                "version": "0.0.1",
                "description": "Dummy namesapace declared and used for testing render templating and previews",
                "system": "",
                "builtin": true,
                "definitions": [
                    {
                        "name": "call_test1",
                        "description": "Run test 1 and reply with predefined data",
                        "annotations": {
                            "system": "Extended documentation about 'call_test' to validate and verify verbose outputs.\nOften used in combination with if statements to fallback to 'description' if undefined.",
                            "read_only": true,
                            "idempotent": false,
                            "destructive": false,
                            "requires_confirmation": false
                        }
                    },
                    {
                        "name": "call_test2",
                        "description": "Run test 2 and reply with predefined data",
                        "annotations": {
                            "system": "Extended documentation about 'call_test' to validate and verify verbose outputs.\nOften used in combination with if statements to fallback to 'description' if undefined.",
                            "read_only": true,
                            "idempotent": false,
                            "destructive": false,
                            "requires_confirmation": false
                        }
                    }
                ]
            }
        }
    }
}
```