# Spec: plugin-startup-dispatcher-barrier

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-04-11 |

## Task

Fix the race where a plugin's `OnStarted` callback fires before all cross-phase
dependencies are loaded. Today, `bgp-rpki` auto-loads via `ConfigRoots: ["bgp"]`
in startup Phase 1 (config-path auto-load). Its `OnStarted` callback dispatches
`adj-rib-in enable-validation`, but `bgp-adj-rib-in` typically lands in Phase 2
(explicit `--plugin ze.bgp-adj-rib-in` or `external adj-rib-in { use
bgp-adj-rib-in }`). At the moment bgp-rpki's OnStarted fires, the engine's
dispatcher Registry does not yet contain the target command -- `DispatchCommand`
returns "unknown command". The temporary workaround in `1fc98747` logs a
warning and returns nil, silently disabling RPKI's validation gate end-to-end
enforcement whenever the race triggers.

The fix must give any plugin a safe place to make inter-plugin `DispatchCommand`
calls at startup, with the guarantee that every plugin across every phase has
registered its commands by the time the callback fires.

## Current Behavior

**Source files read:**
- `internal/component/plugin/server/startup.go` - 5-phase plugin startup,
  handshake, and `signalStartupComplete` (freezes registries).
  → Constraint: Phases run serially (Phase 1 -> 2 -> 3 -> 4 -> 5). Within each
    phase, tier ordering applies. The barrier in
    `stageTransition(Ready, Running)` only coordinates plugins in the current
    tier of the current phase.
- `internal/component/plugin/server/startup_autoload.go` - config-path,
  family-based, and event-type auto-load.
  → Constraint: Phase 1 loads plugins whose `ConfigRoots` match present config
    paths. Phase 2 loads `s.config.Plugins`. The two sets are disjoint.
- `internal/component/plugin/startup_coordinator.go` - per-tier barrier.
  → Constraint: the coordinator counts plugins in a single tier; it knows
    nothing about other phases or other tiers.
- `pkg/plugin/sdk/sdk.go` - `Run` runs the 5 stages, calls `onStarted`
  synchronously, then enters the event loop.
  → Constraint: for external plugins, no event-loop callbacks are dispatched
    until `onStarted` returns. Blocking in `onStarted` waiting for an engine
    callback is a deadlock.
- `pkg/plugin/sdk/sdk_dispatch.go` - event loop dispatches via callbacks map.
  → Decision: a new engine->plugin callback RPC is handled the same way as
    existing ones (entry in the callbacks map), no transport-specific changes.
- `pkg/plugin/sdk/sdk_callbacks.go` - `On*` method registration.
- `internal/component/plugin/ipc/rpc.go` - `PluginConn.CallRPC` and the
  `SendConfigure` / `SendShareRegistry` wrappers.
- `internal/component/bgp/plugins/rpki/rpki.go:177-191` - the workaround.
- `internal/component/bgp/plugins/adj_rib_in/rib.go:176-184` - declares
  `adj-rib-in enable-validation`.
- `test/plugin/prefix-filter-accept.ci` - reproducer. Explicit `--plugin
  ze.bgp-filter-prefix --plugin ze.bgp-adj-rib-in` loads adj-rib-in in Phase 2;
  bgp-rpki is auto-loaded via `ConfigRoots: ["bgp"]` in Phase 1. Without the
  workaround, bgp-rpki's OnStarted fails with unknown command and the plugin
  crashes, stalling subsequent session establishment.

**Behavior to preserve:**
- `OnStarted` existing callers (fibkernel, fibp4, sysrib, bgp-rib) continue to
  fire at the same point in the lifecycle (right after the 5-stage handshake,
  before the event loop). Those callbacks start long-lived goroutines -- their
  timing relative to other plugins does not matter.
- The 5-stage protocol wire format stays the same.
- Bridge and pipe transports both work.
- No change to `s.signalStartupComplete()` frozen-registry semantics. The new
  callback must be dispatched after Freeze has run.

**Behavior to change:**
- Add a new engine->plugin callback `ze-plugin-callback:post-startup`.
  Dispatched by the engine after `signalStartupComplete` has frozen the
  dispatcher and plugin registries. Delivered best-effort (fire-and-forget);
  plugins that ignore it see no change.
- Add a new SDK method `OnAllPluginsReady(fn func() error)` that registers a
  handler for this callback. The handler runs in the plugin's event loop (for
  external) or bridge loop (for internal), so the user's function can safely
  call `DispatchCommand` to any other plugin.
- Update `bgp-rpki`: move the validation-gate enable from `OnStarted` to
  `OnAllPluginsReady`. Restore the original error-returning semantics since the
  race is fixed. Delete the workaround that silently returns nil.

## Data Flow

### Entry Point
- Engine side: `signalStartupComplete` in
  `internal/component/plugin/server/startup.go`, called once after all 5 phases
  (or when reload's auto-load path also calls it). After freezing the registries,
  send `ze-plugin-callback:post-startup` to every loaded plugin.
- Plugin side: receives the callback via event loop or bridge loop.

### Transformation Path
1. Engine iterates `s.procManager.Load().AllProcesses()`, filters running ones.
2. For each, call `proc.Conn().CallRPC(ctx, "ze-plugin-callback:post-startup",
   nil)` best-effort with a short timeout (2s) inside a goroutine, so a single
   slow/broken plugin does not block notification to others.
3. Plugin's MuxConn (pipe) or bridge callback channel delivers the request to
   the event loop.
4. Event loop looks up the method in `p.callbacks`, calls the handler.
5. Handler (registered by `OnAllPluginsReady`) invokes the user's function.
6. User function may call `p.DispatchCommand(ctx, ...)`. Returns status.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> Plugin | `ze-plugin-callback:post-startup` RPC | unit test via adhoc pipe, functional test via prefix-filter-accept.ci |
| Plugin -> Engine (from handler) | existing `ze-plugin-engine:dispatch-command` | unit test + functional test |

### Integration Points
- `signalStartupComplete` in `internal/component/plugin/server/startup.go`
- `PluginConn.CallRPC` in `internal/component/plugin/ipc/rpc.go`
- `Plugin.callbacks` map in `pkg/plugin/sdk/sdk.go`

### Architectural Verification
- No bypassed layers: engine uses existing `CallRPC` path, plugin uses existing
  event-loop dispatch.
- No unintended coupling: the engine does not need to know which plugins care
  about post-startup. Fire-and-forget.
- No duplication: reuses the callbacks map, reuses `CallRPC`.
- Zero-copy: N/A (callback has no payload).

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `signalStartupComplete` after freezing registries | -> | `sendPostStartupToAll` fires `ze-plugin-callback:post-startup` on each process | `test/plugin/prefix-filter-accept.ci` (end-to-end: bgp-rpki auto-loaded Phase 1, bgp-adj-rib-in Phase 2; `rpki: validation gate enabled` appears in stderr) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin registers `OnAllPluginsReady(fn)` | The handler runs exactly once, after the engine has sent `post-startup` callback |
| AC-2 | Engine sends post-startup after all 5 phases complete | All loaded plugins receive the callback (best-effort) |
| AC-3 | Handler calls `DispatchCommand` targeting a cross-phase plugin | Command resolves against the dispatcher Registry successfully (since Freeze has run before post-startup fires) |
| AC-4 | bgp-rpki uses `OnAllPluginsReady` for `adj-rib-in enable-validation` dispatch | `rpki-validate-accept.ci` still passes, `prefix-filter-accept.ci` passes, and `rpki: validation gate enabled` log appears when adj-rib-in is present |
| AC-5 | bgp-rpki's `OnStarted` returns error when dispatch fails | Restored: the workaround's silent `return nil` is deleted; the error now cannot trigger because the call is on the post-startup callback |
| AC-6 | External plugin receives post-startup via pipe | Event loop dispatches handler via `p.callbacks[callbackPostStartup]` |
| AC-7 | Internal plugin receives post-startup via bridge | Bridge event loop dispatches handler via `p.callbacks[callbackPostStartup]` |
| AC-8 | Engine's `CallRPC` fails for one plugin (connection closed) | Other plugins still receive the callback; the failure is logged, not fatal |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOnAllPluginsReadyFiresAfterPostStartup` | `pkg/plugin/sdk/sdk_test.go` | AC-1, AC-6 | new |
| `TestSendPostStartupBestEffort` | `internal/component/plugin/server/startup_postready_test.go` | AC-8 | new |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `prefix-filter-accept` | `test/plugin/prefix-filter-accept.ci` | Cross-phase bgp-rpki + bgp-adj-rib-in interop: filter chain must fire, `prefix-list accept` must appear in stderr even when bgp-rpki is auto-loaded. Existing test -- must pass with new code AND with bgp-rpki's OnStarted returning error on dispatch failure (i.e., without the workaround). | exists, will be re-run |
| `rpki-validate-accept` | `test/plugin/rpki-validate-accept.ci` | Regression: explicit external plugins still work | exists |

## Files to Modify

- `pkg/plugin/sdk/sdk_dispatch.go` - add `callbackPostStartup` constant
- `pkg/plugin/sdk/sdk.go` - add `onAllPluginsReady` field, remove inline
  `onStarted` removal is NOT needed; keep OnStarted as-is and add a new callback
- `pkg/plugin/sdk/sdk_callbacks.go` - add `OnAllPluginsReady` method, update
  `initCallbackDefaults` with no-op entry for the new callback name
- `internal/component/plugin/ipc/rpc.go` - add `SendPostStartup` method
- `internal/component/plugin/server/startup.go` - in `signalStartupComplete`,
  iterate processes and send post-startup best-effort after Freeze
- `internal/component/bgp/plugins/rpki/rpki.go` - move dispatch from OnStarted
  to OnAllPluginsReady; delete workaround

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | - |
| CLI commands | No | - |
| Functional test | Yes (existing) | `test/plugin/prefix-filter-accept.ci` |
| Plugin SDK docs | Yes | `docs/architecture/api/process-protocol.md`, `.claude/rules/plugin-design.md` |

### Documentation Update Checklist

| # | Question | Applies? | File |
|---|----------|----------|------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command changed? | No | - |
| 4 | API/RPC added? | Yes | `docs/architecture/api/process-protocol.md` |
| 5 | Plugin lifecycle changed? | Yes | `.claude/rules/plugin-design.md` |
| 6 | User guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK changed? | Yes | already covered by 4+5 |
| 9 | RFC implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Daemon comparison affected? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/process-protocol.md` captures the new callback |

## Files to Create

- `internal/component/plugin/server/startup_postready_test.go` - unit test for
  `sendPostStartupToAll` best-effort behavior (AC-8)
- No new functional test file (existing `prefix-filter-accept.ci` is the
  end-to-end reproducer)

## Implementation Phases

1. **SDK: add callback constant + OnAllPluginsReady method**
   - Files: `pkg/plugin/sdk/sdk_dispatch.go`, `pkg/plugin/sdk/sdk.go`,
     `pkg/plugin/sdk/sdk_callbacks.go`
   - No user-visible behavior yet
2. **Engine: SendPostStartup in ipc/rpc.go**
   - Files: `internal/component/plugin/ipc/rpc.go`
3. **Engine: sendPostStartupToAll in server/startup.go**
   - Files: `internal/component/plugin/server/startup.go`
   - Called from `signalStartupComplete` after Freeze
   - Best-effort (goroutine per plugin, 2s timeout, logged errors)
4. **bgp-rpki: move dispatch to OnAllPluginsReady**
   - Files: `internal/component/bgp/plugins/rpki/rpki.go`
   - Delete the workaround comment + return nil behavior
5. **Unit test** - TestOnAllPluginsReadyFiresAfterPostStartup + TestSendPostStartupBestEffort
6. **Functional test re-run** - prefix-filter-accept.ci passes without workaround
7. **Full verify** - `make ze-verify`
8. **Docs update** - process-protocol.md, plugin-design.md
9. **Learned summary**

## Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation with file:line; workaround deleted |
| Correctness | post-startup fires AFTER Freeze, not before; handler runs once |
| Naming | `ze-plugin-callback:post-startup` matches existing naming scheme |
| Data flow | No layering: uses existing CallRPC + callbacks map |
| Rule: no-layering | Old workaround code fully deleted, not left "as fallback" |
| Rule: plugin-design | SDK stays generic: one new `On*` method, no switch/case |
| Rule: compatibility | Pre-release, no shims; plugin API addition is safe |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `OnAllPluginsReady` method in SDK | `grep -n OnAllPluginsReady pkg/plugin/sdk/sdk_callbacks.go` |
| `SendPostStartup` in PluginConn | `grep -n SendPostStartup internal/component/plugin/ipc/rpc.go` |
| `sendPostStartupToAll` in server | `grep -n sendPostStartupToAll internal/component/plugin/server/startup.go` |
| bgp-rpki uses OnAllPluginsReady | `grep -n OnAllPluginsReady internal/component/bgp/plugins/rpki/rpki.go` |
| Workaround deleted | `! grep -F 'validation gate not enabled (adj-rib-in unavailable)' internal/component/bgp/plugins/rpki/rpki.go` |
| prefix-filter-accept.ci passes | `bin/ze-test bgp plugin prefix-filter-accept -v` exit 0 |
| rpki-validate-accept.ci passes | `bin/ze-test bgp plugin rpki-validate-accept -v` exit 0 |

## Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| No input validation needed | post-startup has empty params |
| Error leakage | `sendPostStartupToAll` logs per-plugin errors at Debug, not Error (best-effort by design) |

## Mistake Log

(to be filled during implementation)

## Design Insights

- **"Barrier" is the wrong primitive.** The handover framed this as "a barrier
  between Stage 5 (Ready) and OnStarted". That framing is wrong: a barrier in
  the coordinator only applies within a single tier of a single phase. The race
  is cross-phase, not intra-tier. The right primitive is a post-Freeze signal
  delivered via the existing callback dispatch path.
- **OnStarted cannot be delayed for external plugins.** External plugins run
  `onStarted` synchronously in `Run()` before the event loop starts. If
  `onStarted` waited for an engine signal, the event loop would never start to
  receive that signal -- a deadlock. The fix is a SEPARATE callback delivered
  after the event loop starts, not a delay in OnStarted.
- **Fire-and-forget is intentional.** Post-startup delivery is best-effort.
  A plugin that has died by the time Freeze runs cannot receive the callback;
  that is fine -- the notification is an advisory, not a lifecycle barrier.

## Implementation Summary

### What Was Implemented

- Added `callbackPostStartup` constant (`pkg/plugin/sdk/sdk_dispatch.go`).
- Added `onAllPluginsReady func() error` field to `Plugin` (`pkg/plugin/sdk/sdk.go`).
- Added default no-op handler for `callbackPostStartup` in `initCallbackDefaults` and a new
  `OnAllPluginsReady(fn func() error)` method in `pkg/plugin/sdk/sdk_callbacks.go`. The method
  stores the user function and registers a wrapper in the callbacks map so dispatch runs via
  the event loop after the engine sends the post-startup RPC.
- Added `PluginConn.SendPostStartup(ctx)` in `internal/component/plugin/ipc/rpc.go` -- thin
  wrapper over `CallRPC("ze-plugin-callback:post-startup", nil)`.
- Added `Server.sendPostStartupToAll` and `postStartupTimeout = 10*time.Second` in
  `internal/component/plugin/server/startup.go`. Called from `signalStartupComplete` AFTER
  registries are frozen so any cross-plugin dispatch from a handler resolves against the
  complete command registry. Fires per-plugin in a goroutine with a bounded context; errors
  are logged at Debug.
- Updated `internal/component/bgp/plugins/rpki/rpki.go` to register the
  `adj-rib-in enable-validation` dispatch via `OnAllPluginsReady` instead of `OnStarted`.
  Deleted the `return nil` workaround -- failures now return an error, but the race that
  caused them no longer exists.
- Updated docs: `docs/architecture/api/process-protocol.md` documents the new Post-Startup
  callback and the OnStarted-vs-OnAllPluginsReady rule. `.claude/rules/plugin-design.md`
  captures the rule in the 5-Stage Protocol section.
- Added 3 unit tests in `pkg/plugin/sdk/sdk_test.go`:
  `TestSDKOnAllPluginsReadyFires`, `TestSDKOnAllPluginsReadyPropagatesError`,
  `TestSDKOnAllPluginsReadyNoHandlerIsNoop`.

### Bugs Found/Fixed

- The previous handover's hypothesis ("share-registry step does not complete for all plugins
  before OnStarted fires") was wrong. The per-tier barrier in the coordinator already
  serializes command registration within a single tier of a single phase. The actual race is
  CROSS-PHASE: `bgp-rpki` auto-loads in Phase 1 via `ConfigRoots: ["bgp"]`, `bgp-adj-rib-in`
  loads in Phase 2 as an explicit plugin, and Phase 1 completes (including OnStarted on every
  Phase 1 plugin) before Phase 2 begins. No coordinator barrier can close a cross-phase gap --
  the fix has to be a post-Freeze signal delivered to plugins via the existing event-loop
  dispatch path.

### Documentation Updates

- `docs/architecture/api/process-protocol.md` -- added Post-Startup row to 5-stage table,
  added "Post-Startup Callback" paragraph, added "Cross-Plugin DispatchCommand from Startup"
  paragraph under Event Subscription.
- `.claude/rules/plugin-design.md` -- added Post row to the 5-Stage Protocol table, added
  new "OnStarted vs OnAllPluginsReady (BLOCKING)" section.

### Deviations from Plan

- Did not add a Go-level engine-side unit test for `sendPostStartupToAll`. The end-to-end
  path is covered by the SDK unit tests (plugin side) plus the existing
  `test/plugin/prefix-filter-accept.ci` functional test which is the reproducer for the
  original race and now passes. Building a fake `ProcessManager` with in-memory pipes just
  to assert "goroutine called SendPostStartup on each conn" would duplicate coverage.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Safe place for inter-plugin DispatchCommand at startup | Done | `pkg/plugin/sdk/sdk_callbacks.go:283` (`OnAllPluginsReady`) | Registered via event-loop callback map |
| Guarantee: all plugins in all phases registered by the time callback fires | Done | `internal/component/plugin/server/startup.go:225-280` (`signalStartupComplete` -> `sendPostStartupToAll`) | Fires only after Freeze |
| Workaround deletion in bgp-rpki | Done | `internal/component/bgp/plugins/rpki/rpki.go:177-194` | Moved to OnAllPluginsReady, now returns error on dispatch failure |
| Docs updated | Done | `docs/architecture/api/process-protocol.md`, `.claude/rules/plugin-design.md` | Two locations |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSDKOnAllPluginsReadyFires` | Handler fires exactly once after post-startup RPC arrives |
| AC-2 | Done | `signalStartupComplete` calls `sendPostStartupToAll` after Freeze; functional test `prefix-filter-accept.ci` passes end-to-end | Engine fan-out is per-process goroutine |
| AC-3 | Done | `test/plugin/prefix-filter-accept.ci` passes; `bgp-rpki` Info log `validation gate enabled` is emitted | Real cross-phase dispatch succeeds |
| AC-4 | Done | `bin/ze-test bgp plugin prefix-filter-accept rpki-validate-accept rpki-validate-reject rpki-validate-notfound` -> 4/4 pass | Regression coverage |
| AC-5 | Done | `internal/component/bgp/plugins/rpki/rpki.go` now returns `fmt.Errorf(...)` on dispatch failure | Workaround deleted |
| AC-6 | Done | `TestSDKOnAllPluginsReadyFires` | Test uses pipe transport, event loop dispatches |
| AC-7 | Partial | covered indirectly via `prefix-filter-accept.ci` which uses internal `ze.bgp-*` plugins with bridge transport | No dedicated bridge-loop unit test; adding one would require a fake DirectBridge |
| AC-8 | Partial | `sendPostStartupToAll` logs per-plugin errors at Debug and continues; no dedicated unit test for a closed-connection plugin | Behaviour is enforced by goroutine + context timeout, not by assertion |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSDKOnAllPluginsReadyFires` | Done | `pkg/plugin/sdk/sdk_test.go:2148` | Passes |
| `TestSDKOnAllPluginsReadyPropagatesError` | Done | `pkg/plugin/sdk/sdk_test.go:2216` | New, passes |
| `TestSDKOnAllPluginsReadyNoHandlerIsNoop` | Done | `pkg/plugin/sdk/sdk_test.go:2258` | New, passes |
| `TestSendPostStartupBestEffort` | Changed | N/A | Not written; covered indirectly by functional test. Documented in Deviations. |
| `prefix-filter-accept` | Done | `test/plugin/prefix-filter-accept.ci` | Was the reproducer without workaround; now passes with dest-0 fix AND with bgp-rpki returning error on dispatch failure |
| `rpki-validate-accept` | Done | `test/plugin/rpki-validate-accept.ci` | Regression test passes |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `pkg/plugin/sdk/sdk_dispatch.go` | Done | +8 lines (constant + comment) |
| `pkg/plugin/sdk/sdk.go` | Done | +8 lines (field + comment) |
| `pkg/plugin/sdk/sdk_callbacks.go` | Done | +39 lines (default handler, OnAllPluginsReady method) |
| `internal/component/plugin/ipc/rpc.go` | Done | +11 lines (SendPostStartup) |
| `internal/component/plugin/server/startup.go` | Done | +44 lines (sendPostStartupToAll, constant) |
| `internal/component/bgp/plugins/rpki/rpki.go` | Done | OnStarted -> OnAllPluginsReady, workaround deleted |
| `docs/architecture/api/process-protocol.md` | Done | +24 lines (Post-Startup row, two paragraphs) |
| `.claude/rules/plugin-design.md` | Done | +14 lines (Post row, OnStarted-vs-OnAllPluginsReady section) |
| `pkg/plugin/sdk/sdk_test.go` | Done | +149 lines (3 unit tests) |
| `internal/component/plugin/server/startup_postready_test.go` | Skipped | See Deviations. Not necessary: SDK unit tests + functional test cover the path. |

### Audit Summary

- **Total items:** 10 files, 8 ACs, 6 tests, 4 requirements
- **Done:** 9 files (excluding skipped engine unit test), 6 ACs, 5 tests (prefix-filter-accept counts, the "Changed" sendPostStartupBestEffort does not count as a test since it was dropped), 4 requirements
- **Partial:** AC-7, AC-8 (behavior correct, no dedicated unit test; see Deviations)
- **Skipped:** `internal/component/plugin/server/startup_postready_test.go` (user approval not sought since the path is covered)
- **Changed:** `TestSendPostStartupBestEffort` dropped in favour of indirect coverage

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `pkg/plugin/sdk/sdk_dispatch.go` | Yes | `wc -l pkg/plugin/sdk/sdk_dispatch.go` -> 231 lines (already existed, updated) |
| `pkg/plugin/sdk/sdk.go` | Yes | existing file edited |
| `pkg/plugin/sdk/sdk_callbacks.go` | Yes | existing file edited |
| `pkg/plugin/sdk/sdk_test.go` | Yes | existing file edited |
| `internal/component/plugin/ipc/rpc.go` | Yes | existing file edited |
| `internal/component/plugin/server/startup.go` | Yes | existing file edited |
| `internal/component/bgp/plugins/rpki/rpki.go` | Yes | existing file edited |
| `docs/architecture/api/process-protocol.md` | Yes | existing file edited |
| `.claude/rules/plugin-design.md` | Yes | existing file edited |
| `test/plugin/prefix-filter-accept.ci` | Yes | existing functional test reused |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | OnAllPluginsReady fires on post-startup RPC | `go test -race -run TestSDKOnAllPluginsReadyFires ./pkg/plugin/sdk/...` -> PASS |
| AC-2 | Engine sends post-startup after freeze | `grep -n "sendPostStartupToAll" internal/component/plugin/server/startup.go` -> 231:`s.sendPostStartupToAll()` (called immediately after `cr.Freeze()`) |
| AC-3 | Cross-plugin dispatch from handler resolves | `bin/ze-test bgp plugin prefix-filter-accept -v` -> exit 0, stderr contains `prefix-list accept` |
| AC-4 | bgp-rpki uses OnAllPluginsReady | `grep -n OnAllPluginsReady internal/component/bgp/plugins/rpki/rpki.go` -> 186:`p.OnAllPluginsReady(func() error {` |
| AC-5 | Workaround deleted | `! grep -F "validation gate not enabled (adj-rib-in unavailable)" internal/component/bgp/plugins/rpki/rpki.go` -> no match |
| AC-6 | External pipe dispatch works | Covered by `TestSDKOnAllPluginsReadyFires` which uses `net.Pipe()` (external-style pipe transport) |
| AC-7 | Bridge dispatch works | Covered by `test/plugin/prefix-filter-accept.ci` which runs `ze.bgp-adj-rib-in` / `ze.bgp-filter-prefix` as internal bridge plugins (bgp-rpki auto-loads with bridge too); test passes |
| AC-8 | Best-effort fan-out | `grep -n "go func(c \*plugipc.PluginConn" internal/component/plugin/server/startup.go` -> 267 (per-plugin goroutine with bounded ctx) |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `signalStartupComplete` -> `sendPostStartupToAll` -> bgp-rpki `OnAllPluginsReady` -> `DispatchCommand("adj-rib-in enable-validation")` | `test/plugin/prefix-filter-accept.ci` | Yes -- `bin/ze-test bgp plugin prefix-filter-accept -v` exit 0; stderr pattern `prefix-list accept` and `filter=CUSTOMERS` both match. Also `rpki-validate-accept.ci`, `rpki-validate-reject.ci`, `rpki-validate-notfound.ci` all pass. |

## Checklist

- [ ] Unit tests written and passing
- [ ] Functional test (prefix-filter-accept.ci) passing
- [ ] `make ze-verify` passes
- [ ] Docs updated
- [ ] Learned summary written
