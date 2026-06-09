## System Template

The system message is the first entry in the session's DAG (`role=system`). It is stored as raw template source and rendered on every commit, exactly like any other message.

### Priority order

The effective system template source is resolved in this order:

1. `system_template` supplied at session creation — stored as the first DAG message immediately
2. `pipeline { system = "..." }` — agent-level default from HCL config, written to DAG on the first commit if no session template was provided
3. Built-in `DefaultSystemTemplate` — the template shipped with the agent binary, used when neither of the above is set

### Setting a custom template at session creation

Pass `system_template` when creating the session. It is stored immediately as the first DAG message.

```bash
curl -X POST http://localhost:9280/v1/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/llama3.2",
    "system_template": "You are a helpful assistant.\n\nToday is {{ date \"2006-01-02\" now }}."
  }'
```

If `system_template` is omitted, the agent-level default is written to the DAG automatically when the first commit is made.

### Changing the system template after creation

The system message is an immutable DAG entry. To use a different system template, create a new session with the desired template. If you need to branch from an existing conversation with a new system prompt, use `?fork_from=<hash>` on the first commit of the new session.

### Updating session pipeline settings

`POST /v1/sessions/:id/system/reset` updates `tools_verbosity` and `plugins` filter on the session metadata — it no longer accepts a `system_template` field.

```bash
curl -X POST http://localhost:9280/v1/sessions/<id>/system/reset \
  -H "Content-Type: application/json" \
  -d '{"tools_verbosity": "full", "plugins": ["skills", "consul"]}'
```

## Default system template

The built-in template rendered when no custom template is configured. Shown here as a reference and starting point for customization:

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
---

## Operational guidelines

- Reach for tools when they are clearly applicable. Prefer the most specific tool over the most general.
- When multiple tools could serve a request, pick the one whose description and guidance best match the user's intent.
- Read each tool's prose carefully — it documents when to use it and when not to.
- Be concise. Surface tool results faithfully. Do not fabricate information you could verify with a tool.
- Before executing any destructive or irreversible action, confirm the user's intent explicitly.
- Use sub-sessions to focus on specific tasks. Once a task is well-defined, create a dedicated session with the appropriate plugins, a clear title, and a description of the plan.
- Keep session metadata current: update the title and description after the first exchange.

---
{{ range $key, $val := .tools.namespaces }}
## {{ $key }}{{ if $val.version }} ({{ $val.version }}){{ end }}{{ if $val.description }} — {{ $val.description }}{{ end }}
{{ if $val.system }}
{{ $val.system }}
{{ end }}
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
> Requires explicit user confirmation before execution.
{{ end }}
{{ end -}}
{{ end }}
```