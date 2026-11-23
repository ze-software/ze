# Spec: plugin-command-named-arguments

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | Post-release plugin protocol and SDK enhancement: passes validated command arguments to external plugin handlers by YANG leaf name instead of positional `args []string`. Today Ze validates every argument against the plugin's `-cmd` YANG leaves before dispatch and passes the tokens in `args []string`. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Today `execute-command` (`ExecuteCommandInput` in `pkg/plugin/rpc/types.go` and `sdk.ExecuteCommandHandler` in `pkg/plugin/sdk/sdk_callbacks.go`) delivers command arguments to a plugin as an ordered `args []string` slice, even though the dispatcher already parsed and validated every argument against the command's YANG leaves (`command.ValidateModelArgs`). External plugin authors must index `args[0]`, `args[1]`, etc. by position rather than reading arguments by their declared YANG leaf name.

Provenance: left out by the owner's approval of phase 11 in `spec-plugin-external-yang-at-startup` (2026-11-17: wanted later, not declined; listed in its `Work Not Done` table). Separate from `plan/release/fixes/spec-yang-single-source-of-truth.md` part D, which moves compiled-in handlers onto `ValidatedArgs` as a release fix; this spec changes the plugin wire protocol and public SDK after the first release.

Goal: carry validated named arguments (keyed by YANG leaf name) across `execute-command` and the Go/Python plugin SDKs so external plugin handlers read their command arguments by name instead of positional `args []string`.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/commands-cli.md`
  → Constraint: `pkg/plugin/rpc` and `pkg/plugin/sdk` are public contracts; `ze-plugin-callback.yang`, the Go SDK, the Python library, and `docs/plugin-development/` must move together.
- [ ] `docs/architecture/api/process-protocol.md`
  → Constraint: `execute-command` RPC wire format and DirectBridge parity.
- [ ] `docs/plugin-development/commands.md`
  → Constraint: documents how plugin handlers receive arguments.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/types.go` - `ExecuteCommandInput` carries `Args []string`.
- [ ] `internal/component/plugin/server/command.go` - `routeToProcess` validates arguments with `command.ValidateModelArgs` and forwards token strings to the plugin.
- [ ] `examples/plugin/go/main.go`, `examples/plugin/python/acme_monitor.py` - handlers read `args[0]`.

**Behavior to preserve:**
- YANG argument validation (`ValidateModelArgs`) continues to refuse bad or missing arguments before calling the plugin.
- Both internal (`DirectBridge`) and external (TLS/JSON RPC) plugins observe identical argument delivery.

**Behavior to change:**
- Deliver named argument values (by YANG leaf name) in `ExecuteCommandInput` and expose them in `pkg/plugin/sdk` and `examples/plugin/python/ze_plugin.py`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Model-validated argument binding | `internal/component/command/argbind.go` `ValidatedArgs`, `local_data.go` `ValidateModelArgs` | Binds CLI tokens to YANG argument leaf names and validates types and mandatory leaves | Extends: pass the bound leaf-name map from `routeToProcess` into `ExecuteCommandInput` |
| `execute-command` RPC and SDK callback | `pkg/plugin/rpc/types.go` `ExecuteCommandInput`, `pkg/plugin/sdk/sdk_callbacks.go` `OnExecuteCommand`, `internal/core/ipc/yang/ze-plugin-callback.yang` | Carries command name, `args []string`, peer, and pipes to internal and external plugins | Extends / replaces positional `args` with named arguments on wire and SDK |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Any operator command routed through `plugin/server/command.go` `routeToProcess` to a plugin process.

### Transformation Path
(fill during design: `ValidateModelArgs` -> bound leaf map -> `ExecuteCommandInput` -> `DirectBridge` / JSON RPC -> `sdk` and Python handler.)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (fill during design) | (fill during design) |

## Files to Modify
- `pkg/plugin/rpc/types.go`, `internal/core/ipc/yang/ze-plugin-callback.yang` - named argument map on `ExecuteCommandInput`
- `internal/component/plugin/server/command.go` - forward bound leaf names from `ValidateModelArgs`
- `pkg/plugin/sdk/sdk_callbacks.go`, `examples/plugin/go/main.go`, `examples/plugin/python/ze_plugin.py`, `examples/plugin/python/acme_monitor.py`
- `docs/architecture/api/process-protocol.md`, `docs/plugin-development/commands.md`, `docs/plugin-development/protocol.md`
