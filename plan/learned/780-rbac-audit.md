# 780 -- RBAC Audit

## Context

Web config mutations, no-auth APIs, and several direct transport paths had weaker controls than the SSH/CLI dispatcher path. The work hardened those gaps by making web edits ask the same authorizer as dispatcher commands, limiting no-auth API callers to read-only access, and adding a local audit component for operator-visible records. The audit trail covers config commit/discard, daemon reload, and failed authentication, with persistence so a restart keeps the evidence available.

## Decisions

- Reused `aaa.Authorizer` for web config handlers with synthetic `config ...` commands, because profile semantics already exist in `authz.Store`.
- Chose a small `audit.Recorder` interface over file ownership in transport components, because the hub owns runtime wiring and components stay easy to test.
- Stored records as JSON lines beside the config file, because Ze already keeps local state self-contained and the log needs append-friendly persistence.
- Exposed TACACS+ accounting drops via `show aaa accounting`, because accounting remains best effort and operators need a counter without blocking commands.

## Consequences

- Web, REST, gRPC, SSH, MCP, dispatcher, and system reload paths can now feed one local audit log.
- REST and gRPC no-auth mode is read-only through a transport-level `CallerIdentity.ReadOnly` flag, with direct config-session writes guarded before mutation.
- `show audit` gives operators a live query path for action, actor, surface, time range, and count filters.
- Components that accept an `audit.Recorder` must keep nil-recorder behavior as a no-op so tests and disabled audit setups remain simple.

## Gotchas

- Capture config diffs before commit or discard, because successful mutation clears the working state.
- `daemon status` is read-only, while `daemon reload` and related lifecycle commands must be classified as writes.
- Web terminal mode bypasses normal HTTP config handlers, so it needs the same synthetic command mapping and audit logic.
- Mixed diffs in `docs/architecture/core-design.md` and `internal/component/plugin/server/server.go` need partial staging to avoid unrelated policy work.

## Files

- `internal/component/audit/`
- `internal/component/web/`
- `internal/component/api/`
- `internal/component/plugin/server/`
- `internal/component/cmd/show/`
- `internal/component/tacacs/accounting.go`
- `cmd/ze/hub/`
- `docs/guide/audit.md`
- `test/plugin/audit-*.ci`, `test/plugin/rbac-web-config-deny.ci`, `test/plugin/rest-no-auth-readonly.ci`, `test/plugin/tacacs-acct.ci`
