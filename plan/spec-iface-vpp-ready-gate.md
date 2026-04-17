# Spec: iface-vpp-ready-gate

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/7 |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/615-vpp-4-iface.md` - why lazy-channel was chosen and what was deferred
4. `internal/component/iface/config.go:1173-1188` - the reconciliation block
5. `internal/component/vpp/vpp.go:222-227` - where `EventConnected` is emitted
6. `internal/plugins/fibvpp/register.go:130-139` - reference pattern for vpp event subscription

## Task

The `iface` component's `applyConfig` calls `Backend.ListInterfaces()` to reconcile
interface addresses (Phase 3) and prune stale Ze-managed interfaces (Phase 4). With
the `vpp` backend and the lazy-channel pattern introduced in `spec-vpp-4-iface`,
the first `ListInterfaces()` fires BEFORE the `vpp` component has completed its
GoVPP handshake. Result: `ifacevpp: VPP connector not available`, which `applyConfig`
records as an error; `OnConfigure` joins the errors and returns; the plugin server
logs `ERROR deliverConfigRPC failed ... list interfaces for reconciliation: ...`.

The backend load itself succeeds (`test/vpp/006-iface-create.ci` passes), but the
startup stderr includes a scary ERROR line and reconciliation never runs once VPP
comes up, so stale addresses on running VPP interfaces would never be cleaned up.

Goal: the iface component must defer reconciliation until the `vpp` backend is
ready, re-run reconciliation when `vppevents.EventConnected` (and `EventReconnected`,
for crash recovery) fires, and stop emitting the ERROR line at startup. The
netlink backend path must be unchanged.

## Required Reading

### Architecture Docs

- [ ] `plan/learned/615-vpp-4-iface.md` - why lazy-channel + what "additive-only fallback" means
  → Decision: use `errors.Is` with a typed sentinel, not string matching on error text
  → Constraint: backend load succeeds independently of VPP handshake state; the gate is post-load only

- [ ] `.claude/rules/plugin-design.md` - `OnStarted` vs `OnAllPluginsReady`, EventBus typed payloads
  → Constraint: cross-plugin dispatch at startup belongs in `OnAllPluginsReady`; EventBus subscriptions belong in `OnStarted` or the first `OnConfigure`
  → Constraint: EventBus subscribers must be stored for unsubscribe on plugin shutdown (see `unsubscribers` in `internal/component/iface/register.go`)

- [ ] `.claude/rules/design-principles.md` - "Explicit > implicit", "Fail-mode awareness"
  → Decision: use a sentinel so the control-flow branch ("not ready") is explicit and testable, not a magic string

### RFC Summaries (MUST for protocol work)

Not a protocol change. No RFC applies.

**Key insights:**
- The vpp component emits `vppevents.EventConnected` exactly once after first successful GoVPP handshake (`internal/component/vpp/vpp.go:225`) and `EventReconnected` on every subsequent reconnect (`:223`). `fibvpp` already subscribes to `EventReconnected` for RIB replay (`internal/plugins/fibvpp/register.go:133`).
- `applyConfig` already has an "additive-only" fallback path when `ListInterfaces` fails (`internal/component/iface/config.go:1177-1188`): it adds desired addresses but does not remove stale ones. That fallback is exactly what we want to keep on the first failed call; the only change is to NOT treat the "not ready" case as an error.
- Reconciliation is idempotent. Calling `applyConfig(activeCfg, activeCfg, b)` after VPP is live will diff desired-vs-observed and prune correctly. Re-running on EventReconnected is equally safe.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/component/iface/config.go` — `applyConfig(cfg, previous, b)` (line 982). At line 1176 calls `b.ListInterfaces()`; on error at line 1177 calls `record("list interfaces for reconciliation", err)` which appends to `errs`, then falls back to additive-only (lines 1179-1187) and returns the populated `errs`. The non-empty `errs` propagates to `OnConfigure`/`OnConfigApply` as an error via `joinApplyErrors`.
- [ ] `internal/component/iface/register.go:175-212` — `OnConfigure` path. Calls `LoadBackend(cfg.Backend)` then `applyConfig(cfg, nil, b)`. If `applyConfig` returns a non-empty error slice, `OnConfigure` returns the joined error. `activeCfg = cfg` is set AFTER the error check, so a failed reconcile on first apply leaves `activeCfg == nil`.
  → Constraint: if the "not ready" branch must still set `activeCfg` so EventConnected can re-run reconciliation, we need `activeCfg = cfg` set regardless of the not-ready branch.
- [ ] `internal/component/iface/register.go:283-347` — `OnConfigApply` transactional path. Calls `applyConfig(cfg, previousCfg, b)` wrapped in `sdk.NewJournal.Record`; on error triggers rollback.
  → Constraint: reconfiguration apply must NOT rollback on "backend not ready" alone. If rollback fires, every successfully-applied non-reconcile phase gets reverted for the wrong reason.
- [ ] `internal/plugins/ifacevpp/ifacevpp.go:78-116` — `ensureChannel`. Line 91 sets `b.chErr = fmt.Errorf("ifacevpp: VPP connector not available")` when `vppcomp.GetActiveConnector() == nil`. No sentinel.
  → Constraint: the sentinel must be defined in the `iface` package (not `ifacevpp`) because `iface/config.go` is the consumer. `ifacevpp` already imports `iface` so wrapping is trivial.
- [ ] `internal/plugins/ifacevpp/query.go:22-28` — `ListInterfaces()`. First line: `if err := b.ensureChannel(); err != nil { return nil, err }`. So the sentinel-wrapped error propagates up unchanged.
- [ ] `internal/component/vpp/vpp.go:117-175` — `VPPManager.Run`. `setActiveConnector(m.connector)` at line 147 happens BEFORE `runOnce` (which does the actual GoVPP handshake). So `GetActiveConnector()` returning non-nil does NOT mean "connected". Connection establishment is gated by `m.connector.Connect(connectCtx, 10, time.Second)` at line 213 inside `runOnce`. The `EventConnected` / `EventReconnected` emission at lines 223-226 happens AFTER the handshake succeeds.
  → Constraint: `ifacevpp` must use `connector.IsConnected()` (not `GetActiveConnector() != nil`) to decide whether to return "not ready". Currently `ensureChannel` only checks `!= nil`, which is why `NewChannel()` returns `"govpp: not connected"` and the sentinel path never fires. This is a bug in the ifacevpp.go:89-93 guard.
- [ ] `internal/component/vpp/conn.go` — `Connector.NewChannel` returns `"govpp: not connected"` when `!c.connected`. `IsConnected()` exists.
  → Decision: rather than wrap `NewChannel`'s error text, have `ensureChannel` check `connector.IsConnected()` up front and synthesize the sentinel-wrapped error without calling `NewChannel` at all.
- [ ] `internal/plugins/fibvpp/register.go:109-143` — `OnStarted` pattern. Uses `GetActiveConnector()` and `connector.NewChannel()`. Subscribes to `EventReconnected` for replay. Does NOT subscribe to `EventConnected` because fibvpp uses a mock backend on failure, so initial-connect is a non-event for it.
  → Decision: iface subscribes to BOTH `EventConnected` and `EventReconnected`. Initial-connect matters here because the first reconciliation was deferred.

**Behavior to preserve:**
- `ListInterfaces` still returns `([]InterfaceInfo, error)` — no signature change.
- `applyConfig` still returns `[]error` — no signature change.
- Netlink backend: `ListInterfaces` works synchronously, reconciliation runs on every apply, no deferral. No behavior change.
- Additive-only path: when reconciliation is skipped, configured addresses still get applied immediately via `b.AddAddress(osName, addr)` (lines 1180-1187).
- `EventConnected` is emitted exactly once per VPP Manager lifecycle; `EventReconnected` is emitted on every subsequent successful handshake after a crash. fibvpp's existing subscription pattern remains unchanged.
- `006-iface-create.ci` still passes; the test is extended with a `reject=stderr:pattern=deliverConfigRPC failed` assertion, not rewritten.

**Behavior to change:**
- `applyConfig` no longer returns an error when the only failure is "backend not ready" during reconciliation. Instead it logs at debug and skips Phase 3 (address reconcile) + Phase 4 (delete non-config interfaces).
- `iface` component subscribes to `vppevents.EventConnected` + `EventReconnected` and runs a reconciliation pass using `activeCfg` when they fire.
- `ifacevpp.ensureChannel` checks `connector.IsConnected()` so the sentinel fires reliably instead of propagating the generic `"govpp: not connected"` from `NewChannel`.

## Data Flow (MANDATORY)

### Entry Point

- **Startup path:** ze process starts → plugins registered → engine starts plugins in topological order → `vpp` plugin's `OnStarted` launches `VPPManager` goroutine → `iface` plugin's `OnConfigure` fires with config → `LoadBackend("vpp")` → `applyConfig(cfg, nil, b)`.
- **Reconnect path:** VPP crashes → `VPPManager.runOnce` returns error → backoff → re-exec VPP → `m.emitEvent(vppevents.EventReconnected)` → iface handler re-runs reconciliation.
- **First-connect path (the fix):** iface `applyConfig` first call fails at `ListInterfaces` with `iface.ErrBackendNotReady` → applyConfig skips reconcile, returns no error → `activeCfg` set → later VPP connects → `m.emitEvent(vppevents.EventConnected)` → iface handler calls `reconcileOnReady(activeCfg, b)`.

### Transformation Path

1. **Sentinel propagation:** `vpp.Connector.IsConnected()` false → `ifacevpp.ensureChannel` sets `b.chErr = fmt.Errorf("ifacevpp: VPP connector not available: %w", iface.ErrBackendNotReady)` → `ListInterfaces` returns it unchanged → `applyConfig` checks `errors.Is(err, iface.ErrBackendNotReady)` at line 1177.
2. **Skip reconcile branch:** instead of `record("list interfaces for reconciliation", err)`, log `log.Debug("iface reconcile deferred, backend not ready")`, run the additive address loop (unchanged), return without appending to `errs`.
3. **Subscribe once:** the iface plugin stores an `unsubscribe` function for the vpp events, scoped like the existing `unsubscribers` list.
4. **Event fire:** on `EventConnected` or `EventReconnected`, the handler reads `activeCfg`, and if non-nil and backend is vpp, calls a new `reconcileOnReady(cfg, GetBackend())` helper. That helper runs ONLY Phase 3 + Phase 4 of the current `applyConfig` body (addresses + stale-interface removal), not the create/modify/admin-up phases.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| iface component ↔ ifacevpp plugin | `Backend` interface — sentinel error flows back via `ListInterfaces` | [ ] |
| vpp component ↔ iface component | EventBus: subscribe to `(vppevents.Namespace, EventConnected/EventReconnected)` | [ ] |
| Goroutine boundary | EventBus callback runs in `Emit`'s dispatch goroutine; `reconcileOnReady` mutates backend state, must not race with an in-flight `OnConfigApply` | [ ] |

### Integration Points

- `internal/component/iface/config.go:applyConfig` — gains a sentinel-aware branch at the Phase 3 boundary.
- `internal/component/iface/backend.go` — gains `ErrBackendNotReady` sentinel and `reconcileOnReady` helper.
- `internal/component/iface/register.go` — adds `vppevents` subscription, appends to `unsubscribers`.
- `internal/plugins/ifacevpp/ifacevpp.go` — `ensureChannel` uses `connector.IsConnected()` and wraps the sentinel.

### Architectural Verification

- [ ] No bypassed layers: the sentinel flows through the existing `Backend.ListInterfaces` error channel; no side channel.
- [ ] No unintended coupling: iface depends on `vppevents` (already re-exported as a leaf package with no back-import risk).
- [ ] No duplicated functionality: `reconcileOnReady` factors out the exact two phases from `applyConfig` rather than duplicating them.
- [ ] Zero-copy preserved: N/A (no wire encoding path involved).

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `interface.backend=vpp` with vpp component still handshaking | → | `applyConfig` detects `ErrBackendNotReady`, defers reconcile | `test/vpp/006-iface-create.ci` extended with `reject=stderr:pattern=deliverConfigRPC failed` + `reject=stderr:pattern=list interfaces for reconciliation` |
| `vppevents.EventConnected` emission after VPP handshake | → | iface event handler calls `reconcileOnReady(activeCfg, b)` | `TestReconcileOnReady_InvokedOnEventConnected` in `internal/component/iface/config_test.go` (unit: fake EventBus, fake Backend) |
| `vppevents.EventReconnected` emission after VPP crash recovery | → | iface event handler calls `reconcileOnReady(activeCfg, b)` | `TestReconcileOnReady_InvokedOnEventReconnected` in `internal/component/iface/config_test.go` |
| Sentinel error from `ifacevpp.ListInterfaces` when connector not connected | → | `ensureChannel` wraps `iface.ErrBackendNotReady` | `TestEnsureChannel_NotConnectedReturnsSentinel` in `internal/plugins/ifacevpp/ifacevpp_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `interface.backend=vpp` delivered while `vpp.Connector.IsConnected()` is false | `ifacevpp.ensureChannel` returns an error satisfying `errors.Is(err, iface.ErrBackendNotReady)` |
| AC-2 | `applyConfig` call where `b.ListInterfaces()` returns a sentinel-wrapped `ErrBackendNotReady` | applyConfig returns an `errs` slice that does NOT contain the reconciliation error; additive address loop still runs; log at debug contains `"iface reconcile deferred, backend not ready"` |
| AC-3 | iface OnConfigure path triggers applyConfig with sentinel error | OnConfigure returns nil (no `deliverConfigRPC failed`); `activeCfg = cfg` is set |
| AC-4 | `vppevents.EventConnected` emitted on the EventBus after iface has deferred reconciliation | iface handler invokes `reconcileOnReady(activeCfg, backend)`; backend observes Phase 3 + Phase 4 calls (address reconcile + stale-interface removal) |
| AC-5 | `vppevents.EventReconnected` emitted after VPP crash recovery | same behavior as AC-4 (re-runs Phase 3 + Phase 4 using current `activeCfg`) |
| AC-6 | Netlink backend: `applyConfig` call with successful `ListInterfaces` | Reconciliation runs synchronously (Phase 3 + Phase 4), as before. No deferral, no event subscription side effect. |
| AC-7 | `iface` plugin shutdown | Vpp-event subscription is unsubscribed via the `unsubscribers` cleanup path |
| AC-8 | `ListInterfaces` returns a NON-sentinel error (e.g., real VPP error after connection established) | applyConfig still records the error in `errs` and returns it; error path unchanged |
| AC-9 | `006-iface-create.ci` functional test | PASSES; stderr contains no `deliverConfigRPC failed`; stderr contains no `list interfaces for reconciliation` at error level |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnsureChannel_NotConnectedReturnsSentinel` | `internal/plugins/ifacevpp/ifacevpp_test.go` | AC-1 — sentinel returned when connector.IsConnected() is false | [ ] |
| `TestEnsureChannel_NoConnectorReturnsSentinel` | `internal/plugins/ifacevpp/ifacevpp_test.go` | AC-1 — sentinel returned when connector is nil | [ ] |
| `TestApplyConfig_SkipsReconcileOnSentinel` | `internal/component/iface/config_test.go` | AC-2 — mock backend returning sentinel causes applyConfig to skip reconcile and return empty errs (or errs without the reconcile error); desired addresses still added | [ ] |
| `TestApplyConfig_RecordsNonSentinelListError` | `internal/component/iface/config_test.go` | AC-8 — mock backend returning a non-sentinel error is still recorded | [ ] |
| `TestReconcileOnReady_AddsMissingRemovesStale` | `internal/component/iface/config_test.go` | AC-4/5 — helper invoked with cfg + backend runs Phase 3 and Phase 4 only | [ ] |
| `TestReconcileOnReady_InvokedOnEventConnected` | `internal/component/iface/config_test.go` | AC-4 — with fake EventBus, emitting `(vpp, connected)` triggers reconcileOnReady | [ ] |
| `TestReconcileOnReady_InvokedOnEventReconnected` | `internal/component/iface/config_test.go` | AC-5 — same with `(vpp, reconnected)` | [ ] |
| `TestReconcileOnReady_NoOpWhenActiveCfgNil` | `internal/component/iface/config_test.go` | defensive — event fires before first config apply, no crash | [ ] |
| `TestUnsubscribeOnShutdown` | `internal/component/iface/register_test.go` (if one exists) or as part of config_test.go | AC-7 — subscription removed when plugin exits | [ ] |

### Boundary Tests (MANDATORY for numeric inputs)

N/A — no numeric ranges in this spec.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `006-iface-create` (extended) | `test/vpp/006-iface-create.ci` | With vpp backend, ze starts, iface OnConfigure returns no error, stderr has no `deliverConfigRPC failed` and no `list interfaces for reconciliation` at error level | [ ] |

### Future (if deferring any tests)

- End-to-end reconcile-on-connect test (pre-existing address on VPP interface that is NOT in config gets removed after EventConnected) — **DEFERRED to spec-vpp-stub-iface-api** because it requires `vpp_stub.py` to implement `sw_interface_dump`, `sw_interface_add_del_address`, `sw_interface_details`. This spec validates the ready-gate control flow; the full address-reconcile-after-connect coverage is extended in spec-vpp-stub-iface-api.

## Files to Modify

- `internal/component/iface/backend.go` — add `ErrBackendNotReady` sentinel.
- `internal/component/iface/config.go` — sentinel-aware branch at line 1177; extract `reconcileOnReady(cfg, b)` helper covering Phase 3 + Phase 4 (or a functionally equivalent refactor that keeps `applyConfig` body tidy).
- `internal/component/iface/register.go` — in `OnConfigure`, add subscriptions to `vppevents.EventConnected` and `vppevents.EventReconnected` into the existing `unsubscribers` list. Handler reads `activeCfg`, calls `reconcileOnReady` if non-nil. Must run regardless of which backend is active (reconcileOnReady is idempotent on netlink and the handler is already gated by `activeCfg != nil`).
- `internal/plugins/ifacevpp/ifacevpp.go` — change `ensureChannel` lines 88-93 to check `connector.IsConnected()` and wrap `iface.ErrBackendNotReady`. Preserve existing `!= nil` check as a secondary guard.
- `test/vpp/006-iface-create.ci` — add `reject=stderr:pattern=deliverConfigRPC failed` and `reject=stderr:pattern=list interfaces for reconciliation`.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes (reject on existing test) | `test/vpp/006-iface-create.ci` |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes (minor — event subscription added) | `docs/architecture/core-design.md` if it documents iface ↔ vpp coupling; likely no doc edit required — verify during Completion Checklist |

## Files to Create

None. All edits are modifications to existing files.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. /ze-review gate | Review Gate |
| 5. Full verification | `make ze-verify-fast` |
| 6. Critical review | Critical Review Checklist |
| 7. Fix issues | inline |
| 8. Re-verify | repeat 5 |
| 9. Repeat 6-8 | max 2 |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | repeat 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase 1 — Sentinel + ifacevpp gate.**
   - Tests: `TestEnsureChannel_NotConnectedReturnsSentinel`, `TestEnsureChannel_NoConnectorReturnsSentinel`
   - Files: `internal/component/iface/backend.go`, `internal/plugins/ifacevpp/ifacevpp.go`, `internal/plugins/ifacevpp/ifacevpp_test.go`
   - Verify: tests fail → implement sentinel + IsConnected() check → tests pass.

2. **Phase 2 — applyConfig deferral.**
   - Tests: `TestApplyConfig_SkipsReconcileOnSentinel`, `TestApplyConfig_RecordsNonSentinelListError`
   - Files: `internal/component/iface/config.go`, `internal/component/iface/config_test.go`
   - Verify: tests fail → implement sentinel branch + extract `reconcileOnReady` helper → tests pass.

3. **Phase 3 — Event subscription + reconcile-on-ready.**
   - Tests: `TestReconcileOnReady_InvokedOnEventConnected`, `TestReconcileOnReady_InvokedOnEventReconnected`, `TestReconcileOnReady_NoOpWhenActiveCfgNil`, `TestReconcileOnReady_AddsMissingRemovesStale`
   - Files: `internal/component/iface/register.go`, `internal/component/iface/config.go`, `internal/component/iface/config_test.go`
   - Verify: tests fail → implement subscription (appended to `unsubscribers`) + handler → tests pass.

4. **Phase 4 — Subscription cleanup test.**
   - Tests: `TestUnsubscribeOnShutdown`
   - Files: as Phase 3.
   - Verify: tests fail → confirm subscription is added to `unsubscribers` slice so existing shutdown cleanup covers it → tests pass.

5. **Phase 5 — Functional test.**
   - Tests: extended `test/vpp/006-iface-create.ci`.
   - Files: `test/vpp/006-iface-create.ci`.
   - Verify: run `bin/ze-test vpp 006-iface-create`; assert rejects for the error patterns.

6. **Phase 6 — Full verification.**
   - Command: `make ze-verify-fast` (timeout 180s).

7. **Phase 7 — Complete spec.**
   - Fill audit tables, Pre-Commit Verification, Review Gate.
   - Write `plan/learned/NNN-iface-vpp-ready-gate.md`.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-9 has a test and a file:line in Audit |
| Correctness | Sentinel wraps `iface.ErrBackendNotReady` consistently; `errors.Is` branch is correct; `reconcileOnReady` runs Phase 3 + Phase 4 only (not Phase 1 or Phase 2) |
| Naming | `ErrBackendNotReady` follows Go sentinel convention; `reconcileOnReady` clear |
| Data flow | Sentinel flows through `Backend.ListInterfaces` only — no new side channels. EventBus subscription follows existing `unsubscribers` pattern in `register.go`. |
| Rule: no-layering | No old path kept alongside new — the "not ready" branch replaces the error-recording branch for this specific error class |
| Rule: plugin-design.md | Subscription in OnConfigure is OK (that's where `unsubscribers` is populated); no cross-plugin DispatchCommand at startup (which would require OnAllPluginsReady) |
| Rule: design-principles.md "Lazy over eager" | reconcileOnReady runs ONLY reconciliation phases, does not redo create/admin-up |
| Rule: goroutine-lifecycle.md | EventBus handler is an existing long-lived mechanism; no new per-event goroutines introduced |
| Rule: integration-completeness.md | Wiring test is a `.ci` (`006-iface-create.ci`) that exercises the full path; not deferrable |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `iface.ErrBackendNotReady` sentinel exists | `grep -n "ErrBackendNotReady" internal/component/iface/backend.go` |
| `ensureChannel` wraps the sentinel | `grep -n "ErrBackendNotReady" internal/plugins/ifacevpp/ifacevpp.go` |
| `applyConfig` branches on sentinel | `grep -n "errors.Is.*ErrBackendNotReady" internal/component/iface/config.go` |
| `reconcileOnReady` exists | `grep -n "func reconcileOnReady" internal/component/iface/config.go` |
| Vpp-event subscription in register.go | `grep -n "vppevents.EventConnected\|vppevents.EventReconnected" internal/component/iface/register.go` |
| Subscription in unsubscribers list | `grep -n "unsubscribers = append" internal/component/iface/register.go` (context shows the vpp subs) |
| Functional test updated | `grep -n "reject=stderr:pattern=deliverConfigRPC" test/vpp/006-iface-create.ci` |
| `006-iface-create.ci` still passes | `bin/ze-test vpp 006-iface-create` exits 0 |
| Unit tests pass | `go test -race ./internal/component/iface/... ./internal/plugins/ifacevpp/...` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | N/A — all inputs are internal (sentinel error, EventBus events from trusted in-process source) |
| Resource exhaustion | Only two subscription handlers added; unsubscribe covered by existing `unsubscribers` cleanup |
| Error leakage | Debug-level log only reveals "backend not ready"; no secrets, no connector state |
| Race condition on activeCfg | `activeCfg` is a package-level variable currently read/written from `OnConfigure`, `OnConfigApply`, `OnConfigRollback`. Adding a new reader (the EventBus handler) is the same shape as existing writers. Needs to either use the existing `dhcpMu`/equivalent OR have its own mutex. Audit during implementation — may require a small refactor of `activeCfg` access. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| String-match `"VPP connector not available"` in applyConfig | Brittle — small error message change breaks the branch | Typed sentinel `iface.ErrBackendNotReady` + `errors.Is` |
| Retry loop in applyConfig | Blocks config-apply for up to retry budget; masks truly broken VPP | Event-driven — subscribe to EventConnected, re-run when ready |
| Silent nil-return on not-ready | Masks real errors; permanently disables reconciliation if sentinel fires for an unrelated cause | Explicit branch: log+skip only for sentinel, record everything else |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- `vpp` component's `setActiveConnector` at line 147 runs BEFORE `runOnce` starts the handshake. So "connector non-nil" does NOT imply "GoVPP connected". `ifacevpp.ensureChannel` currently relies on the subsequent `NewChannel()` call to fail, but that returns a generic `"govpp: not connected"` error. Checking `IsConnected()` up front is more robust and lets the sentinel fire cleanly.
- `applyConfig` already has a "partial success" contract — it returns `[]error`, not a single error, so multiple phases can fail independently. The not-ready branch slots in naturally: skip Phase 3 + Phase 4, let everything else proceed, return `errs` without that particular entry.
- The iface plugin's existing `unsubscribers` + `dhcpMu` machinery is the right place for the new vpp-event subscription. No new infrastructure needed.

## RFC Documentation

N/A — this is orchestration, not protocol.

## Implementation Summary

### What Was Implemented

- `iface.ErrBackendNotReady` sentinel (`internal/component/iface/backend.go:22`) with doc comment explaining the deferred-reconcile contract.
- `ifacevpp.ensureChannel` now checks `connector.IsConnected()` up front and returns `fmt.Errorf("ifacevpp: VPP connector not ready: %w", iface.ErrBackendNotReady)` without caching, so the sentinel path fires reliably and a later `ensureChannel` retries cleanly once vpp is up (`internal/plugins/ifacevpp/ifacevpp.go:91-116`).
- `reconcileOnReady(cfg, b)` helper in `internal/component/iface/config.go:1201` factors Phase 3 + Phase 4 out of `applyConfig`. Returns `(nil, true)` when `ListInterfaces` wraps `ErrBackendNotReady`, `(errs, false)` otherwise.
- `applyConfig` branches on `deferred`: when true, logs `iface reconcile deferred, backend not ready` at debug, runs the additive-only `addDesiredAddresses` fallback, and returns without recording an error (`config.go:1180-1188`).
- `reconcileOnVPPReady(activeCfg *atomic.Pointer[ifaceConfig])` is the package-level entry point called from the EventBus handler; no-ops when activeCfg is nil or backend unregistered (`config.go:1292-1310`).
- `register.go` wires subscriptions to `vppevents.EventConnected` + `vppevents.EventReconnected` inside `vppReadyOnce.Do` (so a config reload cannot double-subscribe); both handlers route through `events.AsString` to `reconcileOnVPPReady(&activeCfg)` and append to the existing `unsubscribers` slice for shutdown cleanup (`register.go:273-285`).
- `activeCfg` promoted from a plain `*ifaceConfig` to `atomic.Pointer[ifaceConfig]` so the EventBus goroutine reading it does not race with the SDK goroutine writing it during `OnConfigApply` (`register.go:135`).
- Functional test `test/vpp/006-iface-create.ci` extended with `reject=stderr:pattern=deliverConfigRPC failed` + `reject=stderr:pattern=list interfaces for reconciliation` (lines 162-165).
- Unit coverage: `TestReconcileOnReady_DefersOnSentinel`, `_RecordsNonSentinelError`, `_AddsMissing`, `_PrunesNonConfigInterface`, `TestApplyConfig_SkipsReconcileOnSentinel`, `TestReconcileOnVPPReady_NoOpWhenActiveCfgNil`, `_RunsReconcile`, `_InvokedOnEventConnected`, `_InvokedOnEventReconnected`, `TestUnsubscribeOnShutdown` (`internal/component/iface/config_test.go`). `TestEnsureChannel_NoConnectorReturnsSentinel`, `_NotConnectedReturnsSentinel`, `_NotReadyDoesNotCache` in `internal/plugins/ifacevpp/ifacevpp_test.go`.

### Bugs Found/Fixed

- **Generic `"govpp: not connected"` error.** The original `ensureChannel` relied on `NewChannel()` returning that string when the connector had been set but the handshake was still in flight (vpp.go:147 sets `ActiveConnector` before `runOnce` calls `Connect`). The error text never flowed as `ErrBackendNotReady`, so the sentinel branch never fired. Fixed by explicitly checking `connector.IsConnected()` before calling `NewChannel`.

### Documentation Updates

- None required. This spec is orchestration; no YANG, CLI, API, or RFC behavior changes. `docs/architecture/core-design.md` already documents iface↔vpp coupling at the component level; the event subscription is an internal refinement that does not alter the documented boundary.

### Deviations from Plan

- **`reconcileOnReady` signature.** Spec asks for a helper that returns `[]error`. Implementation returns `(errs []error, deferred bool)` instead -- the extra return value lets `applyConfig` distinguish the "backend not ready" case (which should not pollute `errs`) from a genuine reconcile failure without requiring the caller to `errors.Is`-scan the slice. Functionally equivalent to the spec's intent and aligns with the "Explicit > implicit" principle.
- **`vppReadyOnce`.** Not in the spec's file list, but added because `OnConfigure` can fire multiple times (SDK re-invokes on reload). Without the `sync.Once` the plugin would accumulate duplicate subscriptions across reloads.
- **`activeCfg` atomic promotion.** Spec's Security Review flagged the race as "audit during implementation". The atomic.Pointer promotion was required and was applied during Phase 3; no separate follow-up needed.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| iface defers reconciliation until vpp backend ready | Done | `internal/component/iface/config.go:1180-1188` | `reconcileOnReady` signals deferred; `applyConfig` falls back to additive-only |
| Re-run reconcile on `vppevents.EventConnected` / `EventReconnected` | Done | `internal/component/iface/register.go:273-285` | `vppReadyOnce` guarded, routed to `reconcileOnVPPReady` |
| Stop emitting "deliverConfigRPC failed" at startup | Done | `test/vpp/006-iface-create.ci:164` | `reject=stderr:pattern=deliverConfigRPC failed` |
| Netlink backend path unchanged | Done | `config.go:1201-1212` | Netlink's `ListInterfaces` never returns `ErrBackendNotReady`, so the `deferred=false` branch runs; no behavior change |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestEnsureChannel_NotConnectedReturnsSentinel`, `_NoConnectorReturnsSentinel` (`internal/plugins/ifacevpp/ifacevpp_test.go`) | Sentinel returned in both paths |
| AC-2 | Done | `TestReconcileOnReady_DefersOnSentinel`, `TestApplyConfig_SkipsReconcileOnSentinel` (`internal/component/iface/config_test.go`) | `deferred=true`, `errs` empty, desired addrs still applied |
| AC-3 | Done | `TestApplyConfig_SkipsReconcileOnSentinel` | `applyConfig` returns empty errs; `register.go:205` sets `activeCfg` after `OnConfigure` sees no error |
| AC-4 | Done | `TestReconcileOnVPPReady_InvokedOnEventConnected`, `TestReconcileOnVPPReady_RunsReconcile` | EventConnected emission triggers Phase 3 + Phase 4 via recordingEventBus |
| AC-5 | Done | `TestReconcileOnVPPReady_InvokedOnEventReconnected` | EventReconnected wired identically |
| AC-6 | Done | `TestReconcileOnReady_AddsMissing`, `_PrunesNonConfigInterface` | Netlink path: deferred=false, Phase 3 + Phase 4 run synchronously |
| AC-7 | Done | `TestUnsubscribeOnShutdown` | After unsubscribe, Emit has 0 subscribers and handler does not fire |
| AC-8 | Done | `TestReconcileOnReady_RecordsNonSentinelError` | Non-sentinel errors recorded in errs |
| AC-9 | Pending | `test/vpp/006-iface-create.ci` | Reject patterns added; test run via `make ze-verify-fast` validates end-to-end |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestEnsureChannel_NotConnectedReturnsSentinel` | Done | `internal/plugins/ifacevpp/ifacevpp_test.go:64` | |
| `TestEnsureChannel_NoConnectorReturnsSentinel` | Done | `ifacevpp_test.go:45` | |
| `TestApplyConfig_SkipsReconcileOnSentinel` | Done | `config_test.go:TestApplyConfig_SkipsReconcileOnSentinel` | |
| `TestApplyConfig_RecordsNonSentinelListError` | Changed | `TestReconcileOnReady_RecordsNonSentinelError` | Same AC (AC-8), asserts at the `reconcileOnReady` layer since applyConfig is a thin wrapper |
| `TestReconcileOnReady_AddsMissingRemovesStale` | Split | `TestReconcileOnReady_AddsMissing` + `_PrunesNonConfigInterface` | Two focused tests instead of one combined |
| `TestReconcileOnReady_InvokedOnEventConnected` | Done | `TestReconcileOnVPPReady_InvokedOnEventConnected` | Name uses `reconcileOnVPPReady` (actual symbol) |
| `TestReconcileOnReady_InvokedOnEventReconnected` | Done | `TestReconcileOnVPPReady_InvokedOnEventReconnected` | |
| `TestReconcileOnReady_NoOpWhenActiveCfgNil` | Done | `TestReconcileOnVPPReady_NoOpWhenActiveCfgNil` | |
| `TestUnsubscribeOnShutdown` | Done | `config_test.go:TestUnsubscribeOnShutdown` | |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/iface/backend.go` | Done | `ErrBackendNotReady` added |
| `internal/component/iface/config.go` | Done | `reconcileOnReady` + `reconcileOnVPPReady` + `addDesiredAddresses` |
| `internal/component/iface/register.go` | Done | `vppReadyOnce`, atomic.Pointer promotion, two subs appended to `unsubscribers` |
| `internal/plugins/ifacevpp/ifacevpp.go` | Done | `ensureChannel` checks `IsConnected()` |
| `test/vpp/006-iface-create.ci` | Done | Two reject patterns added (lines 163-165) |

### Audit Summary

- **Total items:** 9 AC + 9 TDD tests + 5 files = 23
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (test naming: `reconcileOnVPPReady` prefix; `AddsMissingRemovesStale` split into two focused tests)
- **Pending:** 1 (AC-9 -- covered by `make ze-verify-fast` functional run)

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | NOTE | `/ze-review` not invoked in this session (slash command, user-driven). Adversarial self-review applied instead (see below). | - | acknowledged |

### Adversarial self-review (questions from `rules/quality.md`)

| # | Question | Answer |
|---|----------|--------|
| 1 | What would `/ze-review-deep` find now? | activeCfg atomic promotion covers the SRV race; `vppReadyOnce` covers reload double-subscribe; no TODOs left. |
| 2 | Test cases skipped because unlikely? | None. Event-driven tests added for AC-4/5, shutdown cleanup, and defensive nil activeCfg. |
| 3 | Every new function reachable from a user entry point? Name the path. | `applyConfig -> reconcileOnReady`. `OnConfigure/OnConfigApply` writes activeCfg. EventBus from `vpp/vpp.go:225` reaches `reconcileOnVPPReady` via `register.go:278`. |
| 4 | If I doubled the test count, which tests would I add? | `TestReconcileOnVPPReady_DefersAgainWhenBackendStillNotReady` (second Connected event before connector handshake completes). Low-value since `IsConnected()` gating is already covered at the ifacevpp layer. Not added. |
| 5 | Unanswered questions? | None outstanding. |
| 6 | If I deliberately broke production, would the test catch it? | Yes: flipping the Subscribe namespace to a bogus string makes `bus.Emit` return n=0 and `TestReconcileOnVPPReady_InvokedOnEventConnected`'s `require.Equal(t, 1, n)` fails. Flipping reconcileOnVPPReady to no-op makes the orphan assertion fail. Confirmed by mental walkthrough, not re-run -- the tests PASS against the current impl and inspect concrete backend state changes (not just absence of errors), so observer-exit antipattern does not apply. |
| 7 | Renamed a registered name? Grepped consumers? | No renames. |
| 8 | Added a guard/fallback? Checked siblings? | `ensureChannel` gains an `IsConnected()` guard. `ensureChannel` is the ONLY caller of `NewChannel` in ifacevpp (grep confirms). No sibling call sites to update. |
| 9 | Reactor concurrency touched? Ran `make ze-race-reactor`? | No reactor code touched. N/A. |

### Fixes applied

- None post-review; all findings addressed during implementation (activeCfg atomic, vppReadyOnce, sentinel wrapping, subscription cleanup test).

### Final status

- [x] Adversarial self-review clean (0 BLOCKER, 0 ISSUE)
- [x] All NOTEs recorded above
- [ ] `/ze-review` formal run -- deferred to user (interactive slash command)

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/iface/backend.go` | Yes | `ls -la` shows modified |
| `internal/component/iface/config.go` | Yes | `ls -la` shows modified |
| `internal/component/iface/config_test.go` | Yes | `ls -la` shows modified |
| `internal/component/iface/register.go` | Yes | `ls -la` shows modified |
| `internal/plugins/ifacevpp/ifacevpp.go` | Yes | `ls -la` shows modified |
| `internal/plugins/ifacevpp/ifacevpp_test.go` | Yes | `ls -la` shows modified |
| `internal/plugins/ifacevpp/query.go` | Yes | `ls -la` shows modified |
| `test/vpp/006-iface-create.ci` | Yes | `ls -la` shows modified |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Sentinel returned when connector not ready | `go test ./internal/plugins/ifacevpp/...` all pass; `grep ErrBackendNotReady internal/plugins/ifacevpp/ifacevpp.go` -> line 116 wrap |
| AC-2 | `applyConfig` skips reconcile on sentinel | `TestApplyConfig_SkipsReconcileOnSentinel` PASS (tmp/ready-gate/test.log:15) |
| AC-3 | OnConfigure returns nil with sentinel | `TestApplyConfig_SkipsReconcileOnSentinel` verifies applyConfig returns empty errs; `register.go:205` sets activeCfg after len(errs)==0 |
| AC-4 | EventConnected triggers reconcile | `TestReconcileOnVPPReady_InvokedOnEventConnected` PASS (tmp/ready-gate/test.log:6) |
| AC-5 | EventReconnected triggers reconcile | `TestReconcileOnVPPReady_InvokedOnEventReconnected` PASS (tmp/ready-gate/test.log:8) |
| AC-6 | Netlink path unchanged | `TestReconcileOnReady_AddsMissing`, `_PrunesNonConfigInterface` PASS (tmp/ready-gate/test.log:17-19) |
| AC-7 | Unsubscribe on shutdown | `TestUnsubscribeOnShutdown` PASS (tmp/ready-gate/test.log:10) |
| AC-8 | Non-sentinel errors surface | `TestReconcileOnReady_RecordsNonSentinelError` PASS (tmp/ready-gate/test.log:13) |
| AC-9 | 006-iface-create.ci passes with reject patterns | `bin/ze-test vpp 006-iface-create` exit 0, 1/1 pass (tmp/ready-gate/ci-006.log) |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config `interface.backend=vpp` while vpp handshaking | `test/vpp/006-iface-create.ci` | Yes -- `bin/ze-test vpp 006-iface-create` exit 0; stderr contains no `deliverConfigRPC failed` and no `list interfaces for reconciliation` (rejects would have fired if present) |

### Verification state

- `golangci-lint run --timeout=120s ./cmd/ze/... ./cmd/ze-test/... ./internal/... ./pkg/... ./parked/... ./test/...` -- 1 pre-existing issue (`kernel_other_types.go:18 pppoxFD unused`, already logged in `plan/known-failures.md:327`); 0 new issues introduced by this spec.
- `go test -race ./internal/component/iface/... ./internal/plugins/ifacevpp/...` -- all tests pass (tmp/ready-gate/test.log).
- `bin/ze-test vpp 006-iface-create` -- 1/1 PASS in 5.0s (tmp/ready-gate/ci-006.log).
- `make ze-verify-fast` was NOT run to completion in this session because the repo's `verify-lock.sh` requires `flock` which is not installed on this Darwin host. The individual stages (lint scope matching ze-verify-fast, unit tests on affected packages, functional test for this spec) were each run directly and all pass on the in-scope packages.

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify-fast` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (or confirmed not needed)
- [ ] Critical Review passes

### Quality Gates

- [ ] RFC constraint comments (N/A here)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design

- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests (N/A — no numeric ranges)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)

- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written
- [ ] Summary included in commit
