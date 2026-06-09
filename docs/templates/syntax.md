## Syntax

Templates use Go `text/template` `{{ }}` syntax. HCL `${...}` syntax is available in config files (HCL blocks, `plugin.env`, etc.) but **not** in message content or `system_template`.

```
{{ .session.name }}              — variable access
{{ upper "hello" }}              — function call
{{ if .session.parent }}...{{ end }}   — conditional
{{ range $k, $v := .tools.namespaces }}...{{ end }}  — iteration
{{- ... -}}                      — trim surrounding whitespace
```

Full Go template documentation: https://pkg.go.dev/text/template