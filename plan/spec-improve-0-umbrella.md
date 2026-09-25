# Spec: improve-0 -- Comparison-Review Improvements (Umbrella)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. Accepted Findings below, including later additions and recorded closures
4. `ai/rules/writing.md` -- rules for cross-project claims

## Task

An external comparison review of Ze against another open-source routing daemon
produced 9 ranked findings. This umbrella began with 6 accepted findings and
3 declined findings, and also owns the later accepted additions in rows 7-9.

**Scope statement (comparison honesty):** claims below about the other daemon come
from the external review and were NOT independently verified in this session. Ze-side
claims WERE verified against this checkout (main, 2026-07-08); every Ze claim in the
child specs cites the producing function. During each child's design phase, re-verify
any external behavior that shapes a design decision against primary sources directly.

### Accepted Findings -> Child Specs

| Phase | Spec | Adopts | Depends |
|-------|------|--------|---------|
| 1 | `plan/spec-improve-1-nb-transactions.md` (moved 2026-08-29) | Operator-facing config transaction contract: IDs, comments, confirmed commit, list/get/rollback-by-id | - |
| 2 | `spec-improve-2-gnmi-state.md` | Operational-state provider fanout; gNMI Get honors CONFIG/STATE/OPERATIONAL/ALL | - |
| 3 | `spec-improve-3-event-replay` (closed 2026-09-05, `d74f428b47` / `99bbe13ab7`) | Opt-in JSONL protocol event capture and message-driven replay; timer-driven and multi-peer remainder owned by `plan/spec-improve-3-event-replay-deferred-deterministic-scheduler.md` | - |
| 4 | `spec-improve-4-conformance-fixtures.md` | File-driven protocol conformance fixture format, one BGP fixture first | - (capture/replay prerequisite fulfilled) |
| 5 | `plan/immediate/spec-improve-5-panic-boundaries.md` (moved 2026-08-29) | Explicit recover boundaries at network-input task boundaries | - |
| 6 | `spec-improve-6-yang-coverage.md` | YANG coverage report: per-module implemented/owned/constrained node status | - |
| 7 | `spec-improve-7-yang-handler-gate` (CLOSED 2026-08-29) | Handler-completeness gate: every config-schema root claimed by a delivery surface, blocking test + doctor check (added 2026-07-10 after primary-source re-review). Shipped: `claims.Audit` (`internal/component/config/claims/claims.go`), `./le config claims` in both verify-stage populations, `checkConfigClaims` (`internal/component/doctor/checks_config_claims.go`), `test/ui/doctor-config-claims.ci` | - |
| 8 | `spec-improve-8-fuzz-decode-context.md` | Fuzz the negotiated-capability decode space: context args on existing targets + targets for uncovered surfaces (added 2026-07-10) | - |
| 9 | `plan/immediate/spec-config-verify-rejects-unknown-keys.md` | Strict unknown-key rejection at config verify, inherited from child 7 on 2026-08-29. Source read on 2026-09-19: `validateContainerEntry` skips data keys absent from `entry.Dir`; the owner must trace the full verify path and preserve cross-module contributions and `ze:allow-unknown-fields` | - (child 7 closed) |

### Declined Findings

| Review finding | Decision | Reason |
|----------------|----------|--------|
| 3: uniform typed routing-protocol contract (the reviewed daemon's per-protocol trait shape) | Declined as a standalone registration layer | Ze's generic plugin `Registration` (`internal/component/plugin/registry/registry.go`) is load-bearing (see `ai/rules/plugins.md`); protocols in Ze are components plus many small plugins, not per-protocol monoliths. The predictability benefit ("a maintainer knows where config, state, RPC, tests live") is delivered instead by the conformance fixture format (improve-4) and the state-provider registry (improve-2). Revisit only if OSPF/IS-IS maturation shows real drift between protocol implementations |
| 4: protocol breadth (RIP, VRRP, IGMP...) | Declined | The review itself recommends against chasing breadth before contracts. Ze's product center (appliance NOS + BGP + growing IGP work) does not need RIP/VRRP/IGMP now. No spec |
| 9: CI fuzz-target and benchmark buildability checks | Declined | The reviewed daemon is Rust, where fuzz targets are separate crates not built by `cargo test`, so buildability needs its own CI check. In Go, fuzz targets and benchmarks are ordinary `_test.go` functions (e.g. `internal/component/l2tp/avp_fuzz_test.go`, run via `go test -fuzz` in `internal/le/fuzz/actions.go`) compiled by every `./le test unit` run, and `le perf` builds from `cmd/ze` + `internal` (`internal/le/` native action tables), covered by lint/build. The buildability gap the review's CI check closes does not exist in Ze. Arm64 smoke coverage may deserve its own future discussion (gokrazy targets) but is unrelated to this set |

## Required Reading

### Architecture Docs
- [ ] `ai/rules/writing.md` - governs how external claims are cited
  → Constraint: unverified claims about other daemons must be labeled as from the review, not asserted
- [ ] `ai/rules/plugins.md` - why finding 3 was declined
  → Constraint: no new per-feature field, switch, or factory in core/shared packages

### RFC Summaries (MUST for protocol work)
- Not applicable at umbrella level; child specs list their own.

**Key insights:**
- Every accepted adoption fronts EXISTING Ze machinery (transaction coordinator, plugin
  registry, event bus, .ci harness) rather than importing another daemon's shapes wholesale.

## Current Behavior (MANDATORY)

**Source files read:** (ze-side verification of the review's claims)
- [ ] `internal/component/config/transaction/orchestrator.go` - TxCoordinator has txID and verify/apply/commit/rollback phases (`Execute`, :198-234)
- [ ] `internal/component/gnmi/get.go` - Get serves the running config tree only (:21-26)
- [ ] `internal/component/gnmi/set.go` - Set commits without returning a transaction ID (:97-115)
- [ ] `internal/component/plugin/server/event_ring.go` - ring stores timestamp/namespace/type only (:13-17)
- [ ] `internal/component/bgp/reactor/session.go` - Run read loop has no recover boundary (:706-797)
- [ ] `internal/component/plugin/registry/registry.go` - generic plugin Registration (:39-120)
- [ ] `internal/component/config/yang/loader.go` - registry-driven module loading (:20-28)

**Behavior to preserve:** (unless user explicitly said to change)
- All existing surfaces (gNMI Subscribe machinery, CLI rollback by revision, adj-rib-in
  route replay, plugin Registration fields) keep working; adoptions are additive.

**Behavior to change:** (only if user explicitly requested)
- None at umbrella level; children define their own.

## Data Flow (MANDATORY)

### Entry Point
- Umbrella only: work enters through the accepted-item table; each live child
  documents its own entry point.

### Transformation Path
1. Review finding verified against Ze source (this session, citations above).
2. Accepted finding becomes a child spec with its own design phase.
3. Child specs implemented independently in phase order; improve-4 consumes improve-3's capture format.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Review claims ↔ Ze source | Each Ze claim re-read at the producing function | [ ] |
| Umbrella ↔ children | Child specs carry their own data-flow sections | [ ] |

### Integration Points
- Child specs integrate with: config transaction coordinator, plugin registry, gnmi
  server, BGP reactor, .ci functional test harness.

### Architectural Verification
- [ ] No bypassed layers (children front existing machinery)
- [ ] No unintended coupling (children independent except improve-4 -> improve-3)
- [ ] No duplicated functionality (each child extends, does not recreate)
- [ ] Registration over hardcoding -- every new provider/handler registers via the existing registry (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The review's claims about the other daemon are accurate | External review text | An adoption may copy a capability the other daemon does not actually have; design would chase a phantom | Spot-read primary sources for the findings that shape each child design | validated 2026-07-10 for children 3, 4, 6, 7, 8: primary sources read at the daemon's checkout (event recorder `holo-protocol/src/event_recorder.rs:30-65` + replay `holo-tools/holo-replay/src/main.rs:17-32`; conformance harness `holo-protocol/src/test/stub/mod.rs:320-429`; coverage tool `holo-tools/src/bin/yang_coverage.rs:65-150`; startup callback-completeness abort `holo-daemon/src/northbound/core.rs:815-849`; fuzz decode-context `fuzz/fuzz_targets/bgp/message_decode.rs:7-12`). Children 1, 2, 5 still validate at their own design phase |
| A-2 | The accepted adoptions can be implemented separately | Child scoping in this file | Hidden coupling forces re-ordering | Design phase of each child re-checks Depends | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Adopting operator-contract features (improve-1, improve-2) grows the API surface before RBAC/hardening work completes | Overlap found with `spec-managed-server-hardening.md` during design | Design phases cross-check security specs; gate new RPCs behind existing auth |
| R-2 | Accepted additions remain unscheduled when only the original six children count towards completion | An accepted row has neither a live owner nor closure evidence | AC-1 and the Goal Gates account for every accepted row, including row 9 |

## Wiring Test (MANDATORY)

Umbrella: no feature code of its own; wiring lives in child specs.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Child spec implementations | → | see each child's Wiring Test table | all live accepted-item owners |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Every row in Accepted Findings, including later additions | Each outstanding item has a live owning spec that names this umbrella and passes spec validation; a closed item has a dated closure reference. Row 9 cannot be satisfied by an ownerless table entry |
| AC-2 | A child spec closes | The accepted-item table records its closure evidence and names the live owner of any remainder |

## End-to-End User Stories (MANDATORY for new features)

Umbrella only: user stories live in child specs.

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator follows the adoption set | accepted-item table and live child specs | child spec test plans |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (umbrella: none) | child specs | each child carries its own plan | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A -- umbrella; functional tests live in child specs | - | - | |

## Files to Modify
- `internal/component/config/transaction/` - via spec-improve-1 (transaction records)
- `internal/component/gnmi/` - via spec-improve-2 (Get DataType handling)
- `internal/component/bgp/reactor/` - via spec-improve-3 (capture) and spec-improve-5 (recover boundary)

## Files to Create
- Child specs create their own files; see each spec.

## Implementation Steps

1. Design + implement children in phase order (improve-1 first: highest operator value).
2. improve-3 before improve-4 (fixtures consume the capture format).
3. Update this umbrella's child table as each closes.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every accepted item has a schedulable live owner or dated closure evidence; no remainder is dropped from the completion count |
| Registration over hardcoding | Children register providers/handlers via the existing registry; no new core switch/factory (`ai/rules/plugins.md`) |
| Comparison honesty | No unverified external claim asserted as fact in any child |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- Go's in-package `_test.go` fuzz/bench model made review finding 9 a no-op for Ze;
  language-level differences must be checked before copying CI shapes across ecosystems.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Every accepted item, including rows 7-9, has met its owning spec's gates and has closure evidence; any surviving remainder has a live owner and prevents umbrella completion
- [ ] `./le verify worktree` passes after each child lands

### TDD
- [ ] Tests written (per child)
- [ ] Tests FAIL (per child)
- [ ] Tests PASS (per child)

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `improve-7-yang-handler-gate.md`, 2026-07-10

Deferred by spec-improve-7-yang-handler-gate (Known Limitations).

Strict unknown-key rejection at config verify (reject config keys absent from
the schema). The 2026-09-19 source reading confirms the permissive branch in
`validateContainerEntry`. The live owner is
`plan/immediate/spec-config-verify-rejects-unknown-keys.md`; its design must trace
the full verify path and preserve schema exceptions and cross-module contributions.
