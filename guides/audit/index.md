# Audit Trail

Ze keeps a local structured audit log for operator actions that change state or fail authentication. The log is append-only from the user's point of view: there is a query command, but no command to delete or truncate records.

## What Is Recorded

| Action | Surfaces |
|--------|----------|
| `config-commit` | SSH/CLI config editor, web config editor, REST config sessions, gRPC config sessions |
| `config-discard` | SSH/CLI config editor, web config editor, REST config sessions, gRPC config sessions |
| `daemon-reload` | SIGHUP and `daemon reload` command dispatch |
| `auth-fail` | SSH, web, REST, gRPC, MCP |

Each entry has `timestamp`, `actor`, `remote-addr`, `surface`, `action`, `detail`, and `outcome` fields. `detail` carries the config diff for commit and discard records when Ze has one.

<!-- source: internal/core/audit/audit.go -- Entry fields and action constants -->
<!-- source: internal/component/web/handler_config.go -- web config audit recording -->
<!-- source: internal/component/api/rest/server.go -- REST config and auth-fail audit recording -->
<!-- source: internal/component/api/grpc/server.go -- gRPC config and auth-fail audit recording -->
<!-- source: internal/component/mcp/audit.go -- MCP auth-fail audit recording -->

## Storage

For a daemon started from a real config file, the audit log is stored next to that file as `<config-base>.audit.jsonl`. For stdin configs (`ze -`) and web-only test mode, Ze uses an in-memory audit log.

The in-memory query cache keeps the newest records up to the configured retention limit in code. The default is 10,000 entries, with accepted bounds from 100 to 100,000.

<!-- source: cmd/ze/hub/audit.go -- defaultAuditPath and openAuditLog -->
<!-- source: internal/core/audit/audit.go -- retention bounds -->
<!-- source: internal/core/audit/store.go -- JSON-lines persistence -->

## Querying

Use `show audit` from the daemon CLI or `ze show audit` from the shell:

```
ze show audit
ze show audit action config-commit
ze show audit actor alice
ze show audit surface web count 20
ze show audit since 2026-05-24T10:00:00Z until 2026-05-24T11:00:00Z
```

The response is structured data with `entries` and `count`. Filters can be combined. Time filters use RFC 3339 timestamps.

<!-- source: internal/component/cmd/show/audit.go -- show audit filters -->

## Authorization Notes

Web config mutations now use the same profile authorizer as SSH, API, and MCP command dispatch. A read-only user can still open `/show/` pages, but `POST /config/set/`, add, delete, rename, commit, and discard are denied before the draft is changed.

REST and gRPC without a configured token or per-user authenticator use the built-in `api` identity in read-only mode. Read commands work. Command writes and config sessions return 403.

<!-- source: internal/component/web/handler_config.go -- authorizeWebConfigMutation -->
<!-- source: internal/component/api/engine.go -- read-only caller enforcement -->
