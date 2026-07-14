# Spec: rib-arch-0 -- BGP Engine / RIB Architecture Follow-ups (Umbrella)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. Child specs: `spec-rib-arch-1-*` through `spec-rib-arch-8-*`
4. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Umbrella index for deep BGP engine / RIB architecture follow-ups. The items are
distinct but low-priority; grouped so the intent survives. `rib-arch-1` (protorib
central store) in particular is a design decision, not a mechanical task.

History: this set began as the consolidation skeleton `spec-followup-bgp-rib-arch`
(backlog triage 2026-07-06, verified deferral survivors). On 2026-07-08 it was split
into one child spec per work item under the `spec-rib-arch-*` prefix, per the spec-set
convention in `ai/rules/planning.md` ("Spec Sets"). Each child is a `skeleton`
(captured intent, not a designed spec); select and design children individually when
picked up. **Point selection at this umbrella; implement children one at a time.**

### Split verification (2026-07-08)

Each item's producing `file:line` was re-checked against the codebase during the split.
Two triage anchors had drifted and are flagged for design-time re-verification in the
relevant child spec:

- **rib-arch-3**: the triage cited `PackContext.ExtendedNextHop`; the `PackContext`
  type no longer exists anywhere in the tree. The live inject entry point is
  `injectRoute` (`internal/component/bgp/plugins/rib/rib_commands.go:225`), which today
  only accepts simple prefix families and validates IPv6 next-hops per rfc8950
  (`rib_commands.go:4`).
- **rib-arch-4**: the triage said `SelectMultipath` is "show-path only"; it is now
  called from the best-path pipeline (`internal/component/bgp/plugins/rib/rib_pipeline_best.go:98`,
  `bestpath.go:157`), so whether the realtime event already carries ECMP siblings must
  be re-verified before designing (today `BestChangeEntry.BackupNextHop` is a dedicated
  backup, "never an ECMP sibling" -- `rib/events/events.go:82`).

### Re-verification (2026-07-14)

Full anchor re-check against live code, 6 days after the split. Anchors proved durable
(only rib-arch-8 drifted, +2 lines). Material findings that change scope, not just line
numbers:

- **rib-arch-1**: the "second consumer" trigger has ALREADY fired. flowexport
  (`internal/plugins/flowexport/enrichbgp.go:107`) and forked sysrib
  (`internal/component/sysrib/sysrib.go:889`) both subscribe the `BestChangeBatch` delta
  and re-implement accumulation + full-table replay. The design question is now
  actionable, not speculative.
- **rib-arch-4**: LARGELY SUPERSEDED. Atomic N-nexthop ECMP already reaches the FIB via
  `sysribevents.BestChangeEntry.ECMPPaths`
  (`internal/component/sysrib/events/events.go:62`, on-wire). Only BGP-specific
  `SelectMultipath` multipath (today `show`-only) remains; re-scoped in the child.
- **rib-arch-5**: assumption A-2 is FALSE. `BestChangeEntry` is lossy (no ORIGIN /
  LOCAL_PREF / communities / aggregator / unknown transitives), so a faithful RFC 9069
  Route-Monitoring UPDATE needs a richer event or a RIB-attribute lookup.
- **rib-arch-7**: the 2026-07-10 Root Cause Finding still holds (`CommandContext.Meta`
  has two write sites and zero readers); the preserved WIP fixture under `tmp/scratch/`
  has been deleted and must be reconstructed from the child spec body.
- **rib-arch-3 / -6**: assumptions A-1 are now VALIDATED (reusable extended-next-hop
  encoder; sufficient Forward-handle AddRef/Release/Bytes API) -- both de-risked.

Cross-cutting theme: several children (rib-arch-1 / -4 / -5) hinge on what the BGP RIB
plugin's `BestChangeEntry` carries. It is deliberately lossy; a shared decision on
whether to enrich it would inform all three.

### Child Specs

| Spec | Item (pre-split `L#` = row in pre-triage `plan/deferrals.md`) | Verified anchor (2026-07-08) | Nature |
|------|--------------------------------------------------------------|------------------------------|--------|
| `spec-rib-arch-1-protorib-store.md` | protorib-0 central RIB store (L221): engine-owned per-protocol store vs event-bus delta model | `redistribute/producer.go:40` `EmitBestChange`; `rib/events/events.go:90` `BestChangeBatch`; learned 634/685 | Design question |
| `spec-rib-arch-2-plugin-ipc-raw-bytes.md` | plugin-ipc-raw-bytes (L184): length-prefixed raw-bytes filter IPC vs JSON string | `pkg/plugin/rpc/types.go:182` `FilterUpdateInput.Update string` (hex `Raw` opt-in at :183) | Refactor (SDK contract) |
| `spec-rib-arch-3-inject-rfc5549.md` | rib-inject RFC 5549 (L58): extended next-hop for injected routes | `rib/rib_commands.go:225` `injectRoute`; parse-side `attribute/mpnlri.go` | Feature |
| `spec-rib-arch-4-fib-ecmp-realtime.md` | fib-ecmp realtime best-change (L122): atomic N-nexthop event delivery | `rib/bestpath.go:157` `SelectMultipath`; `rib/rib_pipeline_best.go:98`; `rib/events/events.go:82` | Feature |
| `spec-rib-arch-5-bmp-locrib.md` | bmp-6 Loc-RIB monitoring (L231): RFC 9069 PeerType=3 from BestChangeBatch | `plugins/bmp/`; `rib/events/events.go:90` | Feature (RFC 9069) |
| `spec-rib-arch-6-rs-fastpath-consumer.md` | rs-fastpath state-tracker consumer (L228): first production `locrib.OnChange` subscriber | `rib/forward_observer.go:37` `observeForwardHandles` (nil-check logger today); learned 784 | Feature |
| `spec-rib-arch-7-llgr-multipeer-ci.md` | llgr-readvertisement multi-peer `.ci` (L68): multi-peer partial-deployment fixture | existing `test/plugin/llgr-*.ci` (single-peer); `plugins/gr/gr_egress.go:57` | Test gap |
| `spec-rib-arch-8-nlri-rewrite.md` | NLRI structural rewrite via ModAccumulator (L60): general NLRI-byte rewrite field | `filterapi/filterapi.go:98` `ModAccumulator` (`SetWithdraw` at :151; no NLRI rewrite) | Feature |

All children have `Depends | -` (independent); design and implement in any order.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/plugins/rib/` - RIB pipeline, best-path, events, forward handles
  → Constraint: verify each child's `file:line` evidence against this source before designing.
- [ ] `pkg/plugin/rpc/types.go` - filter IPC types (`FilterUpdateInput.Update`)
  → Constraint: verify current behaviour against this source before designing (rib-arch-2).

**Key insights:**
- The items are independent; this umbrella only indexes them. Real design happens per child.

## Current Behavior (MANDATORY)

**Source files read:** (re-read per child at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeBatch` (:90) is the RIB's delta event; `BestChangeEntry.BackupNextHop` (:82) is a dedicated backup, "never an ECMP sibling"
- [ ] `internal/component/bgp/redistribute/producer.go` - `EmitBestChange` (:40) publishes deltas on the event bus
- [ ] `pkg/plugin/rpc/types.go` - `FilterUpdateInput.Update string` (:182) carries text attributes/NLRI as JSON; hex `Raw` (:183) is opt-in
- [ ] `internal/component/bgp/plugins/rib/forward_observer.go` - `observeForwardHandles` registers a nil-check `loc.OnChange` subscriber (:37) that does not read `Change.Forward` bytes

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces
  named in each child's Task section.

**Behavior to change:**
- Only the specific gap enumerated in each selected child spec.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Received UPDATE / best-path change events; filter IPC; RIB inject command

### Transformation Path
1. A route change or injected route enters the RIB
2. Best-path / forward / redistribute machinery processes it
3. Consumers (FIB, BMP, filters, redistribute) receive the result via `BestChangeBatch` deltas or filter IPC

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| engine → plugin | filter IPC (JSON today, raw-bytes proposed in rib-arch-2) | [ ] |
| RIB → consumers | event-bus delta (`BestChangeBatch`) vs proposed central store (rib-arch-1) | [ ] |

### Integration Points
- `internal/component/bgp/plugins/rib/` - RIB pipeline, events, forward handles
- `internal/component/bgp/plugins/bmp/` - BMP monitoring (rib-arch-5)
- `pkg/plugin/rpc/` - filter IPC (rib-arch-2)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each child's verified `file:line` anchor still holds when the child is designed | 2026-07-08 split verification | Re-scope the child | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A child's anchor drifts further before it is picked up (rib-arch-3/-4 already drifted once) | grep at design time finds a renamed/removed symbol | Re-verify against live code; update the child's Task before designing |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Inject a route needing extended next-hop | → | RFC 5549 next-hop encoded (rib-arch-3) | (fill during design) |
| Multipath best-path change | → | N nexthops delivered atomically (rib-arch-4) | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (per-child; each child defines its own testable ACs when it moves to `design`) | (define per child at design time) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define per child at design time) | (per child) | per child Task item | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A at umbrella level - each child carries its own `.ci` (rib-arch-3/-5/-7) or opts out (internal items) | per child | engine/RIB behaviour end-to-end | |

## Files to Modify

- `internal/component/bgp/plugins/rib/events/events.go` - see child specs (rib-arch-1/-4/-5)
- `pkg/plugin/rpc/types.go` - see rib-arch-2
- `internal/component/bgp/plugins/bmp/bmp.go` - see rib-arch-5

## Implementation Steps

1. **Phase: select** - pick one child spec; move it `skeleton` → `design`.
2. **Phase: design** - re-verify the child's `file:line` evidence and fill its Data Flow / Wiring / AC sections.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per the selected child.
5. **Full verification** - `make ze-verify`.
6. **Complete child** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every selected child has feature code + test
- [ ] Wiring Test table complete in the selected child (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/deferral-tracking.md`). Each child moves to `design` when someone picks it up.
- Siblings: `spec-rib-arch-1-protorib-store.md`, `spec-rib-arch-2-plugin-ipc-raw-bytes.md`, `spec-rib-arch-3-inject-rfc5549.md`, `spec-rib-arch-4-fib-ecmp-realtime.md`, `spec-rib-arch-5-bmp-locrib.md`, `spec-rib-arch-6-rs-fastpath-consumer.md`, `spec-rib-arch-7-llgr-multipeer-ci.md`, `spec-rib-arch-8-nlri-rewrite.md`.
