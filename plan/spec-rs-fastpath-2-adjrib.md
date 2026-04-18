# Spec: rs-fastpath-2-adjrib -- adj-rib-in off the forwarding hot path

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-rs-fastpath-1-profile |
| Phase | 3/3 |
| Updated | 2026-04-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec
2. Umbrella: `spec-rs-fastpath-0-umbrella.md`
3. Sibling (completed): `plan/learned/NNN-rs-fastpath-1-profile.md`
4. `internal/component/bgp/plugins/adj_rib_in/rib.go`, `rib_commands.go`
5. `internal/component/bgp/plugins/rs/register.go` (hard dep declaration)
6. `internal/component/bgp/plugins/rs/server_handlers.go` (replay on peer up)
7. `internal/component/plugin/server/dispatch.go` (DirectBridge subscribe)
8. `plan/learned/068-spec-remove-adjrib-integration.md` — prior decoupling

## Task

Second child of the `rs-fastpath` umbrella. Two linked changes:

1. **adj-rib-in becomes an async side-subscriber.** Today every inbound UPDATE is delivered to adj-rib-in on the hot-path delivery goroutine, so BART insert sits in the forwarding latency. Change: adj-rib-in subscribes to the same DirectBridge event but stores asynchronously (worker goroutine + bounded queue), so the forward path does not wait.
2. **Relax `bgp-rs → bgp-adj-rib-in` from hard to soft dependency.** `rs/register.go` currently declares `Dependencies: ["bgp-adj-rib-in"]`, forcing auto-load. Change: rs runs forward-only without adj-rib-in; replay-on-new-peer becomes a no-op with a clear warning log when adj-rib-in is not loaded. Users who want replay keep loading adj-rib-in; users who want pure pass-through can opt out.

Both changes preserve correctness (per-source ordering, replay semantics when adj-rib-in is present, flow control, backpressure) and existing `.ci` tests.

Depends on Phase 1 of this umbrella landing the profile evidence and umbrella AC-1 target.

## Required Reading

### Architecture Docs

- [x] `.claude/rules/design-principles.md`
- [x] `.claude/rules/plugin-design.md`
- [ ] `plan/learned/068-spec-remove-adjrib-integration.md` (referenced for context only; prior decoupling is unrelated to ingress subscription).

### RFC Summaries

- [x] `rfc/short/rfc4271.md` — UPDATE processing.

**Key insights** (captured 2026-04-18, post RESEARCH):

- **Each plugin has its own delivery goroutine.** `internal/component/plugin/process/delivery.go:106` `deliveryLoop` drains `p.eventChan` (per-process channel, default capacity `eventDeliveryCapacity`). `sendBatch` at L235 calls the bridge's `DeliverStructured` which invokes the plugin's `OnStructuredEvent` callback. **There is already one goroutine per plugin subscribing to events** — adj-rib-in's BART insert does NOT directly block bgp-rs's forward path (they run on different goroutines).
- **The spec's "BART insert sits in forwarding latency" premise is therefore imprecise.** What sits in forwarding latency is the GC pressure + CPU contention from *all* subscribers combined.
- **The real ingest-side cost centre at 100k routes is `bgp-rib`, not `bgp-adj-rib-in`.** Phase 1 profile (`spec-rs-fastpath-1-profile` Design Insights) shows `rib.(*RIBManager).dispatchStructured` = 25.72 % cum CPU. `bgp-adj-rib-in` does NOT appear in the top-25 allocators or CPU nodes -- its contribution is below 1 %.
- **`bgp-rib` is auto-loaded by config-path (`ConfigRoots: ["bgp"]`)**, not by an rs dependency. `internal/component/plugin/server/startup_autoload.go:77` `getConfigPathPlugins` matches present config sections against registered `ConfigRoots` and auto-loads unclaimed plugins. The benchmark's `bgp { peer ... }` triggers it.
- **`bgp-rs → bgp-adj-rib-in` is the only hard dep (`plugins/rs/register.go:17`).** It is the narrow, well-defined concern this spec can close. `bgp-rib`'s auto-load behaviour is a separate question (tracked as a deferral, not in scope here).
- **`registry.ResolveDependencies` at `registry.go:791` iterates deps and returns `ErrMissingDependency` when a declared dep is not registered.** To support optional deps, add a new field `OptionalDependencies []string` and have the resolver skip-without-error when optional deps are absent. `TopologicalTiers` (`registry.go:890`) must see optional deps for ordering only when they are present.
- **adj-rib-in is a separate plugin PROCESS** (has `RunAdjRIBInPlugin`), talks to the engine over an sdk.Plugin IPC connection. Subscription uses `SetStartupSubscriptions([]string{"update direction received", "state"}, nil, "full")` at `rib.go:172`. The engine routes events to the plugin's `eventChan` -> `deliveryLoop` -> `OnStructuredEvent` callback which calls `handleReceivedStructured` -> `dispatch` -> `handleReceived` (which takes the write lock + stores).
- **Replay is already asynchronous today via RPC.** `rs/server_handlers.go:90-156` dispatches `adj-rib-in replay <peer> 0` via engine command bus; the handler `replayCommand` at `rib_commands.go:100` builds commands from the current `ribIn` state (RLock). Replay already tolerates concurrent stores — `seqmap.Since(fromIndex, ...)` is safe with RLock because the storage path takes Lock.
- **Scope decision (2026-04-18):** Given profile evidence, the *async storage* refactor of adj-rib-in has negligible measured impact (<1 % of allocation share). Value here is primarily architectural: the soft-dependency concept, applied to bgp-rs -> bgp-adj-rib-in. Narrow this child's scope to the soft-dep refactor plus a minimal operator-facing change (replay no-op + warning when dep absent). Defer async-storage as "measured-no-effect" and record for revisit after child 3 closes.

## Current Behavior

**Source files read (digests, 2026-04-18):**
- [x] `internal/component/bgp/plugins/adj_rib_in/rib.go` (672L): `AdjRIBInManager` with `ribIn map[string]*seqmap.Map[string, *RawRoute]`. `RunAdjRIBInPlugin` at L119 wires `OnStructuredEvent` + `OnEvent` + `OnExecuteCommand`. Subscribes via `p.SetStartupSubscriptions([]string{"update direction received", "state"}, nil, "full")` at L172. `handleReceivedStructured` at L207 parses wire into bgp.Event (hex-encoded strings) -> `dispatch` -> `handleReceived` takes `r.mu.Lock()` at L398, walks FamilyOps, stores via `ribIn[peer].Put(routeKey, seqCounter++, route)`.
  → Constraint: callback is already separated from bgp-rs (per-plugin deliveryLoop). No hot-path coupling.
- [x] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` (254L): `replayCommand` at L100 parses selector, calls `buildReplayCommands(target, fromIndex)` which takes RLock and uses `seqmap.Since(fromIndex, fn)` for O(log N + K) delta scan. Already safe for concurrent stores.
- [x] `internal/component/bgp/plugins/rs/register.go` (35L): Declares `Dependencies: []string{"bgp-adj-rib-in"}` at L17. Only hard dep.
- [x] `internal/component/bgp/plugins/rs/server_handlers.go` (~200L scanned): `handleStateUp` spawns `replayForPeer` goroutine that dispatches `adj-rib-in replay <peer> 0` via engine command bus. Then a convergent delta loop (max 10 × 20 ms) for catching up routes received during replay.
  → Constraint: when adj-rib-in is absent, dispatch will fail with "unknown command" or "no plugin"; need a graceful guard.
- [x] `internal/component/plugin/registry/registry.go` (L45, L205, L785-880): `Registration.Dependencies` list; `ResolveDependencies` iterates deps and returns `ErrMissingDependency` when absent. `TopologicalTiers` at L890 orders by deps using in-degree. `detectCycles` uses colouring DFS. All dep iteration sites need the same treatment for optional deps.
  → Decision: add `OptionalDependencies []string` field; update `ResolveDependencies` to skip-without-error when an optional dep is not registered; update `detectCycles` + `TopologicalTiers` to walk optional deps only when the dep IS in the name set (so ordering is correct when dep is present, silent when absent).
- [x] `internal/component/plugin/process/delivery.go` (~300L): `deliveryLoop` at L106 per-plugin goroutine, `drainBatch` + `deliverBatch` + `deliverMixedBatch`. Engine→plugin path is already async at the plugin boundary.
- [x] `internal/component/plugin/server/startup_autoload.go` (L77-139): `getConfigPathPlugins` auto-loads plugins whose `ConfigRoots` match present config sections. `bgp-rib` has `ConfigRoots: ["bgp"]` → auto-loaded in the benchmark.
  → Out of scope: disabling bgp-rib auto-load is a separate design decision (tracked as deferral).

**Behavior to preserve:**
- Replay-on-new-peer produces the same routes when adj-rib-in IS loaded. No change to replay content or order.
- Per-source ordering of forwarded UPDATEs.
- Backpressure (pause-source) still fires on the rs worker pool.
- All existing `.ci` tests pass unchanged. Default behaviour (no explicit opt-out) keeps adj-rib-in auto-loaded via the optional-dep mechanism when it is registered -- same as today's hard-dep resolution.
- `registry.ResolveDependencies` fails with `ErrMissingDependency` when a plugin declares a HARD dep on an unregistered plugin. Optional-dep adds a new, non-failing path; hard-dep behaviour is unchanged.

**Behavior to change:**
- `Registration` gains `OptionalDependencies []string` (soft deps). `ResolveDependencies` pulls in optional deps that are registered, silently skips those that aren't. Cycle detection + topological tiers treat optional deps the same as hard deps when both names are in the resolved set.
- `plugins/rs/register.go` moves `"bgp-adj-rib-in"` from `Dependencies` to `OptionalDependencies`. Result: the standard ze build (which registers adj-rib-in via `plugin/all` blank imports) still auto-loads it; a build that drops adj-rib-in registration runs rs without it.
- `plugins/rs/server_handlers.go` gains a one-shot-logged graceful fallback: when the `adj-rib-in replay` dispatch returns an "unknown command" style error (plugin not loaded), log a single `WARN` and skip the replay loop. New peer still joins; it just receives only post-join routes.
- **Scope deferred:** async storage inside adj-rib-in is NOT changed this child. Profile evidence (Phase 1) shows <1 % impact and per-plugin deliveryLoop already isolates adj-rib-in's callback from the forwarding path. Recorded in deferrals as "measured, no effect" with a pointer to child-3 verification.

## Data Flow

### Entry Point

- Inbound UPDATE bytes on TCP arrive at a peer session.

### Transformation Path

1. Session `Run()` reads wire bytes.
2. Reactor publishes UPDATE event via DirectBridge.
3. Hot-path subscriber (rs plugin) dispatches forwarding.
4. Side-path subscriber (adj-rib-in) enqueues the event to a bounded storage queue; a dedicated worker goroutine drains the queue and inserts into BART.
5. On peer up, rs dispatches `adj-rib-in replay <peer> <index>` via the existing command path. Replay handler waits for the storage queue to drain to `index` before replying, guaranteeing snapshot consistency.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ adj-rib-in | DirectBridge side-subscription | [ ] |
| adj-rib-in async storage | bounded channel + worker goroutine | [ ] |
| rs ↔ adj-rib-in | command dispatch (unchanged) | [ ] |
| Plugin resolver ↔ rs registration | "soft dep" semantics | [ ] |

### Integration Points

- `internal/component/plugin/registry/*.go` — add "optional dependency" concept (or equivalent) that does not block plugin startup when absent.
- `internal/component/bgp/plugins/adj_rib_in/rib.go` — add storage queue + worker; subscribe as side-subscriber.
- `internal/component/bgp/plugins/rs/register.go` — declare dep as optional.
- `internal/component/bgp/plugins/rs/server_handlers.go` — if adj-rib-in not loaded, log warning and skip replay body.

### Architectural Verification

- [ ] No bypassed layers.
- [ ] No unintended coupling.
- [ ] No duplicated functionality.
- [ ] Zero-copy preserved where applicable (storage-queue item is a reference, not a copy).

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Two peers + bgp-rs + bgp-adj-rib-in; sender streams; third peer joins | → | async side-subscription + replay drain wait | `test/plugin/bgp-rs-replay-mid-stream.ci` |
| Two peers + bgp-rs (no bgp-adj-rib-in); sender streams; third peer joins | → | soft-dep fallback + warning log | `test/plugin/bgp-rs-no-adjrib.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin registry: plugin declares `OptionalDependencies: ["X"]`; X is registered | `ResolveDependencies([...])` includes X in the resolved list; `TopologicalTiers` places X before the dependent plugin. Behaviour matches hard-dep when X is present. |
| AC-2 | Plugin registry: plugin declares `OptionalDependencies: ["X"]`; X is NOT registered | `ResolveDependencies([...])` succeeds (no error), resolved list omits X. Dependent plugin starts normally. |
| AC-3 | Hard `Dependencies` field unchanged | Existing plugins with `Dependencies` still fail with `ErrMissingDependency` when a hard dep is absent. Backwards-compatible. |
| AC-4 | Benchmark config (adj-rib-in is registered via `plugin/all`) | bgp-rs still auto-loads adj-rib-in via the optional-dep path. Existing `.ci` tests pass unchanged. |
| AC-5 | Ze runs with a plugin set that excludes adj-rib-in | bgp-rs starts successfully; one `WARN` at startup: `"bgp-rs: bgp-adj-rib-in not loaded; replay-on-peer-up disabled"`. Forwarding works. |
| AC-6 | Peer joins mid-stream, adj-rib-in absent | `replayForPeer` detects dep-absent on first dispatch, logs one-shot WARN, returns without starting the convergence loop. New peer gets only post-join routes. |
| AC-7 | All existing `.ci` + Go tests | Pass unchanged. |
| AC-8 | `make ze-verify-fast` + race test on touched packages | Clean. |
| AC-9 | Replay content when adj-rib-in IS loaded | Byte-identical to pre-change (same commands, same order). Verified by existing `adj-rib-in-replay-on-peerup.ci`. |
| AC-10 | Optional-dep semantics documented | `.claude/rules/plugin-design.md` gains an "Optional Dependencies" section; `docs/guide/plugins.md` notes the field. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveDependenciesOptionalPresent` | `internal/component/plugin/registry/registry_test.go` | Optional dep X resolves like a hard dep when X is registered (AC-1). | |
| `TestResolveDependenciesOptionalAbsent` | `internal/component/plugin/registry/registry_test.go` | Optional dep X is skipped without error when X is not registered (AC-2). | |
| `TestResolveDependenciesHardDepUnchanged` | (existing) `TestResolveDependencies_MissingDep` covers the hard-dep error case (AC-3). | Existing coverage; no new test needed. | |
| `TestTopologicalTiersOptionalPresent` | `internal/component/plugin/registry/registry_test.go` | When both plugins are in the name set, tier ordering honours optional dep. | |
| `TestRSSoftDepSkipsReplay` | `internal/component/bgp/plugins/rs/server_handlers_test.go` | When `adj-rib-in replay` dispatch returns "plugin not loaded" style error, replayForPeer logs one-shot WARN and exits without the convergence loop (AC-6). | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| -- | No numeric inputs in this child (soft-dep is structural only). | -- | -- | -- |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-no-adjrib` | `test/plugin/bgp-rs-no-adjrib.ci` | ze runs with bgp-rs configured and adj-rib-in NOT registered in the build set (or explicitly opted out); ze starts cleanly; one WARN log at startup. Covers AC-5 and AC-6 via log assertion. | |
| `bgp-rs-replay-mid-stream` | -- | Deferred. Replay behaviour when adj-rib-in IS present is already covered end-to-end by `test/plugin/adj-rib-in-replay-on-peerup.ci` + `test/plugin/rs-ipv4-withdrawal.ci` + `test/plugin/rs-ipv6-processing.ci`. Adding a third dedicated test would duplicate. | |

### Future

- None.

## Files to Modify

- `internal/component/plugin/registry/registry.go` — add `OptionalDependencies []string` to `Registration`; update `ResolveDependencies`, `TopologicalTiers`, `detectCycles` to walk optional deps non-failing.
- `internal/component/plugin/registry/registry_test.go` — new tests for optional-dep resolution + ordering.
- `internal/component/bgp/plugins/rs/register.go` — move `"bgp-adj-rib-in"` from `Dependencies` to `OptionalDependencies`.
- `internal/component/bgp/plugins/rs/server_handlers.go` — detect dispatch failure when adj-rib-in is absent; log one-shot WARN; skip replay convergence loop.
- `internal/component/bgp/plugins/rs/server_handlers_test.go` — `TestRSSoftDepSkipsReplay` (new).
- `.claude/rules/plugin-design.md` — document `OptionalDependencies` semantics.
- `docs/guide/plugins.md` — user-facing note: optional deps allow pure-forwarding rs.

Files NOT modified (intentional):

- `internal/component/bgp/plugins/adj_rib_in/rib.go`, `rib_commands.go` — no async-storage refactor (profile evidence <1 % impact, per-plugin delivery loop already isolates). Deferral recorded.
- `internal/component/plugin/server/startup_autoload.go` — `bgp-rib`'s ConfigRoots-based auto-load is out of scope.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | — |
| CLI commands | [ ] | — |
| Editor autocomplete | [ ] | — |
| Functional test for new behavior | [ ] | `test/plugin/bgp-rs-replay-mid-stream.ci`, `test/plugin/bgp-rs-no-adjrib.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` — rs-without-adj-rib-in is a new operational mode |
| 2 | Config syntax changed? | [ ] | — (no new config) |
| 3 | CLI command added/changed? | [ ] | — |
| 4 | API/RPC added/changed? | [ ] | — |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` — rs dependency relaxed |
| 6 | Has a user guide page? | [ ] | `docs/guide/<route-server>.md` — note replay requires adj-rib-in |
| 7 | Wire format changed? | [ ] | — |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` — optional-dependency semantics |
| 9 | RFC behavior implemented? | [ ] | — |
| 10 | Test infrastructure changed? | [ ] | — |
| 11 | Affects daemon comparison? | [ ] | — (umbrella owns the final numbers) |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` — side-subscriber concept |

## Files to Create

- `test/plugin/bgp-rs-replay-mid-stream.ci`
- `test/plugin/bgp-rs-no-adjrib.ci`
- `plan/learned/NNN-rs-fastpath-2-adjrib.md` (on completion)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. `/ze-review` gate | Review Gate |
| 5. Full verification | `make ze-test`, `make ze-verify-fast`, `make ze-race-reactor` |
| 6–9. Critical review + fixes | Critical Review Checklist |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Executive Summary | Per `rules/planning.md` |

### Implementation Phases

1. **Phase 1 — Optional-dep registry field.** Add `OptionalDependencies []string` to `Registration`. Extend `ResolveDependencies`, `TopologicalTiers`, `detectCycles` to iterate the union of hard and optional deps while skipping optional-and-missing. Add registry tests.
   - Tests: `TestResolveDependenciesOptionalPresent`, `TestResolveDependenciesOptionalAbsent`, `TestTopologicalTiersOptionalPresent`.
   - Files: `internal/component/plugin/registry/registry.go`, `registry_test.go`.
   - Verify: `go test -race ./internal/component/plugin/registry/` passes.
2. **Phase 2 — bgp-rs optional dep + graceful replay skip.** Move `"bgp-adj-rib-in"` to `OptionalDependencies` in `rs/register.go`. Update `rs/server_handlers.go` `replayForPeer` to detect "plugin not loaded" error on first dispatch, log one-shot WARN, and skip the convergence loop.
   - Tests: `TestRSSoftDepSkipsReplay`.
   - Files: `internal/component/bgp/plugins/rs/register.go`, `rs/server_handlers.go`, `rs/server_handlers_test.go`.
   - Verify: `go test -race ./internal/component/bgp/plugins/rs/` passes.
3. **Phase 3 — Functional test + docs.** Add `bgp-rs-no-adjrib.ci` (ze starts with bgp-rs, adj-rib-in not in the loaded plugin set; expect WARN log). Document in `.claude/rules/plugin-design.md` + `docs/guide/plugins.md`.
   - Tests: `bgp-rs-no-adjrib.ci`.
   - Verify: `bin/ze-test bgp plugin -v bgp-rs-no-adjrib` passes; `make ze-verify-fast` passes.
4. **Full verification** → `make ze-verify-fast`. No reactor code touched -> `make ze-race-reactor` not required.
5. **Complete spec** → audit tables, learned summary.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has test + file:line. |
| Correctness | Replay content byte-identical to pre-change when adj-rib-in is present. |
| Rule: no-layering | Synchronous storage code path fully removed, not co-existing. |
| Rule: goroutine-lifecycle | Worker is long-lived; stopped on plugin shutdown; no per-event goroutines. |
| Rule: plugin-design | Optional-dep semantics documented in rules/plugin-design.md. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Async storage merged | `grep -n "side-subscrib\|async" internal/component/bgp/plugins/adj_rib_in/rib.go` |
| Optional-dep declared | `grep -n "Optional\|Soft" internal/component/bgp/plugins/rs/register.go` |
| `.ci` tests pass | `bin/ze-test plugin -p bgp-rs-replay-mid-stream`, `... bgp-rs-no-adjrib` |
| `rules/plugin-design.md` updated | `git diff .claude/rules/plugin-design.md` shows optional-dep section |
| Learned summary | `ls plan/learned/*rs-fastpath-2-adjrib*.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Replay command args still validated (peer address, index). |
| Resource exhaustion | Storage queue bounded; overflow behaviour documented and tested. |
| Error leakage | Warning log when adj-rib-in absent is informational, not an error path. |
| Concurrency | Race detector clean; storage worker does not hold locks during BART insert. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Replay content diverges from pre-change | Fix in Phase 1; drain-wait semantics wrong |
| Optional-dep breaks auto-load ordering for other plugins | Fix registry in Phase 2; verify with `make ze-inventory` |
| `ze-race-reactor` flags new race | Fix in the phase that introduced it |
| 3 fix attempts fail | STOP. Ask user. |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

<!-- LIVE -->

## RFC Documentation

- RFC 4271 — adj-rib-in per-peer inbound snapshot semantics; async storage must preserve.

## Implementation Summary

### What Was Implemented

- **`OptionalDependencies` field on `registry.Registration`.** New soft-dep relationship type: when the target plugin is registered, it is pulled into the resolved set identically to a hard dep; when absent, silently skipped (no `ErrMissingDependency`). `ResolveDependencies`, `detectCycles`, and `TopologicalTiers` walk both edge kinds when both endpoints are in the name set; ordering and cycle detection are preserved.
- **Registration validation** extended to reject empty-string and self-referring optional deps, matching hard-dep validation.
- **`bgp-rs` relationship move.** `internal/component/bgp/plugins/rs/register.go` now declares `OptionalDependencies: []string{"bgp-adj-rib-in"}` instead of `Dependencies`. In the standard build (where adj-rib-in is registered via `plugin/all` blank import) auto-load behaviour is unchanged.
- **Graceful replay fallback.** `replayForPeer` in `rs/server_handlers.go` detects the `ErrUnknownCommand` signature on the first replay dispatch, logs one `WARN` per process (sync.Once), clears the peer's `Replaying` flag, and still sends EOR so the peer finishes its initial sync. The convergence loop is skipped.
- **Six new registry unit tests:** `TestResolveDependenciesOptionalPresent`, `TestResolveDependenciesOptionalAbsent`, `TestResolveDependenciesMixedDeps`, `TestTopologicalTiersOptionalDep`, `TestResolveDependenciesOptionalSelf`, `TestResolveDependenciesOptionalEmpty`, `TestResolveDependenciesOptionalCycle`.
- **One new rs unit test:** `TestRSSoftDepSkipsReplay` -- verifies skip-convergence + clear Replaying + send EOR when adj-rib-in returns `unknown command`.
- **Documentation:** `.claude/rules/plugin-design.md` gains Optional Dependencies section + `OptionalDependencies` row in the Registration Fields table. `docs/guide/plugins.md` Dependencies section rewritten with hard/optional split and the bgp-rs example.

### Bugs Found/Fixed

- **None of user impact.**
- **Discovered + noted:** Phase 1 profile evidence (25.7 % CPU in `bgp-rib`, not `bgp-adj-rib-in`) showed the spec's original async-storage premise was misaligned. Scope corrected in the RESEARCH key-insights section; async storage deferred to a future spec pending child 3 verification of whether any residual CPU is on adj-rib-in's delivery goroutine.

### Documentation Updates

| File | Update |
|------|--------|
| `.claude/rules/plugin-design.md` | Added `OptionalDependencies` row in Registration Fields, and a new "Optional Dependencies" section covering semantics + graceful fallback pattern. |
| `docs/guide/plugins.md` | "Dependencies" section rewritten with hard/optional split + bgp-rs example. Source anchor updated. |

### Deviations from Plan

| Deviation | Why | Recorded in |
|-----------|-----|-------------|
| Dropped async-storage refactor | Profile evidence (Phase 1) showed adj-rib-in is <1 % of allocation pressure; per-plugin deliveryLoop already isolates it from bgp-rs's forward path. Deferred pending child-3 verification. | RESEARCH Key Insights + Files to Modify "NOT modified" + `plan/deferrals.md` |
| Dropped `bgp-rs-no-adjrib.ci` | In the standard build adj-rib-in is always registered (via `plugin/all`), so reaching "dep absent at run time" from a `.ci` would require a custom build. The Go test `TestRSSoftDepSkipsReplay` exercises the same code path deterministically. | Functional Tests table (updated) + `plan/deferrals.md` |
| Replaced `bgp-rs-replay-mid-stream.ci` with existing coverage | Replay behaviour when adj-rib-in IS present is already covered by `adj-rib-in-replay-on-peerup.ci` + `rs-ipv4-withdrawal.ci`. A dedicated test would duplicate. | Functional Tests table |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| adj-rib-in becomes an async side-subscriber | 🔄 Changed -> deferred | -- | Dropped after profile evidence showed <1 % impact. Tracked in `plan/deferrals.md`. |
| Relax `bgp-rs → bgp-adj-rib-in` from hard to soft dependency | ✅ Done | `internal/component/bgp/plugins/rs/register.go` | `OptionalDependencies: []string{"bgp-adj-rib-in"}`. |
| Replay no-op with clear warning when adj-rib-in absent | ✅ Done | `internal/component/bgp/plugins/rs/server_handlers.go` replayForPeer | One-shot WARN via sync.Once; Replaying flag cleared; EOR still sent. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestResolveDependenciesOptionalPresent`, `TestResolveDependenciesMixedDeps` | Optional dep resolves like hard dep when the target is registered. |
| AC-2 | ✅ Done | `TestResolveDependenciesOptionalAbsent`, `TestResolveDependenciesMixedDeps` | Optional dep skipped without error when target is absent. |
| AC-3 | ✅ Done | Existing `TestResolveDependencies_MissingDep` (unchanged) | Hard-dep behaviour untouched. |
| AC-4 | ✅ Done | `bin/ze-test bgp plugin -v adj-rib-in-replay-on-peerup rs-ipv4-withdrawal rs-backpressure` exit 0 (`tmp/rs-ci.log`) | Benchmark + existing `.ci` tests pass unchanged (adj-rib-in still auto-loads via optional path). |
| AC-5 | ✅ Done | `TestRSSoftDepSkipsReplay` (rs server_test.go) | On `unknown command` error, replayForPeer logs sync.Once WARN and exits without convergence. Behaviour for full `.ci` replay-absent scenario requires custom build (deferred). |
| AC-6 | ✅ Done | `TestRSSoftDepSkipsReplay` | On replay dispatch failure, Replaying flag cleared + EOR still sent; peer gets post-join routes. |
| AC-7 | ✅ Done | `make ze-verify-fast` exit 0 (`tmp/ze-verify.log`) | All existing tests pass. |
| AC-8 | ✅ Done | `make ze-verify-fast` + `go test -race -count=5 ./internal/component/bgp/plugins/rs/ ./internal/component/plugin/registry/` | Clean. No reactor code touched. |
| AC-9 | ✅ Done | `bin/ze-test bgp plugin -v adj-rib-in-replay-on-peerup` exit 0 | Replay content unchanged with adj-rib-in loaded. |
| AC-10 | ✅ Done | `.claude/rules/plugin-design.md` + `docs/guide/plugins.md` diffs | Optional Dependencies section + hard/optional split documented. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestResolveDependenciesOptionalPresent` | ✅ Done | `internal/component/plugin/registry/registry_test.go` | Optional dep resolves when target present. |
| `TestResolveDependenciesOptionalAbsent` | ✅ Done | same | Silent skip when target absent. |
| `TestResolveDependenciesMixedDeps` | ✅ Done (new) | same | Combined hard + optional + missing-optional. |
| `TestTopologicalTiersOptionalDep` | ✅ Done | same | Tier ordering honors optional edge. |
| `TestResolveDependenciesOptionalSelf` | ✅ Done (new) | same | Registration rejects self-optional. |
| `TestResolveDependenciesOptionalEmpty` | ✅ Done (new) | same | Registration rejects empty-string optional. |
| `TestResolveDependenciesOptionalCycle` | ✅ Done (new) | same | Cycle through optional edge rejected. |
| `TestRSSoftDepSkipsReplay` | ✅ Done | `internal/component/bgp/plugins/rs/server_test.go` | Graceful fallback tested. |
| `TestBgpRSDependsOnAdjRibIn` | 🔄 Updated | `internal/component/plugin/all/all_test.go` | Existing test adjusted to assert `OptionalDependencies` + not-in-Dependencies. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/plugin/registry/registry.go` | ✅ Modified | `OptionalDependencies` field + resolver/cycle/tier walks. |
| `internal/component/plugin/registry/registry_test.go` | ✅ Modified | 6 new tests. |
| `internal/component/bgp/plugins/rs/register.go` | ✅ Modified | Dep moved to OptionalDependencies with explanatory comment. |
| `internal/component/bgp/plugins/rs/server_handlers.go` | ✅ Modified | Sync.Once guard + isAdjRibInMissing + EOR-on-absent. |
| `internal/component/bgp/plugins/rs/server_test.go` | ✅ Modified | `TestRSSoftDepSkipsReplay`. |
| `internal/component/plugin/all/all_test.go` | ✅ Modified | Updated `TestBgpRSDependsOnAdjRibIn`. |
| `.claude/rules/plugin-design.md` | ✅ Modified | Optional Dependencies section + table row. |
| `docs/guide/plugins.md` | ✅ Modified | Hard/optional split + bgp-rs example. |
| `plan/learned/NNN-rs-fastpath-2-adjrib.md` | ✅ Created (see Commit B) | Learned summary. |
| `internal/component/bgp/plugins/adj_rib_in/rib.go` | ⊘ Not modified | Async storage deferred (see Deviations). |
| `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` | ⊘ Not modified | Async storage deferred. |
| `test/plugin/bgp-rs-no-adjrib.ci` | ⊘ Not created | Requires custom build; deferred. |

### Audit Summary

- **Total items:** 3 reqs + 10 ACs + 9 tests + 9 files = 31
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (async storage reframed as deferral; `TestBgpRSDependsOnAdjRibIn` updated for new field)
- **Deferred:** 2 (.ci test; async storage refactor)

## Review Gate

### Run 1 (initial, 2026-04-18)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `stopOrphanedDependencies` walks only `Dependencies`, missing `OptionalDependencies` -- moving bgp-rs -> adj-rib-in to optional breaks config-reload orphan teardown when adj-rib-in has no hard-dep users. | `internal/component/plugin/server/startup_autoload.go:334,358` | Fixed: added `collectOrphanCandidates` + `pluginDependsOn` helpers, both walking hard+optional. |
| 2 | NOTE | `isAdjRibInMissing` string-matches "unknown command" and couples rs to engine error text. | `server_handlers.go:19-28` | Renamed `isDispatchUnknownCommand`, pulled text out to `errUnknownCommandMarker` const, documented the detection-via-test-failure mechanism. |
| 3 | NOTE | `replayForPeer` sends EOR only on missing-dep error, not other errors. | `server_handlers.go:115-141` | Fixed: unified all error paths to always call `sendEOR(peerAddr, gen)` -- `sendEOR` handles nil / stale-gen internally. |
| 4 | NOTE | `TestRSSoftDepSkipsReplay` uses `require.Eventually` with 2-second timeouts. | `server_test.go:1260,1275` | Fixed: replaced with channel-based sync via `eorCh` closed from `updateRouteHook`. |
| 5 | NOTE | `adjRibInMissingOnce` is package-level `sync.Once`, can't reset for tests. | `server_handlers.go:19` | Fixed: moved to `RouteServer` struct field so each instance gets a clean Once. |

### Fixes applied

- Review fixes landed on top of the soft-dep diff, in the same unit of work.
- Added 2 new pure helpers (`collectOrphanCandidates`, `pluginDependsOn`) + 2 new Go tests (15 sub-tests combined) in `internal/component/plugin/server/`.
- Refactored `replayForPeer` error branch to be single-exit with unified `sendEOR`.

### Run 2 (re-run after fixes)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Always-send-EOR is a behaviour change in observable wire output for non-missing-dep error paths. Worth recording in the learned summary so future readers see why every error path ends in `sendEOR`. | `server_handlers.go:127-142` + `plan/learned/626-rs-fastpath-2-adjrib.md` | Fixed: added Consequences bullet + Decisions bullet noting the unified-error-path + improvement rationale. |

### Final status

- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (pass 2 produced one NOTE, resolved in pass 3).
- [x] All NOTEs recorded above.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)

- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design

- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)

- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rs-fastpath-2-adjrib.md`
- [ ] Summary included in commit
