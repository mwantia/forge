# Forge Service — Roadmap

Centralized list of missing features, known gaps, and planned work. Items are
grouped by subsystem and roughly ordered by payoff-to-effort ratio within each
group. Source proposals live in `../docs/`.

---

## 0. DAG & storage ← current focus

Goal: feature- and production-complete before moving to other areas.

### 0a. Data integrity

| Item | Status | Notes |
|---|---|---|
| Atomic writes — file backend | **Done** | `WriteRaw` writes to `.tmp` then `os.Rename`. |
| `WriteJson` uses `MarshalIndent` | **Done** | Both backends now use `json.Marshal`. |
| `PutIfAbsent` read-then-write race | **Low priority** | Two concurrent commits with the same hash both pass `Has` and both write. Harmless (content-addressed), but worth a note until a backend-level CAS primitive exists. |

### 0b. Performance

| Item | Status | Notes |
|---|---|---|
| Object compression | **Done** | `ObjectStore.Put` gzip-compresses; `GetRaw` decompresses. Legacy uncompressed objects pass through transparently. |
| Postgres `ListEntry` index | **Done** | `CREATE INDEX IF NOT EXISTS forge_kv_prefix ON forge_kv (key text_pattern_ops)` added to schema init. |

### 0c. Tooling

| Item | Status | Notes |
|---|---|---|
| Automatic GC trigger | **Won't fix** | `forge system gc` is sufficient; users can schedule it via cron. Background tickers are explicitly avoided in Forge. |
| `commit_session` tool | **Done** | Registered as `pipeline__commit_session`; calls `PipelineService.CommitSync` which blocks until the sub-session's tool-loop finishes. |

### 0d. `forge system dag` — inspect & debug

Online subcommand group: CLI calls a running agent over HTTP (same
`--http-addr` / `--http-token` flags as `forge system gc`). Auth is the
existing bearer-token mechanism; no direct storage access from the client.

New agent-side routes live under `GET|POST /v1/system/dag/...`, mounted by
the existing `SystemService` (or a new `DagService` if it grows large). All
routes are auth-gated.

| CLI subcommand | HTTP endpoint | Status |
|---|---|---|
| `dag cat <hash>` | `GET /v1/system/dag/objects/<hash>` | **Done** |
| `dag type <hash>` | `GET /v1/system/dag/objects/<hash>/type` | **Done** |
| `dag log <session> [--ref <name>]` | `GET /v1/system/dag/sessions/<id>/log[?ref=<name>]` | **Done** |
| `dag diff <hash-a> <hash-b>` | `GET /v1/system/dag/diff?a=<hash>&b=<hash>` | **Done** |
| `dag refs <session>` | `GET /v1/sessions/<id>/refs` | **Done** |
| `dag verify <session> [--ref <name>] [--all]` | `POST /v1/system/dag/verify` | **Done** |
| `dag objects [--prefix <xx>]` | `GET /v1/system/dag/objects[?prefix=<xx>&list=true]` | **Done** |
| `dag gc [--dry-run]` | `POST /v1/system/dag/gc[?dry_run=true]` | **Done** |

CLI output is plain text by default; `--json` emits newline-delimited JSON for scripting.

---

## 1. Session subsystem

| Item | Status | Notes |
|---|---|---|
| `commit_session` tool | **Missing** | Returns `"not yet implemented"`. See `internal/service/session/tools.go:59`. The other sub-session tools are wired. |
| `PATCH /messages/summarize` | **Stub (501)** | `handlers.go:354`. Needs a pipeline round-trip: assemble history, call provider with a summarize prompt, replace branch with a single summary message. |
| Automatic GC trigger | **Won't fix** | `forge system gc` is sufficient; schedule via cron if needed. Background tickers explicitly avoided. |

---

## 2. Agent lifecycle

Proposal: `../docs/07-proposal-agent-lifecycle-runner.md`

| Item | Status | Notes |
|---|---|---|
| `container.Cleanup` fan-out | **Incomplete** | `Agent.Cleanup` manually calls three services; `sess`, `res`, `events`, `providers`, and `tools` are silently skipped. Fix: call `container.Cleanup(ctx)` after the manual plugins teardown. |
| `Serve` via errgroup | **Not started** | Long-lived services (`srv`, `met`, `pipe`) are goroutined manually. Wrapping them in an `errgroup` with propagating cancellation would surface errors and stop the agent cleanly on first failure. |
| Sequential-init vs. concurrent-loop split | **Not started** | `providers`, `toolsSvc`, `sess`, `res`, `events` run to completion; `srv`, `met`, `pipe` are loops. A `service.Runner` abstraction would make this explicit and remove the boilerplate in `agent.go`. |

---

## 3. System prompt

Proposal: `../docs/02-proposal-system-prompt-composition.md`  
Proposal: `../docs/05-proposal-system-snapshot-and-cli-normalization.md`

| Item | Status | Notes |
|---|---|---|
| Defined layer order | **Partial** | `pipeline/prompt.go` assembles layers but order is not formally guaranteed and cache-stability properties are not enforced. Target: agent → model → plugins (alphabetic) → session → resources. |
| Plugin-level `System()` contribution | **SDK defined, not consumed** | `DriverCapabilities` has no `System` field yet; service assembler does not call it. |
| Per-tool `ToolAnnotations.Prompt` | **Not started** | SDK addition needed; assembler would inject per-tool prose after each plugin's blurb. |
| System snapshot as DAG ref | **Not started** | Store the assembled static prompt as a `system` ref on first dispatch; subsequent turns load from the object store instead of re-assembling. Makes replays fully reproducible. |
| `forge sessions system show/edit/regen` | **Not started** | CLI to inspect, override, and re-assemble the system prompt for a session. `regen` re-runs the full prompt assembly pipeline and forks HEAD with the new root. |

---

## 4. Configuration tooling

Proposal: `../docs/04-proposal-config-tooling.md`

| Item | Status | Notes |
|---|---|---|
| `forge config generate` | **Not started** | Emit a minimal or full config skeleton. |
| `forge config validate` | **Not started** | Parse + decode against the live schema without starting the agent. |
| `forge config format` | **Not started** | Canonical `hclwrite` formatting. |
| `forge config schema` | **Not started** | Machine-readable JSON schema for editor integrations. |
| `meta {}` block interpolation | **WIP** | `internal/config/processor.go` TODO — `meta.*` values not yet injectable into other blocks. |

---

## 5. Eventflows

Proposal: `../docs/06-proposal-eventflows.md`  
Proposal: `../docs/09-proposal-events-status-command.md`

| Item | Status | Notes |
|---|---|---|
| Event HCL block + named webhook endpoints | **Not started** | Named `event "<id>" { ... }` blocks; `POST /v1/events/<id>` fires a pipeline run. No separate event database — sessions and DAG refs are the only storage. |
| Timespan batching + max queue | **Not started** | Batch window and queue depth guard options on event blocks. |
| `forge events status <name>` | **Not started** | Nomad-style status view with full allocation table (one row per dispatched branch). Replaces `forge events get`. |
| `EventBranch` type + `EventStatus.Branches` | **Not started** | Branch name encodes `event/<id>-<RFC3339>`; `FiredAt` recovered without extra storage. |

---

## 6. Plugin channel service

| Item | Status | Notes |
|---|---|---|
| `internal/service/channel/` | **Not started** | `ChannelPlugin` is defined in the SDK and consumed by `discord`, `unifi`, and `browser` plugins, but no service exists to dispatch to them. No HTTP routes, no DI registration. |
| Channel capability gating | **Not started** | Depends on the service above. `PluginsService` already stores `DriverCapabilities`; channel routing just needs a consumer. |

---

## 7. Sandbox service

| Item | Status | Notes |
|---|---|---|
| `SandboxService` implementation | **Stub** | `internal/service/sandbox/service.go` exists but does nothing. The SDK exports `SandboxPlugin`. No service consumes it. |

---

## 8. Embeddings endpoint

| Item | Status | Notes |
|---|---|---|
| `POST /v1/embeddings` | **Missing** | The old route is absent from the new server layout. Either re-add it under `provider/` (using `ProviderPlugin.Embed`) or formally announce its removal. |

---

## 9. Plugin migration

All plugins have been ported to the `Driver` interface. The ones below are not
yet listed in `plugins.yaml` and therefore excluded from the `all` build tag.

| Plugin | In `plugins.yaml` | Notes |
|---|---|---|
| `openai` | Yes | |
| `skills` | Yes | Needs skills-rework (proposal 08) for progressive disclosure. |
| `searxng` | Yes | |
| `unifi` | Yes | Requires channel service (§6) for full functionality. |
| `plane` | Yes | |
| `consul` | Yes | |
| `nomad` | Yes | |
| `discord` | **No** | Requires channel service. |
| `mcp` | **No** | |
| `browser` | **No** | |

### Skills rework

Proposal: `../docs/08-proposal-skills-rework.md`

The current skills plugin exposes one tool per `SKILL.md` file. The proposed
model exposes four fixed tools (`skill__activate`, `skill__list`, `skill__read_file`,
`skill__execute_script`) and pushes the skill catalog into the system prompt layer.
This caps token cost at session start and scales to large skill sets.

---

## 10. CI

| Item | Status | Notes |
|---|---|---|
| GitHub Actions workflow | **Missing** | No `.github/workflows/` in the repo. Minimum: `task setup && task build` matrix across `service/`, `shared/`, and active plugins, plus a `golangci-lint` job. |

---

## 11. External integrations (out of repo)

| Item | Notes |
|---|---|
| OpenViking `ResourcePlugin` | Lives in a sibling module. Schema_version 1 of the archive envelope is the contract. Plugs into `internal/service/resource/` with no service-side changes required. |
