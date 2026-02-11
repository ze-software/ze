# Spec: config-reload-8-daemon-rpcs

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugin/bgp.go` — handlerBgpRPCs(), daemon handlers
4. `internal/plugin/system.go` — systemRPCs(), handleSystemShutdown()
5. `internal/ipc/schema/ze-system-api.yang` — system module RPCs

**Parent spec:** `spec-reload-lifecycle-tests.md` (umbrella)
**Depends on:** `spec-config-reload-7-coordinator-hardening.md` (coordinator wiring must exist)

## Task

Move daemon lifecycle RPCs (`daemon-reload`, `daemon-shutdown`, `daemon-status`) from the `ze-bgp` module to the `ze-system` module. These are system-wide operations that affect all plugins, not BGP-specific actions. Config reload notifies GR, hostname, RIB, and any plugin with `WantsConfigRoots` — it does not belong under `ze-bgp`.

Three changes:

1. **Move RPCs to ze-system module:** Move `daemon-reload`, `daemon-shutdown`, `daemon-status` from `handlerBgpRPCs()` in bgp.go to `systemRPCs()` in system.go. Wire method changes from `ze-bgp:daemon-*` to `ze-system:daemon-*`. CLI command changes from `bgp daemon *` to `daemon *` (or `system daemon *`).

2. **Remove duplicate shutdown:** `ze-system:shutdown` and `ze-bgp:daemon-shutdown` both call `ctx.Reactor.Stop()`. After the move, `ze-system:daemon-shutdown` replaces both. Remove `ze-system:shutdown` (or keep it as an alias — decide during implementation).

3. **Update YANG schemas:** Move the three `rpc daemon-*` definitions from `ze-bgp-api.yang` to `ze-system-api.yang`. The editor's `NewSocketReloadNotifier` in `editor/reload.go` must send `ze-system:daemon-reload` instead of `ze-bgp:daemon-reload`.

4. **Backward-compatible wire method:** The editor and any existing CLI tools send `ze-bgp:daemon-reload`. During the transition, register the handler under both wire methods so old clients still work. The old method can be removed in a future cleanup (no backwards compatibility needed per project rules, but avoids breaking in-flight work).

~~Note on backward compatibility: Per project rules (`.claude/rules/compatibility.md`), Ze has never been released and has no users. The old wire method names can be deleted outright. However, since the editor in the same codebase sends `ze-bgp:daemon-reload`, we must update the sender and receiver together in the same commit.~~

**Superseded:** Simpler approach — update sender and receiver in the same commit. No dual registration needed. Per `.claude/rules/compatibility.md`, Ze has no users.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — system components, module structure

### Source Files (MUST read)
- [ ] `internal/plugin/bgp.go` — handlerBgpRPCs() (line 13), handleDaemonReload (line 69), handleDaemonShutdown (line 46), handleDaemonStatus (line 57)
- [ ] `internal/plugin/system.go` — systemRPCs() (line 10), handleSystemShutdown (line 85)
- [ ] `internal/plugin/command.go` — BgpPluginRPCs(), SystemPluginRPCs(), AllBuiltinRPCs()
- [ ] `internal/plugin/handler.go` — RPCRegistration struct (line 16)
- [ ] `internal/plugin/bgp/schema/ze-bgp-api.yang` — daemon-* RPCs (line 79-94)
- [ ] `internal/ipc/schema/ze-system-api.yang` — system RPCs, existing shutdown (line 36-38)
- [ ] `internal/config/editor/reload.go` — NewSocketReloadNotifier sends "ze-bgp:daemon-reload" (line 41)
- [ ] `internal/plugin/rpc_registration_test.go` — expected wire methods list

**Key insights:**
- `ze-system:shutdown` (system.go:15) and `ze-bgp:daemon-shutdown` (bgp.go:15) both call `ctx.Reactor.Stop()` — exact duplicate
- `RPCRegistration.WireMethod` is the YANG-derived `module:rpc-name` used on the wire
- `RPCRegistration.CLICommand` is the human-readable text command for dispatch
- The editor hardcodes `ze-bgp:daemon-reload` as the wire method — must be updated
- Test `TestRPCRegistrationAllMethods` in rpc_registration_test.go lists all expected wire methods — must be updated

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/bgp.go` — three daemon handlers registered under `ze-bgp:daemon-*`, CLI commands `bgp daemon *`
- [ ] `internal/plugin/system.go` — `ze-system:shutdown` registered separately, same handler body as `ze-bgp:daemon-shutdown`
- [ ] `internal/config/editor/reload.go` — sends `ze-bgp:daemon-reload` over Unix socket

**Behavior to preserve:**
- Daemon reload triggers config reload (coordinator path when available, reactor path as fallback)
- Daemon shutdown stops the reactor
- Daemon status returns uptime, peer count, start time
- Editor commit triggers reload via socket RPC
- All existing functional tests pass

**Behavior to change:**
- Wire methods: `ze-bgp:daemon-reload` → `ze-system:daemon-reload`, `ze-bgp:daemon-shutdown` → `ze-system:daemon-shutdown`, `ze-bgp:daemon-status` → `ze-system:daemon-status`
- CLI commands: `bgp daemon reload` → `daemon reload`, `bgp daemon shutdown` → `daemon shutdown`, `bgp daemon status` → `daemon status`
- YANG schema: daemon RPCs move from ze-bgp-api.yang to ze-system-api.yang
- Editor reload notifier: sends `ze-system:daemon-reload`
- `ze-system:shutdown` removed (replaced by `ze-system:daemon-shutdown`)

## Data Flow (MANDATORY)

### Entry Point
- CLI or editor sends RPC over Unix socket with wire method string
- Server dispatches via rpcDispatcher to registered handler

### Transformation Path
1. Client sends `{"method": "ze-system:daemon-reload"}` (was `ze-bgp:daemon-reload`)
2. Server `rpcDispatcher.Dispatch()` matches wire method to handler
3. Handler executes (reload/shutdown/status)
4. Response sent back to client

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Editor → Server | Unix socket, NUL-framed JSON, wire method string | [ ] |
| Server → Handler | rpcDispatcher lookup by wire method | [ ] |

### Integration Points
- `editor/reload.go` line 41 — wire method string must change
- `rpc_registration_test.go` — expected wire method list must change
- `ze-bgp-api.yang` — remove daemon RPCs
- `ze-system-api.yang` — add daemon RPCs

### Architectural Verification
- [ ] No bypassed layers (same dispatch mechanism, just different module name)
- [ ] No unintended coupling (daemon ops move to system module where they belong)
- [ ] No duplicated functionality (removes ze-system:shutdown duplicate)
- [ ] Zero-copy preserved where applicable (N/A)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDaemonReloadWireMethod` | `internal/plugin/rpc_registration_test.go` | `ze-system:daemon-reload` registered, `ze-bgp:daemon-reload` gone | |
| `TestDaemonShutdownWireMethod` | `internal/plugin/rpc_registration_test.go` | `ze-system:daemon-shutdown` registered, `ze-bgp:daemon-shutdown` gone | |
| `TestDaemonStatusWireMethod` | `internal/plugin/rpc_registration_test.go` | `ze-system:daemon-status` registered, `ze-bgp:daemon-status` gone | |
| `TestNoDuplicateShutdown` | `internal/plugin/rpc_registration_test.go` | No `ze-system:shutdown` alongside `ze-system:daemon-shutdown` | |
| `TestDispatchDaemonReload` | `internal/plugin/handler_test.go` | `daemon reload` dispatches correctly under new CLI command | |
| `TestDispatchDaemonStatus` | `internal/plugin/handler_test.go` | `daemon status` dispatches correctly | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing reload tests | `test/reload/*.ci` | Reload behavior unchanged | |
| All existing plugin tests | `test/plugin/*.ci` | Plugin behavior unchanged | |

## Files to Modify
- `internal/plugin/bgp.go` — remove daemon-reload, daemon-shutdown, daemon-status from handlerBgpRPCs()
- `internal/plugin/system.go` — add daemon-reload, daemon-shutdown, daemon-status to systemRPCs(); remove ze-system:shutdown (replaced by daemon-shutdown)
- `internal/plugin/bgp/schema/ze-bgp-api.yang` — remove daemon-* RPC definitions
- `internal/ipc/schema/ze-system-api.yang` — add daemon-* RPC definitions; remove standalone shutdown
- `internal/config/editor/reload.go` — change wire method to `ze-system:daemon-reload`
- `internal/plugin/rpc_registration_test.go` — update expected wire method lists
- `internal/plugin/handler_test.go` — update dispatch tests for new CLI commands

## Files to Create
- None

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Update expected wire methods in tests** — Change `ze-bgp:daemon-*` to `ze-system:daemon-*` in rpc_registration_test.go. Remove `ze-system:shutdown` from expected list.
   → **Review:** Are all test expectations updated?

2. **Run tests** — Verify FAIL (wire methods don't match yet)
   → **Review:** Tests fail for the right reason?

3. **Move handlers to system.go** — Move `handleDaemonReload`, `handleDaemonShutdown`, `handleDaemonStatus` from bgp.go to system.go. Update `systemRPCs()` to include them with `ze-system:daemon-*` wire methods and `daemon *` CLI commands. Remove from `handlerBgpRPCs()`. Remove `ze-system:shutdown` entry (replaced by `daemon-shutdown`). Remove `handleSystemShutdown` (body identical to `handleDaemonShutdown`).
   → **Review:** No dangling references? No duplicate handlers?

4. **Update YANG schemas** — Move `rpc daemon-reload`, `rpc daemon-shutdown`, `rpc daemon-status` from ze-bgp-api.yang to ze-system-api.yang. Remove `rpc shutdown` from ze-system-api.yang.
   → **Review:** YANG modules still valid? No duplicate RPC names?

5. **Update editor reload notifier** — Change `ze-bgp:daemon-reload` to `ze-system:daemon-reload` in editor/reload.go line 41.
   → **Review:** The editor and server now agree on wire method?

6. **Update handler tests** — Fix `TestDispatchBgpDaemonReload` to use new CLI command path. Update any server tests that reference `ze-bgp:daemon-*`.
   → **Review:** All test references updated?

7. **Verify all** — `make lint && make test && make functional` (paste output)
   → **Review:** Zero lint issues? All tests pass? No YANG validation errors?

## Implementation Summary

### What Was Implemented
- Moved `handleDaemonShutdown`, `handleDaemonStatus`, `handleDaemonReload` from `bgp.go` to `system.go`
- Updated `systemRPCs()` to include daemon RPCs with `ze-system:daemon-*` wire methods and `daemon *` CLI commands
- Removed `ze-system:shutdown` and `handleSystemShutdown` (exact duplicate of daemon-shutdown)
- Removed daemon RPCs from `handlerBgpRPCs()` in `bgp.go`
- Moved `rpc daemon-*` definitions from `ze-bgp-api.yang` to `ze-system-api.yang`
- Removed `rpc shutdown` from `ze-system-api.yang`
- Updated `editor/reload.go` to send `ze-system:daemon-reload`
- Updated `handleSystemHelp` fallback text
- Updated 19 files total (7 source + 9 test/functional + 3 architecture docs)

### Bugs Found/Fixed
- None — clean refactoring

### Investigation → Test Rule
- N/A — all behavior was already tested

### Design Insights
- Daemon lifecycle RPCs are system-wide operations (reload notifies GR, hostname, RIB plugins) — they correctly belong in ze-system, not ze-bgp
- `ze-system:shutdown` was an exact duplicate of `ze-bgp:daemon-shutdown` — consolidating removed dead code

### Documentation Updates
- `docs/architecture/api/wire-format.md` — Updated example table: `ze-bgp:daemon-status` → `ze-system:daemon-status`, added `ze-system:version-software`
- `docs/architecture/api/ipc_protocol.md` — Removed stale "Daemon Control" section (old `bgp daemon *` commands), added `daemon shutdown/status/reload` to System Namespace
- `docs/architecture/api/commands.md` — Removed stale `system shutdown`, added new "Daemon Commands" section with `daemon shutdown/status/reload`
- `internal/ipc/method_test.go` — Updated stale test example: `ze-bgp:daemon-status` → `ze-system:daemon-status`
- `internal/ipc/dispatch_test.go` — Updated stale test example: `ze-bgp:daemon-shutdown` → `ze-system:daemon-shutdown`

### Deviations from Plan
- Spec proposed individual `TestDaemonReloadWireMethod` etc. tests — instead, the existing `TestRPCRegistrationExpectedMethods` and `TestRPCRegistrationPerModule` tests were updated to validate the new wire methods. This is more maintainable than adding redundant single-assertion tests.
- Additional test files beyond spec's "Files to Modify" needed updating: `server_test.go`, `benchmark_test.go`, `cmd/ze/cli/main_test.go`, `cmd/ze/schema/main_test.go`, `internal/ipc/yang_test.go`, `internal/yang/rpc_test.go`, `test/parse/cli-schema-methods.ci`

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| daemon-reload moved to ze-system | ✅ Done | `system.go:23` | Wire method `ze-system:daemon-reload`, CLI `daemon reload` |
| daemon-shutdown moved to ze-system | ✅ Done | `system.go:21` | Wire method `ze-system:daemon-shutdown`, CLI `daemon shutdown` |
| daemon-status moved to ze-system | ✅ Done | `system.go:22` | Wire method `ze-system:daemon-status`, CLI `daemon status` |
| ze-system:shutdown removed (duplicate) | ✅ Done | `system.go` | `handleSystemShutdown` deleted, replaced by `handleDaemonShutdown` |
| Editor sends ze-system:daemon-reload | ✅ Done | `editor/reload.go:41` | `ipc.Request{Method: "ze-system:daemon-reload"}` |
| YANG schemas updated | ✅ Done | `ze-bgp-api.yang`, `ze-system-api.yang` | daemon RPCs moved, shutdown removed |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestDaemonReloadWireMethod | 🔄 Changed | `rpc_registration_test.go:109` | Covered by `TestRPCRegistrationExpectedMethods` |
| TestDaemonShutdownWireMethod | 🔄 Changed | `rpc_registration_test.go:109` | Covered by `TestRPCRegistrationExpectedMethods` |
| TestDaemonStatusWireMethod | 🔄 Changed | `rpc_registration_test.go:109` | Covered by `TestRPCRegistrationExpectedMethods` |
| TestNoDuplicateShutdown | 🔄 Changed | `rpc_registration_test.go:109` | `ze-system:shutdown` removed from expected list |
| TestDispatchDaemonReload | ✅ Done | `handler_test.go:1833` | Dispatches `daemon reload` |
| TestDispatchDaemonStatus | ✅ Done | `handler_test.go:1802` | Dispatches `daemon status` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/bgp.go` | ✅ Modified | Removed 3 daemon handlers + registrations |
| `internal/plugin/system.go` | ✅ Modified | Added 3 daemon handlers, removed shutdown, updated fallback help |
| `internal/plugin/bgp/schema/ze-bgp-api.yang` | ✅ Modified | Removed daemon-* RPC definitions |
| `internal/ipc/schema/ze-system-api.yang` | ✅ Modified | Added daemon-* RPCs, removed shutdown |
| `internal/config/editor/reload.go` | ✅ Modified | Wire method → `ze-system:daemon-reload` |
| `internal/plugin/rpc_registration_test.go` | ✅ Modified | Updated counts and expected methods |
| `internal/plugin/handler_test.go` | ✅ Modified | Updated dispatch tests, command lists, old-commands checks |

### Additional Files Modified (beyond plan)
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/server_test.go` | ✅ Modified | Wire method references updated |
| `internal/plugin/benchmark_test.go` | ✅ Modified | CLI command references updated |
| `cmd/ze/cli/main_test.go` | ✅ Modified | CLI→wire method mapping updated |
| `cmd/ze/schema/main_test.go` | ✅ Modified | RPC count and expected method list updated |
| `internal/ipc/yang_test.go` | ✅ Modified | YANG RPC lists updated |
| `internal/yang/rpc_test.go` | ✅ Modified | YANG RPC extraction lists updated |
| `test/parse/cli-schema-methods.ci` | ✅ Modified | Functional test expectations updated |
| `docs/architecture/api/wire-format.md` | ✅ Modified | Stale `ze-bgp:daemon-status` example updated (critical review) |
| `docs/architecture/api/ipc_protocol.md` | ✅ Modified | Stale `bgp daemon *` commands removed, `daemon *` added (critical review) |
| `docs/architecture/api/commands.md` | ✅ Modified | Stale `system shutdown` removed, daemon commands added (critical review) |
| `internal/ipc/method_test.go` | ✅ Modified | Stale test example updated (critical review) |
| `internal/ipc/dispatch_test.go` | ✅ Modified | Stale test example updated (critical review) |

### Audit Summary
- **Total items:** 24
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (tests covered by existing comprehensive tests instead of individual tests)

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [x] No premature abstraction (moving existing handlers, no new framework)
- [x] No speculative features (corrects module ownership, nothing new)
- [x] Single responsibility (system module owns daemon lifecycle, bgp module owns BGP ops)
- [x] Explicit behavior (wire methods clearly indicate module ownership)
- [x] Minimal coupling (daemon ops have no BGP-specific logic)
- [x] Next-developer test (developer looking for "reload" finds it in system module)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified — all target tests failed for correct reasons)
- [x] Implementation complete
- [x] Tests PASS (verified — `go test -race ./...` all pass)
- [x] Boundary tests cover all numeric inputs (N/A)
- [x] Feature code integrated into codebase (`internal/*`)
- [x] Functional tests verify end-user behavior (existing tests cover regression)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (96/96, 100%)

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read (N/A)
- [x] RFC references added to code (N/A)
- [x] RFC constraint comments added (N/A)

### Completion (after tests pass - see Completion Checklist)
- [x] Architecture docs updated with learnings (3 docs: wire-format.md, ipc_protocol.md, commands.md)
- [x] Implementation Audit completed (all items have status + location)
- [x] All Partial/Skipped items have user approval (none)
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/228-config-reload-8-daemon-rpcs.md`
- [ ] All files committed together
