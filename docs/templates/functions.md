## Functions

All functions are available in both the system template and any message content.

### Rendering

| Function | Description |
|---|---|
| `render str` | Parse and execute `str` as a template with the same variables and functions as the enclosing template. Use when a variable (e.g. `.model.system`) may itself contain template expressions. |

```
{{ render .model.system }}
{{ if .model.system }}{{ render .model.system }}{{ end }}
```

`render` is recursive — the rendered output may call `render` again. Infinite loops are possible; keep nesting depth shallow.

### Time

| Function | Returns | Description |
|---|---|---|
| `now` | string (RFC3339) | Current local time |
| `utcnow` | string (RFC3339) | Current UTC time |
| `unixnow` | int | Current Unix timestamp (seconds) |
| `date format timestamp` | string | Format an RFC3339 timestamp using a Go layout string |

Go time layout reference: `2006-01-02`, `15:04:05`, `Mon Jan 2 2006`, `02 Jan 06 15:04 MST`, etc.

```
{{ date "2006-01-02" now }}                  → 2026-06-08
{{ date "Monday, January 2" now }}           → Sunday, June 8
{{ date "15:04 UTC" utcnow }}                → 14:32 UTC
```

### Strings

| Function | Description |
|---|---|
| `upper str` | Uppercase |
| `lower str` | Lowercase |
| `trimspace str` | Strip leading/trailing whitespace |
| `trim str cutset` | Strip cutset characters from both ends |
| `trimprefix str prefix` | Strip prefix |
| `trimsuffix str suffix` | Strip suffix |
| `chomp str` | Strip trailing newline |
| `replace str old new` | Replace all occurrences |
| `split sep str` | Split into list |
| `join sep list` | Join list into string |
| `substr str offset length` | Substring by byte offset/length (-1 = to end) |
| `strlen str` | String length in bytes |
| `indent spaces str` | Indent all lines after the first by N spaces |
| `format fmt args...` | sprintf-style formatting |

### JSON

| Function | Description |
|---|---|
| `jsonencode value` | Encode value to JSON string |
| `jsondecode str` | Decode JSON string to value |

### Math

| Function | Description |
|---|---|
| `abs n` | Absolute value |
| `ceil n` | Round up |
| `floor n` | Round down |
| `min a b ...` | Minimum |
| `max a b ...` | Maximum |

### IDs and names

| Function | Returns | Description |
|---|---|---|
| `uuid` | string | Random UUID v4 |
| `uuidv6` | string | Time-ordered UUID v6 |
| `uuidv7` | string | Time-ordered UUID v7 |
| `uniquename` | string | Human-readable unique name, e.g. `bold-ritchie-theta` |

### Filesystem

| Function | Description |
|---|---|
| `file path` | Read entire file contents as a string |
| `path path` | Resolve a path to its absolute form |

### Environment

| Function | Description |
|---|---|
| `env name` | Value of the named environment variable (error if unset) |