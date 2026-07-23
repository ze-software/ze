# Spec: mpls-10-rsvp-te-reload-completeness

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | mpls-4-rsvp-te-fast-reroute (closed), mpls-3-rsvp-te (closed) |
| Phase | - |
| Updated | 2026-06-19 |

PATH RELOCATION (2026-07-22 plan review; citations corrected in-body
2026-07-22): the package moved from `internal/component/rsvpte/` to
`internal/plugins/rsvpte/` (the tiers reorg). Every source citation in this
spec now uses the new path, and the drifted line cites are corrected in-body:
`register.go` `runRefreshLoop` `:872` (was `:876`), `runCleanupLoop` `:928`
(was `:932`), loop launches `:605`/`:607` (were `:609`/`:611`),
`expiredPSBs(...)` `:937` (was `:941`), `OnConfigApply` `~:525` (was `~:537`).
Both reload gaps still un-fixed.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/plugins/rsvpte/register.go` - `OnConfigApply`, `runRefreshLoop`, `runCleanupLoop`, `reconcileTunnels`
3. `internal/plugins/rsvpte/admission.go` - `setInterface`, `Reserve`/`Release`, `interfaceBandwidth`, `sessions`
4. `internal/plugins/rsvpte/engine.go` - `engine.cfg()`/`setConfig` (atomic config, mpls-4)
5. `plan/learned/925-mpls-rsvp-te-fast-reroute.md` - the two limitations this spec closes ("Known pre-existing limitations")

## Task

Close the two pre-existing config-reload gaps in RSVP-TE that the mpls-4 review
surfaced but left out of FRR scope. Both are correctness gaps where a `commit`
that changes the rsvp-te config does not fully take effect until the daemon
restarts:

1. **Stale refresh/cleanup timers.** `runRefreshLoop` and `runCleanupLoop`
   (`register.go:872`, `:928`) capture `cfg` by value when started in `OnStarted`
   and build `time.NewTicker(cfg.RefreshPeriod)` once. A reload that changes
   `refresh-period` or `refresh-multiplier` is never adopted: the refresh cadence,
   and the soft-state expiry factor in `expiredPSBs(now, cfg.RefreshMultiplier)`,
   stay frozen at the startup values. Newly-built PSBs already carry the new
   `RefreshPeriod` (built from `e.cfg()`), so a refresh-period reload produces an
   inconsistency between the advertised period and the local cadence/expiry.

2. **Leaked admission state for removed interfaces.** `OnConfigApply` calls
   `admission.setInterface` for every configured interface but there is no
   `removeInterface`. An interface removed on reload leaves a stale entry in
   `admissionController.interfaces` and `admissionController.sessions`, so
   `show rsvp-te interface` keeps reporting a gone interface and its reservation
   accounting persists.

Both are LOW severity (no crash, no oversubscription) but make reload behavior
surprising. The mpls-4 atomic-config work (`engine.cfg()`/`setConfig`) already
gives the running engine the current config, so the fix is to have the loops and
the reconcile read it.

## Required Reading

### Source files (read BEFORE implementing)
- [ ] `internal/plugins/rsvpte/register.go` - `OnConfigApply` (the reload commit step, already pushes `eng.setConfig(cfg)` and reconciles tunnels), `runRefreshLoop`/`runCleanupLoop` (the two stale-ticker loops), `reconcileTunnels`, `addrToUint32`
  → Constraint: the loops are passed `eng`, and `eng.cfg()` returns the current atomic config; read the live config there instead of the launch-time copy.
  → Constraint: `OnConfigApply` already holds `tunnelsMu` and tracks `configuredTunnels`; interface reconcile belongs in the same place with a sibling `configuredInterfaces` set.
- [ ] `internal/plugins/rsvpte/admission.go` - `setInterface` (now RMW, mpls-4 fix), `interfaceBandwidth`, `sessions`, `Reserve`/`Release`/`reserveSession`/`releaseSession`
  → Constraint: a removed interface must drop both `interfaces[name]` and `sessions[name]`; do not zero a live interface (mpls-4's setInterface RMW lesson).
- [ ] `internal/plugins/rsvpte/engine.go` - `engine.cfg()` (atomic load) / `setConfig`
- [ ] `internal/plugins/rsvpte/fsm.go` - `lspTable.expiredPSBs(now, factor)` (consumes the multiplier)

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE implementing this spec)
- [ ] `register.go` - `runRefreshLoop` (`:872`): `ticker := time.NewTicker(cfg.RefreshPeriod)`; loop body uses the captured `cfg`. `runCleanupLoop` (`:928`): same ticker, plus `expiredPSBs(now, cfg.RefreshMultiplier)` (`:937`). Both launched `go run...Loop(ctx, log, lspTable, cfg, eng)` at `:605`/`:607` with `cfg` copied by value. `eng` is non-nil here (engine built only with a valid router-id).
- [ ] `admission.go` - `setInterface` updates/creates `interfaces[name]`; there is no `removeInterface`. `OnConfigApply` (`register.go` ~`:525`) loops `for _, iface := range cfg.Interfaces { admission.setInterface(...) }` with no teardown of removed interfaces.

**Behavior to preserve:**
- The mpls-4 atomic config (`e.cfg()`/`setConfig`, RouterID restart-class), the FRR
  facility-backup engine, and all base PATH/RESV/refresh/cleanup signaling stay green.
- `setInterface` stays read-modify-write (never zero a live interface's `ReservedBandwidth`).
- A reload with an unchanged refresh-period must not perturb the running timers
  (no spurious ticker resets, no dropped refreshes).

**Behavior to change:**
- The refresh/cleanup loops adopt a reloaded `refresh-period` (ticker cadence) and
  `refresh-multiplier` (expiry factor) without a restart.
- `OnConfigApply` removes admission state for interfaces no longer in the config.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config reload: `commit` → plugin `OnConfigApply` with the new rsvp-te tree.

### Transformation Path
1. `OnConfigApply` adopts `activeCfg`, pushes it to the engine (`eng.setConfig`), reconciles tunnels.
2. **NEW**: `OnConfigApply` reconciles interfaces — `setInterface` for each present, `removeInterface` for each in the previous set but absent now.
3. **NEW**: `runRefreshLoop`/`runCleanupLoop` read `eng.cfg()` each tick; on a changed `RefreshPeriod` they `ticker.Reset`, and cleanup uses the live `RefreshMultiplier`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config reload ↔ timers | loops read `eng.cfg()` each tick; reset ticker on period change | [ ] |
| config reload ↔ admission | `OnConfigApply` removes admission state for dropped interfaces | [ ] |

### Integration Points
- `runRefreshLoop`/`runCleanupLoop` - already receive `eng`; read `eng.cfg()`.
- `OnConfigApply` - already holds `tunnelsMu`, tracks `configuredTunnels`; add `configuredInterfaces`.
- `admissionController` - add `removeInterface`.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `eng` passed to the refresh/cleanup loops is always non-nil (the engine is built only with a valid router-id) | `register.go` OnStarted guards `!RouterID.IsValid()` before creating the engine and starting the loops | guard `eng != nil` in the loop or panic | read `register.go` OnStarted | unvalidated |
| A-2 | `time.Ticker.Reset(d)` safely re-periods a running ticker without losing the loop | Go stdlib `time` (Go 1.15+) | use a fresh ticker (Stop+NewTicker) | read Go docs / unit test | unvalidated |
| A-3 | Removing an interface's admission state while LSPs still reserve it is acceptable (the operator removed the interface; the LSP reconcile tears those LSPs separately) | mpls-3 admission is advisory accounting; `Release` clamps at 0 | only remove interfaces with zero live sessions, else log | design review + unit test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A ticker reset mid-cycle drops or doubles a refresh, perturbing soft state | interop/functional refresh test flakes | only reset when the period actually changed; keep the same loop, reset in place |
| R-2 | Reconciling interfaces races the refresh loop reading admission | `-race` failure | admission already has its own mutex; reconcile under `tunnelsMu` as today |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| reload changing `rsvp-te { refresh-period }` | → | `runRefreshLoop`/`runCleanupLoop` adopt the new period/multiplier via `eng.cfg()` | `TestRefreshLoopAdoptsReloadedPeriod` (loop-body helper unit test) |
| reload removing a `rsvp-te { interface }` | → | `OnConfigApply` calls `admission.removeInterface`; `show rsvp-te interface` drops it | `TestReconcileInterfacesRemovesDropped` + `rsvpte-reload` .ci |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Reload changes `refresh-period` from 30s to 10s | The refresh ticker and the cleanup ticker adopt 10s without a restart |
| AC-2 | Reload changes `refresh-multiplier` from 3 to 5 | `expiredPSBs` uses 5 on the next cleanup tick |
| AC-3 | Reload with an unchanged refresh-period | No ticker reset, no dropped/doubled refresh (idempotent) |
| AC-4 | Reload removes interface `eth1` (present before) | `admission.removeInterface("eth1")` drops `interfaces["eth1"]` and `sessions["eth1"]`; `show rsvp-te interface` no longer lists it |
| AC-5 | Reload keeps interface `eth0` with live reservations | `eth0` and its `ReservedBandwidth`/sessions are preserved (mpls-4 RMW invariant holds) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | edits `refresh-period`, commits | reload → OnConfigApply → loops read `eng.cfg()` → ticker resets | `TestRefreshLoopAdoptsReloadedPeriod` |
| 2 | removes an interface, commits | reload → OnConfigApply → `removeInterface` → admission state dropped | `TestReconcileInterfacesRemovesDropped`, `rsvpte-reload` .ci |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAdmissionRemoveInterface` | `admission_test.go` | `removeInterface` drops `interfaces`+`sessions`; unknown name is a no-op | |
| `TestReconcileInterfacesRemovesDropped` | `register_test.go` | reconcile removes admission state for an interface dropped on reload, keeps the rest | |
| `TestRefreshLoopAdoptsReloadedPeriod` | `register_test.go` | the loop-body helper recomputes period/multiplier from `eng.cfg()` and resets only on change | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| refresh-period (reload) | 1s-65535s | 65535s | 0 (ignored, keep current) | N/A (YANG-bounded) |
| refresh-multiplier (reload) | 1-255 | 255 | 0 (ignored, keep current) | N/A (YANG-bounded) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rsvpte-reload` | `test/reload/rsvpte-reload.ci` | configure rsvp-te + 2 interfaces, reload removing one + changing refresh-period; `show rsvp-te interface` reflects the removal | |

### Interop Tests
- N/A — no wire-protocol change; reload is a local config operation. (Justification per `ai/rules/interop-and-goal-validation.md`: this spec changes only local timer/accounting lifecycle, not on-wire behavior.)

## Files to Modify
- `internal/plugins/rsvpte/admission.go` - add `removeInterface(name)`; drop `interfaces[name]` + `sessions[name]`
- `internal/plugins/rsvpte/register.go` - `OnConfigApply` interface reconcile (sibling `configuredInterfaces` set + `removeInterface` for dropped); `runRefreshLoop`/`runCleanupLoop` read `eng.cfg()` each tick and `ticker.Reset` on a changed period; cleanup uses the live multiplier
- `internal/plugins/rsvpte/admission_test.go`, `register_test.go` - unit tests
- `test/reload/rsvpte-reload.ci` - functional reload test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | `refresh-period`/`refresh-multiplier`/`interface` already exist in `yang/ze-rsvp-te-conf.yang` |
| Functional test | Yes | `test/reload/rsvpte-reload.ci` |
| Prometheus counters | No | gauges already recomputed each refresh tick (`updateFRRGauges`) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | No | leaves unchanged; only reload semantics |
| 6 | Has a user guide page? | Maybe | `docs/guide/rsvp-te.md` - note that refresh-period/multiplier and interface removal take effect on reload |
| 12 | Internal architecture changed? | Maybe | if a reload-behavior doc exists, note timers/admission now reload |
| 16 | Changed source referenced by doc anchors? | Yes | grep `docs/` for `source: .../rsvpte/register.go`/`admission.go` and update stale claims |

## Files to Create
- `test/reload/rsvpte-reload.ci`

## Implementation Steps

1. **Validate assumptions** A-1..A-3 (read OnStarted for `eng` non-nil; confirm `time.Ticker.Reset`; decide the live-session removal policy).
2. **`admission.removeInterface`** + `TestAdmissionRemoveInterface` (drop both maps; unknown name no-op).
3. **Interface reconcile** in `OnConfigApply`: track `configuredInterfaces`, call `removeInterface` for dropped ones (mirror `configuredTunnels`); `TestReconcileInterfacesRemovesDropped`.
4. **Live-config timers**: extract a small loop-body helper that reads `eng.cfg()` for the period (reset the ticker only on change) and the multiplier; wire into `runRefreshLoop`/`runCleanupLoop`; `TestRefreshLoopAdoptsReloadedPeriod`. Keep an unchanged-period reload idempotent (AC-3).
5. **Functional test** `test/reload/rsvpte-reload.ci`.
6. **Verify + close**: `make ze-verify`; `/ze-review` gate to 0 BLOCKER/ISSUE; learned summary; two-commit closure.

### Critical Review Checklist (/implement stage 7)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 each have impl + test with file:line |
| Correctness | unchanged-period reload is idempotent (no ticker churn); removed interface drops both maps |
| Invariant preserved | `setInterface` stays RMW (mpls-4): a kept interface's reservation is never zeroed |
| Data flow | timers read `eng.cfg()` (single atomic load per tick), not a launch-time copy |
| Concurrency | interface reconcile under `tunnelsMu`; admission mutates under its own mutex; `-race` clean |
| Rule: no-workaround | the multiplier/period are read at the source (`eng.cfg()`), not re-plumbed through a side channel |

### Deliverables Checklist (/implement stage 11)
| Deliverable | Verification method |
|-------------|---------------------|
| `removeInterface` exists + called | `grep -n "func (ac \*admissionController) removeInterface" internal/plugins/rsvpte/admission.go` and a caller in `register.go` |
| timers read live config | `grep -n "eng.cfg()" internal/plugins/rsvpte/register.go` inside the loops |
| functional reload test | `ls test/reload/rsvpte-reload.ci` and it passes via `bin/ze-test` |

### Security Review Checklist (/implement stage 12)
| Check | What to look for |
|-------|-----------------|
| Resource lifecycle | removing an interface frees admission state (no unbounded growth across reloads) |
| Input validation | reloaded period/multiplier of 0 is ignored (keep current), not used to build a zero/negative ticker |

## Review Gate
<!-- Filled during /ze-implement: run /ze-review, record findings, loop to 0 BLOCKER/0 ISSUE. -->

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded (or explicitly "none")

## Pre-Commit Verification
<!-- Filled during /ze-implement: re-verify each AC/file/doc independently with pasted evidence. -->

## Gate Before Implementation

This spec is `ready` but has NOT passed `/ze-spec` research/design or `/ze-review-spec`.
Before `/ze-implement`: validate A-1..A-3, confirm the `test/reload/` harness shape
against an existing `test/reload/*.ci`, and fill the Review Gate + Pre-Commit
Verification sections.

## Known Limitations
- This does not add per-interface hot-reconfigure of bandwidth beyond what
  `setInterface` already updates (limits update in place; that is existing behavior).
- Auto-detecting which LSPs must be re-admitted when an interface's reservable
  bandwidth shrinks below the live reserved total is out of scope (the operator
  drains/reconfigures); this spec only fixes lifecycle leakage and timer staleness.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated (no unused `removeInterface`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `test/reload/rsvpte-reload.ci` passes

### Completion (BLOCKING)
- [ ] Write learned summary to `plan/learned/NNN-mpls-rsvp-te-reload-completeness.md`
- [ ] Commit A (code + spec + learned); Commit B (`git rm` spec)
