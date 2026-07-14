# Spec: rib-arch-4 -- Atomic N-Nexthop Best-Change Event Delivery (Realtime ECMP)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rib-arch-0-umbrella.md` - set context
4. `internal/component/bgp/plugins/rib/bestpath.go` - `SelectMultipath`
5. `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeEntry` / `BestChangeBatch`

## Task

Equal-cost multipath selection exists: `SelectMultipath`
(`internal/component/bgp/plugins/rib/bestpath.go:157`) returns a primary plus siblings and
is called from the best-path pipeline (`internal/component/bgp/plugins/rib/rib_pipeline_best.go:98`).

GAP: the realtime best-change event delivers a **single** best next-hop, not the full
N-nexthop ECMP set atomically. Today `BestChangeEntry.BackupNextHop`
(`internal/component/bgp/plugins/rib/events/events.go:82`) is forwarded "as a DEDICATED
backup next-hop, never an ECMP sibling" -- so FIB consumers receiving best-change events
cannot install an ECMP set from the event alone. Deliver all N equal-cost next-hops
atomically in the best-change event so FIB consumers can program ECMP in realtime.

STALE-ish ANCHOR (verified 2026-07-08): the 2026-07-06 triage said `SelectMultipath` is
"show-path only"; it is now wired into the pipeline (`rib_pipeline_best.go:98`). Re-verify
at design time exactly what the realtime event carries versus what `show` computes, so this
does not re-implement an already-delivered path.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - `SelectMultipath` (:157): primary + equal-cost siblings
  → Constraint: reuse this selection; do not recompute multipath in the event path.
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeEntry` / `BestChangeBatch`
  → Constraint: the event's JSON contract is consumed by forked plugins; adding N next-hops must stay compatible or version explicitly.
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - `SelectMultipath` call site (:98)
  → Constraint: verify whether siblings already reach the event before adding them.

**Key insights:**
- Selection already yields siblings; the gap is carrying them through the realtime event atomically, not a new selection algorithm.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - `SelectMultipath` (:157): `primary, siblings := ...` equal-cost multipath
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - calls `SelectMultipath(candidates, multipathMax, relaxASPath)` (:98)
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeEntry` carries a single best plus `BackupNextHop` (:82) which is "a DEDICATED backup next-hop, never an ECMP sibling"

**Behavior to preserve:**
- Single-best consumers keep working; the `BestChangeBatch` JSON contract for forked plugins; `BackupNextHop` backup semantics (distinct from ECMP).

**Behavior to change:**
- The best-change event carries the full equal-cost next-hop set so FIB consumers can install ECMP atomically from one event.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Best-path recomputation in the RIB pipeline after a route change

### Transformation Path
1. `SelectMultipath` computes primary + equal-cost siblings (`bestpath.go:157`, called at `rib_pipeline_best.go:98`)
2. A `BestChangeEntry` is built for the prefix (`events.go`)
3. `BestChangeBatch` is emitted to consumers (FIB, redistribute, ...)
4. Proposed: the entry carries all N equal-cost next-hops so FIB installs ECMP without a second lookup

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| best-path → event | siblings from `SelectMultipath` populate the `BestChangeEntry` | [ ] |
| event → FIB | FIB consumer installs N next-hops atomically | [ ] |

### Integration Points
- `SelectMultipath` (`bestpath.go:157`) - the sibling source
- `BestChangeEntry` / `BestChangeBatch` (`events.go`) - the event payload
- FIB consumers (`internal/plugins/fib/*`) - the realtime installers

### Architectural Verification
- [ ] No bypassed layers (siblings flow best-path → event → FIB, no side channel)
- [ ] No unintended coupling (FIB stays generic; no per-family ECMP special-case)
- [ ] No duplicated functionality (reuse `SelectMultipath`; do not recompute in the event path)
- [ ] Registration over hardcoding - FIB consumers register; no per-consumer field in a core struct (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The realtime event does not already carry ECMP siblings | `BackupNextHop` is "never an ECMP sibling" (`events.go:82`) | Item already done; close as stale | read the event build path at design | unvalidated |
| A-2 | Adding N next-hops keeps the `BestChangeBatch` JSON contract compatible | `events.go` MarshalJSON is contract-stable | Needs explicit versioning | design review of MarshalJSON | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Non-atomic delivery causes transient FIB blackholes during an ECMP change | traffic loss on multipath churn | deliver the full set in one event; FIB replaces atomically |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Best-path change with N equal-cost paths | → | Loc-RIB Change.ECMP carries the sibling next-hops; sysrib ecmp-paths installs them | `TestCheckBestPathChange_BGPMultipathECMP` + `test/plugin/fib-ecmp-realtime.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Prefix with 3 equal-cost paths becomes best | one best-change event carries all 3 next-hops |
| AC-2 | ECMP set shrinks from 3 to 2 | one atomic event; FIB never transiently holds an incomplete set |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckBestPathChange_BGPMultipathECMP` | `internal/component/bgp/plugins/rib/rib_ecmp_test.go` | the BGP producer runs SelectMultipath and mirrors the sibling next-hops onto Loc-RIB Path.ECMP (incl. same-best refresh) | PASS (RED→GREEN) |
| `TestOnChangeCarriesBestPathECMP` | `internal/core/rib/locrib/locrib_test.go` | siblingNextHops returns a best Path's own ECMP set; insert() dispatches ECMP-membership-only ChangeUpdates | PASS (RED→GREEN) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fib-ecmp-realtime` | `test/plugin/fib-ecmp-realtime.ci` | two equal-cost BGP routes produce a sysrib entry whose ecmp-paths carries both next-hops; withdrawing one shrinks it | PASS (`ze-test bgp plugin fib-ecmp-realtime`) |

## Files to Modify

- `internal/core/rib/locrib/candidate.go` - `Path.ECMP []netip.Addr` carry-through field (design C)
- `internal/core/rib/locrib/manager.go` - `siblingNextHops` returns `best.ECMP` when the best Path carries one
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - `checkBestPathChange` runs `SelectMultipath`, resolves sibling next-hops, mirrors them onto the Loc-RIB Path via a `mirrorToLocRIB` closure used on both the same-best short-circuit and the full best-change path
- Pre-existing repair (surfaced by touching the widely-imported locrib): 6 reactor test mocks gained the `DrainPeerSync` method (added to `ReactorLifecycle` earlier, mocks not updated); `internal/plugins/firewall/nft/host_netns_guard_linux*` errcheck/errorlint

## Implementation Steps

1. **Phase: design** - re-verify what the event carries today (A-1); define the N-nexthop event shape.
2. **Phase: wiring** - failing test asserting N next-hops in the event.
3. **Phase: implement (TDD)** - carry siblings through the event; FIB installs ECMP atomically.
4. **Functional test** - `.ci` proving realtime ECMP.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Best-change event carries N equal-cost next-hops atomically
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Design Verification (2026-07-14)

- **A-1 confirmed BROKEN as a gap:** the realtime producer `checkBestPathChange`
  (`rib_bestchange.go:700`) called `SelectBest`, not `SelectMultipath`, and mirrored ONE
  `locrib.Path` (single next-hop). BGP's `SelectMultipath` siblings reached only the
  `show bgp rib best` display (`item.MultipathPeers`, `rib_pipeline_best.go:122-128`).
- **Decisive architecture fact:** the FIB consumes sysrib's Stream B (built from Loc-RIB
  Changes, `sysrib.go:852` in the default in-process deployment), NOT the BGP best-change
  event. `BestChangeEntry.ECMPNextHops` is `json:"-"` and never reaches the FIB. So the fix
  must populate the Loc-RIB, not the BGP event (design A is dead).
- **Design decision — C over B (approved implicitly by choosing the least-invasive correct
  option):** BGP arbitrates ONE best across peers, so it inserts one Loc-RIB Path per prefix.
  Rather than inserting one Path per next-hop with synthetic Instances (design B — hazardous:
  non-ADD-PATH pathIDs all collide at 0, and it needs hot-path stale-Instance reconciliation),
  attach the equal-cost set to the single Path via `Path.ECMP` and have `siblingNextHops`
  return it directly (design C). No Instance scheme, no reconciliation; the Loc-RIB replaces
  the single Path each change, so a shrinking set just carries a shorter `Path.ECMP`.
- reuses the existing, tested Loc-RIB→sysrib→kernel ECMP machinery (`Change.ECMP` →
  `ecmpNextHops` → `ecmpCollect` → `ECMPPaths` → `buildMultiPath`).

## Implementation Summary

- `locrib.Path` gained `ECMP []netip.Addr` (carry-through, excluded from `key()`/`Equal`);
  `siblingNextHops` returns `best.ECMP` first, else computes intra-source siblings as before.
- `checkBestPathChange` runs `SelectMultipath`, resolves the sibling next-hops via the same
  `bestCandidateNextHopAddr` accessor (before the shard lock, preserving lock order), and
  mirrors them onto the Loc-RIB Path through a `mirrorToLocRIB` closure. The closure is also
  called on the **same-best short-circuit**, which previously suppressed ECMP-membership
  changes (it compares the best, not the sibling set) — the Loc-RIB dedups a true no-op.
- **AC-1 met:** a prefix with 2 equal-cost paths produces one Change carrying both next-hops
  (unit + `.ci`). **AC-2 met:** shrinking the set (withdraw one) emits one atomic ChangeUpdate
  and the sysrib ecmp-paths shrink (`.ci`); the FIB replaces atomically via the existing path.
- **Validation without QEMU:** `show rib` exposes `ecmp-paths`, so the `.ci` verifies the ECMP
  reaches sysrib directly; the kernel `buildMultiPath` install downstream of sysrib's ECMPPaths
  is already covered by the IS-IS/OSPF route-install tests.
- **Pre-existing repair:** touching the widely-imported `locrib` pulled its full reverse-dep
  closure into `ze-lint-changed`, surfacing latent breakage: 6 reactor test mocks missing
  `DrainPeerSync` (added to `ReactorLifecycle` earlier) and 2 firewall lint issues. All fixed.

## Review Gate

Self-review of the diff:
- No arbitration change: `Path.ECMP` is excluded from `key()` and `Equal`, so it never affects
  best-path selection; membership-only changes route through the `ecmpChanged` branch.
- No hot-path regression when multipath is off: `SelectMultipath` returns nil siblings with no
  extra work when `maximum-paths <= 1`; `mirrorToLocRIB` passes a nil ECMP.
- Lock order preserved: sibling next-hops resolved before the shard lock (the accessor takes
  `r.peerMu.RLock`).
Findings: 0 BLOCKER, 0 ISSUE. Note: the ecmp resolution loop calls `bestCandidateNextHopAddr`
per sibling (each takes `r.peerMu.RLock`), only when multipath is on and siblings exist.

## Pre-Commit Verification

Re-verified 2026-07-14:

| Item | Evidence |
|------|----------|
| AC-1 verified | `TestCheckBestPathChange_BGPMultipathECMP` PASS; `.ci` PASS |
| AC-2 verified | `.ci` withdraw-one shrinks ecmp-paths to empty |
| RED captured | disabling the `siblingNextHops` best.ECMP fold fails both unit tests |
| No regression | full locrib/rib/sysrib suites PASS normal and `CGO_ENABLED=1 -race` |
| Structural gates | `ze-lint-changed` 0 issues (after repairing pre-existing DrainPeerSync mocks + firewall lint) |
| Producers read | `rib_bestchange.go:700`, `manager.go:188/244`, `sysrib/ecmp.go:25`, `candidate.go:26` read this session |

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
