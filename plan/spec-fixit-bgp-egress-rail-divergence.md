# Spec: fixit-bgp-egress-rail-divergence

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/learned/1161-bgp-export-filter-applied-twice.md`, `plan/learned/1231-fixit-private-asn-leak.md`, `plan/learned/1245-fixit-bgp-concurrency-races.md`
3. Reproduction captures: `tmp/stress-repro/bgp-plugin-{372,378,394}-*.log`
4. Origin: split out of `plan/spec-fixit-load-dependent-functional-failures.md` (owner decision 2026-07-24 — the forward-rail fix is a spec-sized new primitive, not a redirect).

## Task

Four multi-peer forward tests fail under load with WRONG WIRE BYTES to a peer that establishes
concurrently with an incoming UPDATE being relayed (verified via stress-repro, 32 burners/16 cores):
- `372` remove-private-as-replace-peer: `AS_PATH [65002 64496 65002 64497]` (local AS 65000 rewritten to peer AS 65002).
- `378` rfc7606-relay-one-field: a DUPLICATE announce frame.
- `394` role-otc-egress-filter / `395` role-otc-egress-stamp: a SPURIOUS `WITHDRAWN` / OTC not suppressed.
- `351` redistribute-l2tp-multi-peer-nexthop: multi-peer forward mismatch (same class; not individually reproduced yet).

**Verified root cause — two egress rails disagree.** A route relayed from peer A to peer B reaches B via:
- **Forward rail (correct):** `forwardUpdateCore` (`reactor_api_forward.go:249`) runs `orderedEgressSteps`
  (export policy chain + in-process role/OTC/community filters) on the RECEIVED wire, THEN prepends
  local AS, writes PRE-FILTERED (`forward_pool.go:186`, `forward_rs.go:71`).
- **Replay rail (buggy):** on B's peer-up, adj-rib-in replays the stored RAW route as an announce
  `update hex … add` (`adj_rib_in/rib.go:563-572`, `:736-743`, `formatHexCommand:774-778`). The
  announce builder prepends local AS FIRST, THEN the session write gate runs ONLY `facts.exportFilters`
  on the already-prepended wire (`egress_inject_filter.go:43-91`, esp. `:76`, `:66`), skipping the
  in-process role/OTC filters (`role/register.go:22-31` registers OTC via `filterapi`, not
  `facts.exportFilters`). Wrong order (prepend→filter) + incomplete filter set.
- **372:** replay prepends 65000 → remove-private-as REPLACE rewrites private 65000 → 65002.
- **378:** no export filter → replay bytes == forward bytes → duplicate; amplified by a DOUBLE replay
  trigger (adj-rib-in self-replay + bgp-rs `request bgp adj-rib-in replay`, `rs/server_handlers.go:149-208`).
- **394:** in-process OTC egress step never runs on the replay rail (exact withdraw byte UNVERIFIED).

**Fix (owner decision 2026-07-24): route the peer-up replay through the FORWARD rail** so a relayed
route has ONE egress transform, and dedupe the double trigger. NOT option (b) "make the two rails
identical". Never widen a test to green (these are real product bugs — `no-parking`).

## Required Reading

### Architecture Docs
- [ ] `plan/learned/1161`, `/1231`, `/1245`
  → Constraint: the egress gate (`exportFilterForBody`) exists because of the /1231 private-ASN leak;
     any change must keep originated/injected/redistribute routes egress-filtered.
  → Constraint: egress handling is a property of HOW the route is relayed, not of the write primitive (/1161).
- [ ] `docs/architecture/api/architecture.md`, `docs/architecture/api/process-protocol.md` (new RPC surface)

## Current Behavior (MANDATORY)

**Source files read (during the mechanism investigation, 2026-07-24):**
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` — `forwardUpdateCore` (:249, single-peer capable via explicit `[]*Peer`); `ForwardUpdate` (:177, cache-keyed).
- [ ] `internal/component/bgp/reactor/reactor_api_forward_batch.go` — `ForwardUpdatesDirect` (:53); `resolveSourceInfo` (:202).
- [ ] `internal/component/bgp/reactor/recent_cache.go` — `recentUpdates` is consumer-ack + 5-min valve (:25-67). adj-rib-in routes are NEVER cache-resident.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` — `RawRoute` (:71-77): Family/AttrHex/NHopHex/NLRIHex; source peer is the `ribIn` map key but dropped at replay; `buildReplayCommands` (:750); ingest (:249-330).
- [ ] `internal/component/plugin/types_bgp.go` — `ReactorCacheCoordinator` (:361); `pkg/plugin/sdk/sdk_engine.go:69` (`ForwardCached`); `plugin/server/dispatch_cached.go:24`; `pkg/plugin/rpc/bridge.go:452`.
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` — `replayForPeer` (:149-208), needs a synchronous completion signal for EOR (:212-263).

**Behavior to preserve:**
- Wire correctness of forwarded routes unloaded (the passing case must not change).
- Originated/injected/redistribute routes STILL egress-filtered via `exportFilterForBody` (/1231).
- Standalone adj-rib-in (no bgp-rs) still replays on peer-up.

## Data Flow (MANDATORY)

### Entry Point
adj-rib-in `handleState`/`handleStructuredState` on a peer `isUp` event (`rib.go:563-572`).

### Transformation Path
1. Today: `buildReplayCommands` → `formatHexCommand` (`update hex … add`) → announce rail (prepend→gate).
2. Target: `buildReplayCommands` yields `(sourcePeer, *RawRoute)` → new reactor `RelayStoredRoute` →
   reconstruct received-shape `ReceivedUpdate` (synthesize MP_REACH per family) → `forwardUpdateCore`
   with the single dest peer → one egress transform (filter→prepend), pre-filtered write.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| adj-rib-in plugin ↔ reactor | new typed coordinator call `RelayStoredRoute` (mirrors `ForwardCached`) | [ ] |
| reactor ↔ session write | forward rail writes pre-filtered (no re-gate) | [ ] |

### Integration Points
- `forwardUpdateCore` (`reactor_api_forward.go:249`) — the single egress transform the replay must reuse.
- `ReactorCacheCoordinator` (`types_bgp.go:361`) / DirectBridge (`bridge.go:452`) — where `RelayStoredRoute` plugs in beside `ForwardCached`.
- `exportFilterForBody` (`egress_inject_filter.go:43`) — stays for originated/injected/redistribute; the replay stops using it.

### Architectural Verification
- [ ] One egress transform for relayed routes (no rail divergence)
- [ ] No new communication mechanism (typed coordinator call, not a new pattern — `plugin-design.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `AttrsWire.Packed()` excludes MP_REACH/UNREACH but includes type-3 NEXT_HOP | comment `rib.go:67`; MP/legacy nhop split `rib.go:264` vs `:282` | MP reconstruction step changes | read the wire splitter that populates `AttrsWire` | unvalidated |
| A-2 | `WriteAnnounceUpdate` prepends local AS before the egress gate | inferred from `SendAnnounce` (`session_write.go:509,514`) | order claim wrong | read `WriteAnnounceUpdate` body | unvalidated |
| A-3 | add-path path-id is lost on structured ingest (`rib.go:314-323`) vs legacy (`:861-868`) | two ingest paths differ | add-path replay wrong through forward rail | add-path replay test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | forward-rail reconstruction changes unloaded wire output | existing forward `.ci` reds | mutation-verify: revert switch → OTC missing → red; keep unloaded golden bytes |
| R-2 | new reactor race | `make ze-race-reactor` red, stress-repro red post-fix | -race + stress-repro every reactor change |
| R-3 | 395/351 unreproduced → phantom fix | stress-repro "not reproduced" | reproduce each before claiming its fix |

## Wiring Test (MANDATORY)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| adj-rib-in peer-up replay | → | `RelayStoredRoute` → `forwardUpdateCore` | `.ci` proving OTC/role applied on peer-up replay (mutation-verify) |
| both adj-rib-in + bgp-rs loaded | → | replay-owner dedupe | `.ci`/unit asserting exactly one announce per peer-up |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 372 under stress-repro | AS_PATH `[65000 64496 65002 64497]`; 0 reproductions |
| AC-2 | 378 under stress-repro | each field exactly once; no duplicate; 0 reproductions |
| AC-3 | 394 / 395 under stress-repro | dest sees only the EOR (OTC suppressed); 0 reproductions |
| AC-4 | 351 under stress-repro | multi-peer next-hop forward matches; 0 reproductions |
| AC-5 | both plugins loaded, peer-up | exactly one replay (dedupe); standalone adj-rib-in still replays |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRelayStoredRouteEgress` | `internal/component/bgp/reactor/*_test.go` | reconstructed replay runs the full egress pipeline filter-before-prepend | |
| `TestReplayOwnerDedupe` | `internal/component/bgp/plugins/adj_rib_in/*_test.go` | self-replay gated when rs owns replay | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| 372/378/394/395/351 `.ci` | `test/plugin/` | stress-repro green | |

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_forward.go` (+ `RelayStoredRoute`), `internal/component/plugin/types_bgp.go`
- `pkg/plugin/sdk/sdk_engine.go`, `internal/component/plugin/server/dispatch_cached.go`, `pkg/plugin/rpc/bridge.go` (new method)
- `internal/component/bgp/plugins/adj_rib_in/rib.go`, `rib_commands.go` (source-carrying replay + dedupe flag)
- `internal/component/bgp/plugins/rs/server_handlers.go` (set replay-owner)

## Implementation Steps

1. **Phase 1 (Wiring)** — add `RelayStoredRoute` skeleton + RPC/SDK method + a failing wiring test.
2. **Phase 2** — reconstruct received-shape `ReceivedUpdate` from `RawRoute` (IPv4 first, then per-family MP_REACH synthesis); validate A-1/A-2.
3. **Phase 3** — switch `buildReplayCommands` consumers to the primitive; stress-repro 372/378/394/395.
4. **Phase 4** — replay-owner dedupe (rs owns; adj-rib-in gates self-replay); dedupe test.
5. **Phase 5** — add-path path-id gap (A-3); 351.
6. **Phase 6** — `make ze-verify`, `make ze-race-reactor`, per-test stress-repro; interop where wire-visible.
7. **Phase 7** — independent `/ze-review`; learned summary; close.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| "route via forward rail" = point replay at `ForwardCached` | `ForwardCached`/`recentUpdates` is cache-keyed; adj-rib-in routes never cache-resident | mechanism investigation (`recent_cache.go:25-67`) | needs a NEW `RelayStoredRoute` primitive, not a redirect |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | | | | |
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

## Checklist
### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
### Completion (BLOCKING — before ANY commit)
- [ ] Every AC stress-repro-green
- [ ] `make ze-race-reactor` green
- [ ] `make ze-test` passes
- [ ] Independent review clean
- [ ] Learned summary written
