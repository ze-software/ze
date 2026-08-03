# 987 -- feature-gate child 5: mcp compile-out (registry + neutral command metadata)

## Context

Child 5 of the feature-gate umbrella (`plan/spec-feature-gate-0-umbrella.md`): make
the MCP server compile-out-able via `ze_mcp` for a smaller binary and attack
surface. MCP fits the **construction registry** (lg/web shape, 980/984), not a
dedicated seam: its `MCPServerHandle` is already `Reconfigurable` + `Shutdown` and
migrator-registered. The one real obstacle was that the always-on API command
lister (`apiCommandLister`) reused `serverCommandLister`, which returned
`zemcp.CommandLister` -- so API transitively pinned `internal/component/mcp` into
every binary. The fix is a neutral command-metadata type both surfaces adapt.

## Decisions

- **Neutral always-on command metadata.** New `cmd/ze/hub/command_meta.go` defines
  `commandMeta`/`commandParam`/`commandUIResource` and `commandMetaSource()` (the
  dispatcher traversal + YANG metadata, formerly the body of `serverCommandLister`).
  API adapts it to `api.CommandMeta` directly; the gated `service_mcp.go` wraps it as
  a `zemcp.CommandLister` (`mcpCommandLister`). Chose a neutral hub type over reusing
  `api.CommandMeta` because MCP needs `TaskSupport`/`UIResource` that `api.CommandMeta`
  does not carry.
- **Registry, not a seam.** `service_mcp.go`/`register_mcp.go` (`//go:build ze_mcp`)
  register `buildMCPService` like lg/web; the moved `mcp.go` construction lives there.
  `SetMCP` widened from `*MCPServerHandle` to `Reconfigurable` (as `SetWeb`/`SetLG`).
- **MCP resolution stays always-on; only construction moves.** `main.go` resolves
  addrs/token/`zeconfig.MCPListenConfig` to plain values and passes them via a
  `ServiceDeps.MCP *mcpServiceDeps` sub-struct (pointer: a by-value sub-struct pushed
  `ServiceDeps` to 464 bytes and tripped `hugeParam`, learned 981).
- **MCP shutdown via the registry defer**, dropping the explicit pre-`apiServer.Stop`
  `mcpSrv.Shutdown` -- identical to how web/lg already shut down.

## Consequences

- Bare `go build -tags ze_core`: **0** `internal/component/mcp` symbols (was 342 with
  `ze_mcp`). `ze`/`ze-appliance` keep MCP via `ZE_FEATURES`; `ze-stripped`
  (`ze_core ze_ssh`) drops it. A no-mcp build rejects `environment { mcp {} }` as
  "unknown field", no panic.
- `command_meta.go` is the always-on home for command metadata; any future surface
  (API, MCP, a third) adapts `commandMeta` rather than re-traversing the dispatcher.
  Keep zemcp types out of it.

## Gotchas

- **A-5 was wrong: dep_audit WOULD flag the chaos importers.** `disableable_violations`
  is a build-tag-agnostic text scan over the whole repo; `internal/chaos/{orchestrator,mcp}`
  import `internal/component/mcp` untagged. ze-chaos builds `-tags ze_chaos` (no
  `ze_mcp`) and reaches the orchestrator only under `//go:build ze_chaos` in
  `cmd/ze/ze_chaos_run.go`, so the import never pins mcp into the production `ze`
  daemon. Fix: `dep_audit.py` now skips `DISABLEABLE_NONPROD_PREFIXES`
  (`internal/chaos/`, `internal/test/`) -- non-production trees cannot pin a feature
  into the daemon. Mirrors the engine gate's `NON_FEATURE_PREFIXES`. MCP is the first
  disableable feature chaos imports; lg/web/gnmi did not hit this.
- **`TaskSupportOptional == 0`** is the zero value, so `parseTaskSupportLevel("")`
  equals the pre-gate plugin-command default; the neutral->zemcp conversion is
  byte-faithful (covered by `TestMCPCommandLister`).
- **`ze-validate` flags 12 pre-existing hub exports** (the migrator/registry API) once
  any of their files is in the diff; they are package-internal by design and
  `ze-verify` does not run `validate.py`, so they are not commit blockers.
- The no-sprintf-alloc hook fires on `module+"-api"` when moving `buildParamMap` into a
  fresh file (it was pre-existing in `mcp.go`); converted to `textbuf` in `buildParamMeta`.

## Files

- Created: `cmd/ze/hub/command_meta.go`, `service_mcp.go` (`ze_mcp`), `register_mcp.go`
  (`ze_mcp`), `build_tag_mcp_present_test.go`, `build_tag_mcp_absent_test.go`,
  `service_mcp_test.go`, `internal/component/plugin/all/all_ze_mcp.go` (generated)
- Deleted: `cmd/ze/hub/mcp.go` (split into command_meta.go + service_mcp.go)
- Modified: `cmd/ze/hub/{main.go,main_servers.go,api.go,listener_migrate.go,service_registry.go}`,
  `cmd/ze/hub/{mcp_test.go,mcp_keyperm_test.go}` (gated `ze_mcp`), `feature-gates.txt`,
  `.golangci.yml`, `scripts/dev/dep_audit.py`, `internal/component/plugin/all/all.go`,
  `docs/features.md`, `ai/rules/architecture.md`, `ai/rules/plugins.md`
