# spec-fixit-firewall-concurrency-deadlock

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-15 |

## Task

A command-dispatch deadlock was observed under QEMU when a `firewall { backend nft }`
config block (which drives `LoadBackend` + `RegisterTables` + `ApplyAll` in the firewall
engine) runs alongside the ddos-local responder's own `firewall.ApplyAll` under a
sustained flood: the daemon's `ze-plugin-engine:dispatch-command` stopped responding for
roughly 255 seconds. The root cause is UNVERIFIED: only the symptom was observed, and no
goroutine dump or lock trace was captured. This is a potential real concurrency bug in
shared firewall infrastructure (`internal/component/firewall`), NOT something introduced
by the spec that observed it. Goal: root-cause the hang and fix it at the owning layer,
or, if the contention is inherent, document a hard constraint and make the failure
observable instead of a silent hang.

Skeleton = captured intent with verified `file:line` evidence. Research happens via
`/ze-spec` when this is picked up; the spec moves to `design` then.

## Origin

`plan/deferrals.md` row dated 2026-07-12, "spec-ddos-direction-allowlist (QEMU
observation)", recorded as "none yet (future firewall-concurrency fixit)". The `.ci`
tests for that spec worked around the hang by asserting direction classification rather
than the on-host drop.

### Scope

- IN: root-cause the dispatch hang; fix it in shared firewall infra (or document the
  constraint with evidence); prove it with a reproduction.
- IN: the lock-discipline hazards found while writing this spec (see Problem / Evidence),
  on their own merits.
- OUT: new ddos features; the per-source drop-term narrowing and flowspec withdraw items
  from the same parent spec (separate deferral rows).

## Required Reading

### Source (read before designing)
<!-- NEVER tick [ ] to [x]. Capture insights as -> Decision: / -> Constraint:. -->
- [ ] `internal/component/firewall/registry.go` (:79-124 ApplyAll) - merges every owner's
      tables, then calls `backend.Apply` (:123) holding NO firewall-package lock. Prime
      suspect region.
- [ ] `internal/component/firewall/registry.go` (:64-72 RegisterTables) - takes
      `tableRegistry.mu`; nil tables delete the owner key.
- [ ] `internal/component/firewall/backend.go` (:130 LoadBackend) - takes `backendsMu`;
      `loadBackendLocked` (:137 comment) is the variant `ApplyAll` calls while already
      holding `backendsMu`.
- [ ] `internal/component/firewall/engine.go` (:316-324 OnConfigure) - `LoadBackend`, then
      `RegisterTables("firewall", ...)`, then `ApplyAll`.
- [ ] `internal/component/firewall/engine.go` (:372-399 OnConfigApply) - conditional
      `LoadBackend`, then journalled `RegisterTables` + `ApplyAll`, with a rollback
      closure that calls both again.
- [ ] `internal/plugins/ddos/local/responder.go` (:92-150 applyMitigation) - calls
      `registerTables` then `applyAll` (:135-136) while the caller holds `r.mu` (:88);
      on failure calls both again to roll back (:141-142), still under `r.mu`.
- [ ] `internal/plugins/ddos/local/responder.go` (:63-86 onDetected / onCharacterized) -
      event-bus handlers, both take `r.mu`; fire repeatedly under a sustained flood.
- [ ] `internal/plugins/firewall/nft/backend_linux.go` (:40 Apply) - the kernel reconcile.
      Need its cost and blocking behavior under load.
- [ ] `internal/component/plugin/server/dispatch_registry.go` (:256 opDispatchCommand) -
      shared handler for `dispatch-command`. Establish whether dispatch and event
      delivery share a goroutine or a lock.

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - named by `internal/component/firewall/registry.go:1`
      as the design home for the firewall table registry
  -> Constraint: (fill during research) what the registry promises about concurrent apply

**Key insights:** (fill during research)

## Current Behavior (MANDATORY)

**Source files read (2026-07-15, spec author):**
- [ ] `internal/component/firewall/registry.go` (:79-124 ApplyAll) - snapshots the merged
      table set under `tableRegistry.mu`, releases it (:94), takes `backendsMu` to read or
      autoload `activeBackend` (:97-111), releases it (:111), then calls `b.Apply(all)`
      (:123) holding no firewall-package lock
- [ ] `internal/component/firewall/registry.go` (:104 autoload) - `ApplyAll` autoloads
      `defaultBackendForAutoload` when no backend is loaded and `len(all) > 0`, so a
      plugin can trigger a backend load with no `firewall {}` block present
- [ ] `internal/component/firewall/registry.go` (:113-121 no-backend path) - returns nil
      when nothing to apply, `errFirewallBackendNotLoaded` when tables are pending
- [ ] `internal/plugins/ddos/local/responder.go` (:135-148) - registers the table and
      calls `applyAll` (plus a possible rollback pair) while holding `r.mu` throughout
- [ ] `internal/component/firewall/engine.go` (:379-399) - wraps `RegisterTables` +
      `ApplyAll` in an `sdk.Journal` record/rollback pair

**Behavior to preserve:**
- Table ownership semantics: owner keys, the `ze_*` prefix sweep, and the merge of
  same-name tables across owners (`mergeSameNameTables`, registry.go:95).
- The autoload path: a plugin registering tables with no `firewall {}` block still
  reaches the kernel (registry.go:104).
- The withdraw-as-no-op path: `RegisterTables(owner, nil)` + `ApplyAll` with no backend
  loaded stays a no-op (registry.go:113-121).
- Journalled rollback on a failed reload (engine.go:379).
- ddos-local mitigation still installs and withdraws promptly under an active flood.
- The three registry contract tests stay green: `TestApplyAllAutoLoadsDefaultBackend`,
  `TestApplyAllNoBackendNoTablesIsNoOp`, `TestApplyAllNoDefaultKeepsNotLoadedError`
  (`internal/component/firewall/registry_test.go:12,65,89`).

**Behavior to change:**
- None yet, research first. The fix location depends on whether the hang is lock
  contention inside `internal/component/firewall`, a slow or blocking nft `Apply`, or a
  stalled plugin-engine dispatch goroutine.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config: `firewall { backend nft }` section -> firewall engine `OnConfigure`
  (engine.go:294) / `OnConfigApply` (engine.go:348)
- Events: `ddosevent.Detected` / `Characterized` -> ddos-local responder handlers
  (`internal/plugins/ddos/local/register.go:89-90`)
- Operator: a CLI command -> `ze-plugin-engine:dispatch-command`
  (dispatch_registry.go:256) -- the surface observed to hang

### Transformation Path
1. Firewall engine parses the section, calls `LoadBackend` (engine.go:316/373)
2. Engine calls `RegisterTables("firewall", tables)` then `ApplyAll` (engine.go:321-324)
3. Concurrently: flood -> detector emits AttackDetected -> responder `onDetected` takes
   `r.mu` (responder.go:64) -> `applyMitigation` -> `registerTables` + `applyAll`
   (responder.go:135-136)
4. Both `ApplyAll` calls take `tableRegistry.mu`, release it, take `backendsMu`, release
   it (registry.go:80-111)
5. Both call `b.Apply(all)` (registry.go:123) with no firewall lock held -> nft reconcile
6. Meanwhile an operator command enters `opDispatchCommand` and does not return (~255s)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config -> firewall engine | plugin SDK OnConfigure / OnConfigApply | [ ] |
| event bus -> ddos-local responder | `ddosevent.Detected` / `Characterized` subscribe | [ ] |
| plugin -> shared firewall registry | `RegisterTables` + `ApplyAll` (two owners) | [ ] |
| firewall registry -> kernel | nft backend `Apply` (netlink / shell-out) | [ ] |
| operator -> plugin engine | `ze-plugin-engine:dispatch-command` RPC | [ ] |

### Integration Points
- `internal/component/firewall/` registry, backend, engine (the shared, owning layer)
- `internal/plugins/firewall/nft/` the Linux backend doing the kernel reconcile
- `internal/plugins/ddos/local/` the second `ApplyAll` caller under flood
- `internal/component/plugin/server/` dispatch path that stalled
- Other `ApplyAll` callers that share the hazard: `internal/plugins/copp/register.go`,
  `internal/plugins/policyroute/register.go`, `internal/plugins/flowspec-firewall/engine.go:177-180`,
  `internal/plugins/anomaly/shape/responder.go`, `internal/component/firewall/plugins/irr/irr.go`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - the fix must not special-case ddos inside the
      generic firewall package (`ai/rules/plugin-self-containment.md`)

## Problem / Evidence

**CONFIRMED (read in source, 2026-07-15):**
- `ApplyAll` calls `b.Apply(all)` with no firewall lock held (registry.go:97-123). Two
  concurrent callers (firewall engine + ddos responder) can therefore be inside
  `backend.Apply` at the same time. Whether the nft backend tolerates that is unverified.
- The ddos responder holds `r.mu` across the whole `applyAll` call, and across a second
  `applyAll` on the rollback path (responder.go:136-144): a lock held across a
  potentially slow kernel reconcile.
- `onDetected` and `onCharacterized` contend on `r.mu` (responder.go:64,74), so a
  sustained flood serialises repeated handlers behind each nft reconcile.
- ddos-local registers no command handler of its own (no `OnCommand` under
  `internal/plugins/ddos/local/`), so `r.mu` is not directly on the `dispatch-command`
  path. The link between `r.mu` and the observed dispatch hang is NOT established.

**OBSERVED (QEMU run, 2026-07-12, not reproduced since):**
- `ze-plugin-engine:dispatch-command` stopped responding for roughly 255 seconds with a
  `firewall { backend nft }` block configured while ddos-local mitigation was active
  under a sustained flood.

**UNVERIFIED:**
- The root cause. No goroutine dump, no lock-ordering trace, no nft timing captured.
- Whether this is a true deadlock (a cycle) or livelock / starvation (repeated reconciles
  under flood starving the dispatch path).
- Whether nft `Apply` blocks on a kernel or netlink lock held by another actor.
- Whether the 255s was bounded by a harness timeout rather than genuine recovery.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The hang reproduces under QEMU with the same config shape | observed once during spec-ddos-direction-allowlist QEMU work (deferrals.md:55) | no reproduction: the spec degrades to a lock-discipline audit with a weaker outcome | re-run flood + `firewall { backend nft }` under QEMU | unvalidated |
| A-2 | The contention lives in shared firewall infra, not the ddos plugin | deferrals.md:55 calls it "potential real concurrency bug in shared firewall infra"; `ApplyAll` is the only shared mutable path both actors touch | fix belongs in ddos-local or the plugin engine; AC-5 must change | goroutine dump showing blocked stacks | unvalidated |
| A-3 | nft `Apply` can be slow enough under flood to matter | inference from the ~255s stall; the nft backend reaches the kernel (backend_linux.go:40) | the stall is a genuine lock cycle, not slow-call-under-lock; fix is lock ordering | time `backend.Apply` under load | unvalidated |
| A-4 | `dispatch-command` shares a goroutine or lock with the blocked path | symptom is a dispatch hang while firewall work is in flight | the dispatch hang has an unrelated cause and this spec is scoped wrong | read dispatch_registry.go:256 and the engine dispatch model | unvalidated |
| A-5 | Concurrent `b.Apply` from two owners is a real hazard, not benign | registry.go:123 holds no lock across the call | the unlocked call is fine and only the dispatch path matters | nft backend concurrency review + concurrent-apply test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Not reproducible: root cause never established | QEMU re-run does not stall | fall back to a lock-discipline audit; fix the evidenced hazards on their own merits; do NOT claim the deadlock is fixed |
| R-2 | Serialising reconciles trades a deadlock for a longer stall | dispatch latency rises after the fix | measure before and after; prefer a single reconcile actor / queue over a coarse lock |
| R-3 | Fix regresses the autoload or withdraw-no-op paths | the three registry_test.go contract tests fail | keep them green throughout; they encode the contract |
| R-4 | Fix serialises mitigation behind config reloads, delaying a drop under attack | mitigation install latency rises under flood | measure install latency under load; consider a non-blocking mitigation path |
| R-5 | The repro never runs: `make ze-qemu-all-test` SKIPS the `firewall` suite by default (`mk/test-integration.mk:220` `ZE_QEMU_SKIP_SUITES ?= web,firewall`, passed through at `:239`; the script default agrees at `scripts/evidence/qemu-all-tests.sh:40`). A session reproducing under QEMU with the default target exercises no firewall `.ci` at all and may read the silence as "cannot reproduce" | QEMU run reports the firewall suite skipped, or finishes suspiciously fast | override it (`make ze-qemu-all-test ZE_QEMU_SKIP_SUITES=web`), or use `ze-qemu-needs-linux-test`, which hardcodes `ZE_QEMU_SKIP_SUITES="web"` (`:261`) and so DOES run firewall (`:248-250` names firewall explicitly). Verified by reading the targets, 2026-07-16 |
| R-6 | **The observed "deadlock" may be entangled with a known kernel crash, not only a Go lock hazard.** `mk/test-integration.mk:211-213` states the firewall suite is skipped by default because "firewall crashes the Alpine QEMU kernel on nft set-element-timeout operations". The deferral's symptom (dispatch unresponsive ~255s under a sustained flood with `firewall { backend nft }`) was observed in exactly that environment, so an unresponsive daemon there is not automatically a Go-level deadlock | the stall reproduces only under Alpine QEMU and never on real Linux or with a non-nft backend | before concluding anything about `ApplyAll` locking, establish WHERE the repro runs: real Linux vs Alpine QEMU, nft vs another backend. Note the two QEMU targets disagree about whether firewall is safe to run, which is itself unresolved. `plan/spec-fixit-qemu-runtime-kernel.md` owns moving the QEMU targets onto ze's own 7.1.1 kernel and un-skipping firewall; if it lands first this spec inherits a trustworthy repro environment and R-5/R-6 both fall away. Prefer waiting for it over debugging `ApplyAll` on a kernel ze itself declares unsupported (`tools/kernel-builder/build.py:38` refuses < 7.0) | 
| R-5 | Fix covers ddos only and leaves the other five `ApplyAll` callers exposed | review finds copp / policyroute / flowspec unchanged | fix at the registry layer so every owner benefits |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `firewall { backend nft }` configured while ddos-local mitigates under flood | -> | shared firewall apply path stays responsive | `test/plugin/ddos-firewall-concurrency.ci` |
| Two owners call `ApplyAll` concurrently | -> | `internal/component/firewall/registry.go` ApplyAll serialisation | `TestApplyAllConcurrentOwnersConverge` |
| Operator command during an active nft reconcile | -> | `ze-plugin-engine:dispatch-command` bounded latency | `test/plugin/ddos-firewall-concurrency.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Research phase complete | Root cause identified and evidenced (goroutine dump or lock trace showing blocked stacks), or an evidenced statement that it cannot be reproduced |
| AC-2 | A reproduction exists | A deterministic test (Go or `.ci`) reproduces the stall before the fix and passes after it |
| AC-3 | `firewall { backend nft }` configured while ddos-local mitigates under a sustained flood | `ze-plugin-engine:dispatch-command` keeps responding within a bounded time; no multi-second stall |
| AC-4 | Concurrent `ApplyAll` from two owners (firewall engine + a plugin) | The kernel converges to the merged desired state; no lost update, no interleaved partial apply |
| AC-5 | The fix lands | It lands at the owning layer (shared firewall infra), not as a workaround in ddos-local or in the `.ci` tests (`ai/rules/no-workarounds-for-missing-behavior.md`) |
| AC-6 | Contention proves inherent (fix not possible) | The constraint is documented at the owning layer with evidence, and the failure is observable (log / metric / bounded timeout) rather than a silent hang |
| AC-7 | ddos-direction `.ci` workaround | Re-evaluated: if the hang is fixed, the tests assert the on-host drop rather than only direction classification, or the spec records why not |
| AC-8 | Every other `ApplyAll` caller (copp, policyroute, flowspec-firewall, anomaly/shape, irr) | Benefits from the same fix; none regress |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator runs a CLI command while the box is mitigating a flood with a firewall block configured | command -> dispatch-command -> plugin engine -> response | `test/plugin/ddos-firewall-concurrency.ci` |
| 2 | Operator reloads firewall config while ddos-local holds a drop table | OnConfigApply -> RegisterTables + ApplyAll alongside the responder's | `test/plugin/ddos-firewall-concurrency.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyAllConcurrentOwnersConverge` | `internal/component/firewall/registry_test.go` | AC-4 concurrent owners converge, no lost update | |
| `TestApplyAllSerialisesBackendApply` | `internal/component/firewall/registry_test.go` | AC-4 no two `Apply` calls overlap (if serialisation is the chosen fix) | |
| `TestResponderDoesNotHoldLockAcrossApply` | `internal/plugins/ddos/local/responder_test.go` | evidenced hazard: `r.mu` not held across the kernel reconcile (if that is the chosen fix) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| dispatch response bound (seconds) | define during design | - | - | - |
| nft apply timeout (if introduced) | define during design | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-firewall-concurrency.ci` | `test/plugin/` | firewall block + ddos-local mitigation under flood; assert dispatch-command stays responsive and the drop is installed (needs-linux / QEMU) | |

### Race / Stress
| Check | Command | Status |
|-------|---------|--------|
| Race detector on the concurrent-apply tests | `go test -race ./internal/component/firewall/ ./internal/plugins/ddos/local/` | |

## Files to Modify
- `internal/component/firewall/registry.go` - `ApplyAll` reconcile serialisation / lock discipline (exact change per research)
- `internal/component/firewall/backend.go` - `backendsMu` scope and `loadBackendLocked` interaction, if lock ordering changes
- `internal/plugins/ddos/local/responder.go` - only if research shows the responder must not hold `r.mu` across `applyAll`
- `internal/plugins/firewall/nft/backend_linux.go` - only if the backend must guarantee its own `Apply` safety
- `internal/component/plugin/server/dispatch_registry.go` - only if the dispatch path itself is the blocking layer

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | N/A | no new config surface expected |
| CLI commands/flags | N/A | no new command |
| Functional test for the fix | Yes | `test/plugin/ddos-firewall-concurrency.ci` |
| Doctor check for runtime dependencies | N/A | no new runtime dependency |
| Prometheus counters/metrics | [ ] | consider an apply-latency / contention metric during design (AC-6 observability) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | likely No (bug fix); confirm at design |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` (firewall registry concurrency contract) |
| 13 | Known constraint documented (AC-6 path)? | [ ] | `docs/architecture/core-design.md` + the owning package doc comment |

## Files to Create
- `test/plugin/ddos-firewall-concurrency.ci` (needs-linux; the QEMU reproduction)
- unit tests listed above, in the existing `_test.go` files

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Risks & Assumptions (validate A-1..A-5 first) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 13. /ze-review gate | Review Gate section |
| 14. Present + close | Executive Summary; two-commit closure |

### Implementation Phases
1. **Phase: Reproduce + capture** - drive the flood + `firewall { backend nft }` case under
   QEMU; capture a goroutine dump (SIGQUIT / pprof) during the stall. Confirm or break
   A-1..A-4. No fix until the stacks are in hand.
2. **Phase: Root-cause** - identify the exact blocked path and lock order. Record it in
   Design Insights with `file:line`.
3. **Phase: Fix at the owning layer** - implement per research; keep the registry contract
   tests green.
4. **Phase: Prove** - failing-then-passing reproduction; race detector; the other
   `ApplyAll` callers reviewed (AC-8).
5. **Phase: If irreducible (AC-6)** - document the constraint and make the failure
   observable; re-scope with the user before taking this branch.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | No lost update across owners; kernel converges; no lock held across a kernel call without justification |
| Concurrency | Lock order documented; no cycle; race detector clean |
| Blast radius | Every `ApplyAll` caller considered, not just ddos |
| No workaround | The `.ci` tests assert real behavior; the fix is not a sleep or a test relaxation |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Dispatch stays responsive under flood + firewall block | `bin/ze-test plugin --pattern ddos-firewall-concurrency` |
| Concurrent apply converges | `go test -race -run TestApplyAllConcurrentOwnersConverge ./internal/component/firewall/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Availability | A flood must not be able to wedge the management plane: that is the whole point of this spec |
| Resource exhaustion | Repeated reconciles under flood must not grow unbounded queues |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Cannot reproduce the stall | R-1 fallback: lock-discipline audit; report to user before proceeding |
| Test fails behavior mismatch | Re-read Current Behavior; RESEARCH if the blocked path was wrong |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `internal/plugins/firewall/engine.go:274` was the config-driven apply site (deferrals.md:55) | No such file: `internal/plugins/firewall/` holds only `nft/` and `vpp/` backends. The engine is `internal/component/firewall/engine.go`, with the LoadBackend + RegisterTables + ApplyAll sequence at :316-324 and :372-395 | Spec authoring, 2026-07-15: grepped for the symbols after the path failed to resolve | Citation corrected here; the deferral row's path is wrong |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- `ApplyAll` deliberately drops both locks before `b.Apply` (registry.go:94,111,123). That
  keeps the registry lock off the kernel path but permits two owners inside `Apply` at
  once. Whichever way the deadlock resolves, this is the design tension to settle.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- The root cause is an observed symptom only. Until a goroutine dump exists, every causal
  story in this spec is a hypothesis.

## Implementation Summary
### What Was Implemented
- (fill at completion)
### Bugs Found/Fixed
- (fill at completion)
### Documentation Updates
- (fill at completion)
### Deviations from Plan
- (fill at completion)

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Dispatch no longer hangs under flood + firewall block | functional (QEMU) | `test/plugin/ddos-firewall-concurrency.ci` red before fix, green after |
| Root cause established | trace | goroutine dump / lock trace pasted into Design Insights |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)
- [ ] The parent deferral row (`plan/deferrals.md:55`) is resolved or updated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Race detector clean on the touched packages
- [ ] Goal Validation table filled with concrete evidence

## Open Questions (research before design)

- What were the actual blocked goroutine stacks? Capture a dump during the stall. Without
  this, root cause stays unverified.
- Is `internal/plugins/firewall/nft` `Apply` safe to call concurrently? `ApplyAll` holds no
  lock across it (registry.go:123), so today two owners can enter it at once.
- Should `ApplyAll` serialise reconciles (single reconcile actor / queue) rather than
  letting concurrent callers race into `backend.Apply`? What does that cost mitigation?
- Is holding `r.mu` across `applyAll` (responder.go:88,136) load-bearing, or can the
  responder compute the desired table under the lock and reconcile outside it?
- Does the plugin engine deliver events and `dispatch-command` on the same goroutine? If
  so, any blocking event handler stalls dispatch and the fix may belong there.
- Is there a lock-order cycle between `tableRegistry.mu`, `backendsMu`, `r.mu`, and any
  plugin-engine lock? Enumerate the order each path takes.
- Was the 255s bounded by a timeout, a watchdog, or genuine recovery?
- Do the other `ApplyAll` callers (copp, policyroute, flowspec-firewall, anomaly/shape,
  irr) share the hazard? A fix must cover them, not just ddos.

## Notes
- Authored 2026-07-15 as a skeleton from `plan/deferrals.md:55`. Every `file:line` here was
  read at authoring time. The deferral row's `internal/plugins/firewall/engine.go:274`
  citation did not resolve and is corrected in the Mistake Log.
