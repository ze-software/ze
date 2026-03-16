# 251 — Commit Manager Injection

## Objective

Change `CommitManager` field in `ServerConfig`/`Server` from `*commit.CommitManager` to `any`, removing the BGP commit package import from generic plugin infrastructure.

## Decisions

- Pattern established: "generic infra stores `any`, domain code type-asserts" — consistent with BGPHooks pattern from spec 248.
- Test helpers `newTestContext()` and `newDispatchContext()` needed explicit CommitManager injection after the type change.

## Patterns

- `any` storage pattern: generic server stores domain objects as `any`; domain-specific code retrieves and type-asserts. Avoids import cycles while preserving runtime access.

## Gotchas

- Test helpers that construct `ServerConfig` need updating when field types change — easy to miss if they use struct literals.

## Files

- `internal/component/plugin/types.go` — `CommitManager any` field added to `ServerConfig`
- `internal/component/plugin/server.go` — field type changed to `any`, getter returns `any`, `commit` import removed
- `internal/component/plugin/command.go` — `CommitManager()` delegate returns `any`, `commit` import removed
- `internal/component/bgp/handler/commit.go` — `requireCommitManager()` helper with nil-check + type assert
- `internal/component/bgp/reactor/reactor.go` — injects `commit.NewCommitManager()` in `ServerConfig`
