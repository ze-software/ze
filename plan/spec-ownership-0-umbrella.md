# Spec: ownership-0-umbrella (DESIGN-REVIEW finding #1)

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | umbrella |
| Updated | 2026-07-05 |

## Closed (2026-07-05)

All three children implemented, reviewed (two adversarial passes each), and committed;
`DESIGN-REVIEW.md` finding #1 annotated with the resolved status. Details:
- **P1** RS invariant — `ff686e4a2`, `plan/learned/1063-ownership-1-rs-invariant.md`
- **P2** Coordinator types — `442125776`, `plan/learned/1064-ownership-2-coordinator-types.md`
- **P3** reactor modes — `39d798e66`, `plan/learned/1065-ownership-3-reactor-modes.md`

The already-done production ownership (hub owns Server/EventBus/Engine; BGP is a
config-driven plugin) was preserved, not reverted. This umbrella and its three child
specs are removed on close; the durable record is the three learned summaries above
and the DESIGN-REVIEW finding #1 annotation.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This file (the umbrella)
2. `DESIGN-REVIEW.md` finding #1 (the source)
3. The three child specs: `spec-ownership-1-rs-invariant.md`, `spec-ownership-2-coordinator-types.md`, `spec-ownership-3-reactor-modes.md`
4. `docs/architecture/core-design.md` §1 (documented target architecture)

## Task

Address the still-valid parts of DESIGN-REVIEW.md finding #1 ("Ownership inversion:
the BGP reactor owns the system's nervous system"). Research (2026-07-04, six mapping
passes + direct source verification) found the headline claim **substantially stale**:
the production ownership move is already done, and one of the review's remaining bullets
describes a deliberate feature, not a defect. This umbrella records what is already
resolved (so no future work re-does or reverts it) and decomposes the genuinely-open
remnants into three **independent, non-competing** child specs.

## Verdict (what the research actually found)

- **Production ownership is already correct.** The hub constructs the plugin `Server`
  (`cmd/ze/hub/main.go:441`), treats it as the global `ze.EventBus`
  (`main.go:453`), builds `engine.NewEngine(apiServer, configProvider, pm)`
  (`main.go:502`), and owns the server lifecycle. BGP loads as a config-driven plugin
  and merely *borrows* the hub-owned infra via `SetPluginServerAny`/`SetEventBusAny`
  (`internal/component/bgp/plugin/register.go:144,149`); the hub has zero BGP imports.
  The review read the reactor's `api` field (`reactor.go:232`) but not the wiring.
- **The review's "delete the folder" (RS) bullet is real** — but the reactor-native RS
  forwarding is a deliberate performance path (`plan/learned/663-rs-gap-0-structural-forwarding.md`),
  so the fix is "reconcile ownership without regressing perf," not "move it back."
- **The review's `map[string]any` (Coordinator) bullet is real** and matters most for the
  OSPF/IS-IS multi-protocol future.
- **The review's reactor dual-mode claim is only half true**: the second ownership regime
  is not test-only cruft — it is the self-contained **in-process simulation mode of
  ze-chaos** plus the integration harness. It cannot be deleted; it can be made explicit.

## Already Done — DO NOT REVERT

| Item | Where recorded | Evidence |
|------|----------------|----------|
| Standalone `internal/component/bus/` deleted; the plugin `Server` is the `ze.EventBus` | `learned/324-arch-2-bus`, `425-arch-0` | `main.go:499-502`; `component/bus/` absent |
| Engine supervisor owns EventBus/Config/PluginManager; ordered start/stop | `learned/327-arch-5-engine` | `internal/component/engine/engine.go` |
| Hub creates + owns the plugin Server; reactor borrows it | `learned/421-arch-9`, `533-bgp-boundary-cleanup` | `main.go:441`, `register.go:149` |
| BGP is a config-driven plugin (NOT a `ze.Subsystem`); hub has zero bgp imports | `learned/530`, `533` | `main.go:517,529` (only l2tp/pppoe subsystems) |

**Consequence:** a child spec MUST NOT reintroduce a hub→bgp import, MUST NOT revert BGP
to a `ze.Subsystem`, and MUST NOT collapse the EventDispatcher (BGP data path) into the
EventBus (notifications) — these are settled decisions.

## Child Specs (independent — any order, any subset)

| Child | Problem | State | Priority | Depends |
|-------|---------|-------|----------|---------|
| `spec-ownership-1-rs-invariant.md` | RS "delete the folder" invariant: `reactorForwardRS` owns the hot forwarding path while `plugins/rs/` owns withdrawal/replay/filtered-peer lifecycle; deleting the plugin leaves the hot path alive but breaks lifecycle | genuinely broken | **1 (high)** — correctness-relevant | umbrella |
| `spec-ownership-2-coordinator-types.md` | Replace Coordinator `reactors map[string]any` + `extra map[string]any` bag + `any`-typed `ConfigureEventBus/PluginServer/Metrics` hooks with typed per-protocol interfaces | genuinely present | **2 (med)** — multi-protocol seam | umbrella |
| `spec-ownership-3-reactor-modes.md` | Make production reactor borrow-only and the ze-chaos/integration self-contained reactor an *explicit* constructed mode instead of one inferred from `r.api != nil` | modest clarity | **3 (low)** — production already correct | umbrella |

These do not share code paths: P1 is in `bgp/reactor/forward_rs.go` + `bgp/plugins/rs/`,
P2 is in `plugin/coordinator.go` + `plugin/registry/`, P3 is in `bgp/reactor/reactor.go`
startup + `chaos/inprocess` + integration tests. They can be implemented and closed
independently.

## Shared Constraints (apply to every child)

- No hub→bgp import; BGP infra injection stays via registry hooks / `*Any` setters
  (`learned/533`).
- `pkg/ze/` keeps zero `internal/` imports; `internal/component/plugin/` imports no
  subsystem package (module-tier rule; `make ze-tier-check`, `make ze-plugin-boundary-check`).
- Preserve buffer-first / zero-copy on the BGP hot path (P1 especially).
- Do not weaken tests to pass (`ai/rules/no-workarounds-for-missing-behavior.md`).

## Out of Scope for This Umbrella

Other DESIGN-REVIEW findings (#2 duplication, #3 stringly-typed middle, #4 bus, #5
protocol shape, #6 hot-path safety, #7 SDK boundary, #8 registration) are separate
efforts. Finding #1's already-done pieces (above) are closed and not reopened here.

## Success Criteria (umbrella closes when)

- All three children are either closed (implemented) or explicitly deferred with user
  approval.
- `DESIGN-REVIEW.md` finding #1 is annotated with the corrected status (production
  ownership already resolved; RS + Coordinator + explicit-modes tracked here).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers. -->
- [ ] `DESIGN-REVIEW.md` finding #1 - the source claim being addressed/corrected
  → Constraint: the headline ("reactor owns the nervous system") is stale for production; children fix only the still-valid remnants.
- [ ] `docs/architecture/core-design.md` §1 - documented target: Engine supervises (no BGP knowledge); Server is the shared EventBus
  → Constraint: EventDispatcher (data path) and EventBus (notifications) stay separate.
- [ ] `plan/learned/533-bgp-boundary-cleanup.md`, `plan/learned/421-arch-9-plugin-manager-wiring.md` - the already-done ownership move
  → Constraint: no hub→bgp import; BGP stays a config-driven plugin, not a `ze.Subsystem`.

**Key insights:** production ownership is already correct; the three children fix independent, still-open remnants (RS invariant, typed Coordinator, explicit reactor modes).

## Current Behavior (MANDATORY)

**Source files read:** (verification underpinning the verdict)
- [ ] `cmd/ze/hub/main.go` (441,453,502) - hub creates + owns Server, uses it as EventBus, builds Engine
- [ ] `internal/component/bgp/plugin/register.go` (144,149) - reactor borrows hub-owned infra
- [ ] `internal/component/bgp/reactor/forward_rs.go` (72) - reactor-native RS forwarding (P1)
- [ ] `internal/component/plugin/coordinator.go` (25-31) - `map[string]any` reactors + extra bag (P2)
- [ ] `internal/component/bgp/reactor/reactor.go` (1112) - `externalServer` inferred, standalone mode used by ze-chaos (P3)

**Behavior to preserve:** everything under "Already Done — DO NOT REVERT".

**Behavior to change:** only the three child remnants; this umbrella changes no code directly except a corrective annotation to `DESIGN-REVIEW.md`.

## Data Flow (MANDATORY)

### Entry Point
- DESIGN-REVIEW.md finding #1 (a structural review claim) enters as the work item.

### Transformation Path
1. Verify the claim against production wiring (done — see Verdict).
2. Separate already-done pieces from still-open remnants.
3. Decompose remnants into three independent child specs (P1/P2/P3).
4. Each child runs its own research→design→implement lifecycle and closes independently.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| review claim ↔ code | direct source verification at producing functions | [x] |
| umbrella ↔ child specs | child table with independent Depends | [ ] |

### Integration Points
- `spec-ownership-1-rs-invariant.md`, `spec-ownership-2-coordinator-types.md`, `spec-ownership-3-reactor-modes.md` - the deliverables this umbrella coordinates.

## Wiring Test (MANDATORY — NOT deferrable)

Umbrella coordinates specs; runtime wiring is proven per child. This umbrella's only
concrete artifact is the corrected finding annotation.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| child spec files exist and cover P1/P2/P3 | → | this umbrella's child table | manual review: `ls plan/spec-ownership-{1,2,3}-*.md` |
| DESIGN-REVIEW.md annotated with corrected status | → | finding #1 note | grep DESIGN-REVIEW.md for the correction note |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none at umbrella level) | - | tests live in each child spec | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (per child) | see P1/P2/P3 specs | RS forwarding, coordinator typing, reactor modes | N/A at umbrella |

## Files to Modify
- `DESIGN-REVIEW.md` - annotate finding #1 with corrected status (production already resolved; remnants tracked here)

## Files to Create
- `plan/spec-ownership-1-rs-invariant.md`, `plan/spec-ownership-2-coordinator-types.md`, `plan/spec-ownership-3-reactor-modes.md`

## Implementation Steps

1. Write the three child specs (this session).
2. Implement children in priority order (P1 → P2 → P3), each via `/ze-implement`, each independently closeable.
3. Annotate `DESIGN-REVIEW.md` finding #1 with corrected status.
4. Close the umbrella when all children are closed or explicitly deferred.

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

- [ ] Three child specs written and internally consistent with this umbrella's Already-Done table
- [ ] Each child independently implementable (no cross-child dependency beyond the umbrella)
- [ ] DESIGN-REVIEW.md finding #1 annotated
- [ ] No child reverts an Already-Done item

### Per-child gates (each child spec carries its own; N/A at umbrella level)
- [ ] Tests written — owned by each child spec
- [ ] Tests FAIL (paste output) — in each child during `/ze-implement`
- [ ] Tests PASS (paste output) — in each child during `/ze-implement`
- [ ] `make ze-test` passes (lint + all ze tests) — run per child, not for this coordinating umbrella

## Design Insights

- The review's most consequential finding was ~90% already implemented; only direct
  wiring verification (not field inspection) revealed it. This is a live example of the
  `ai/rules/no-fabrication.md` caller-vs-producer trap: the reactor `api` field exists,
  but production never exercises the self-owning path.
