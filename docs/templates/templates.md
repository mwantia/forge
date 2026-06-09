# Template System

Every message stored in Forge — system, user, assistant, tool — is raw template source. Content is rendered on every send, not at write time. This means `{{ date "2006-01-02" now }}` in a user message always shows today's date, and the tool catalog in the system prompt always reflects the currently loaded plugins.

## Error handling

Template render errors never abort a commit. If a message fails to render:

- The raw template source is sent to the provider instead of the rendered output.
- The error is logged at `WARN` level with the message hash and role.
- The model may surface the raw template text in its response, which is usually enough to diagnose the issue.

Use `POST /v1/pipeline/preview` to catch template errors before committing.

---

## Notes

- **System message is immutable per session.** It is stored as the first DAG entry at session creation (or on the first commit if none was provided). To use a different system prompt, create a new session.
- **Model-level system prompts** (`provider { model "x" { system = "..." } }`) are exposed as `.model.system`. The default system template renders it at the top via `{{ render .model.system }}` if non-empty. Custom session templates can place it anywhere, or omit it entirely.
- **Resource auto-injection is gone.** The old automatic `<relevant-resources>` block injected into the system prompt has been removed. Resource recall will be available as a template function in a future release (see `proposals/22`). For now, use the `resource__recall` tool from within a conversation.
- **Tools verbosity** (`tools_verbosity` on the session) has no effect on template rendering — the template controls what is shown. The field is still settable via `POST /v1/sessions/:id/system/reset` and may be used by future tooling.
- **Replay** re-renders messages against the current template engine. Dynamic values like `now` will differ from the original run. Use `GET /v1/pipeline/contexts/:hash/materialized` to see what was actually sent.
