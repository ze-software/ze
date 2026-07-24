# Spec: fixit-bgp-egress-rail-divergence

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/7 |
| Updated | 2026-07-24 |

> **Concurrency note (2026-07-24).** Two sessions worked this spec at once. Phase 1
> (RPC/SDK/coordinator plumbing) and Phase 2 (`relay_payload.go` byte builders) were
> built in parallel by different sessions and are complementary, not duplicated — the
> join point is `buildRelayUpdate`. Check `git log` and the working tree before
> starting any phase here.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/learned/1161-bgp-export-filter-applied-twice.md`, `plan/learned/1231-fixit-private-asn-leak.md`, `plan/learned/1245-fixit-bgp-concurrency-races.md`
3. Reproduction captures: `tmp/stress-repro/bgp-plugin-{372,378,394}-*.log`
4. Origin: split out of the load-dependent-functional-failures fixit, closed as `plan/learned/1270-fixit-load-dependent-functional-failures.md` (owner decision 2026-07-24 — the forward-rail fix is a spec-sized new primitive, not a redirect).

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
| A-1 | `AttrsWire.Packed()` excludes MP_REACH/UNREACH but includes type-3 NEXT_HOP | comment `rib.go:67`; MP/legacy nhop split `rib.go:264` vs `:282` | MP reconstruction step changes | read the wire splitter that populates `AttrsWire` | **BROKEN** |
| A-2 | the replay rail prepends local AS before the egress gate | inferred from `SendAnnounce` (`session_write.go:509,514`) | order claim wrong | read the announce builder body | **confirmed** |
| A-3 | add-path path-id is lost on structured ingest (`rib.go:314-323`) vs legacy (`:861-868`) | two ingest paths differ | add-path replay wrong through forward rail | add-path replay test | unvalidated |
| A-4 | on peer-up BOTH adj-rib-in self-replay and bgp-rs `replayForPeer` fire; only rs gates the concurrent forward (`Replaying`) | `rib.go:563-572` + `rs/server_handlers.go:149-208` | 378 needs a different dedupe | read both peer-up handlers | **confirmed** |

**A-1 BROKEN — evidence.** `RawMessage.AttrsWire` is assigned `wireUpdate.Attrs()`
(`internal/component/bgp/reactor/reactor_notify.go:331,345`), and `WireUpdate.Attrs()`
builds it over `u.sections.Attrs(u.payload)` — the WHOLE path-attribute section
(`internal/component/bgp/wireu/wire_update.go:106`). `Packed()` returns those bytes
verbatim (`internal/core/bgp/attribute/wire.go:52-54`). So a stored `AttrHex` for an
MP family CONTAINS the entire MP_REACH_NLRI attribute, including every NLRI of the
originating UPDATE — not just this route's. The `rib.go:67` doc comment asserting
otherwise is its author's belief, not a producer fact (`no-fabrication.md`).
→ Constraint: the reconstruction MUST strip attribute types 14/15 from the stored
   block and re-synthesize a single-NLRI MP_REACH, otherwise a replayed MP route
   emits a duplicate MP_REACH and re-announces the whole original NLRI set.
→ Constraint: attribute ORDER of the surviving attributes must be preserved —
   the `.ci` expectations are exact hex, and a real forward preserves source order.

**A-2 confirmed — evidence.** The replay rail is `update hex … add` →
`handleUpdateWire`/`DispatchNLRIGroups`
(`internal/component/bgp/plugins/cmd/update/update_wire.go:391-403`,
`update_text.go:767-798`) → `AnnounceNLRIBatch`
(`internal/component/bgp/reactor/reactor_api_batch.go:33`), whose AS_PATH builder
prepends local AS for a non-RS-client eBGP peer
(`buildBatchASPath` :317-345, `packedWithLocalASPrepended` :385) BEFORE
`sendUpdateWithSplit` → `writeUpdateGated(update, gate=true)`
(`session_write.go:246,268-289`) runs `exportFilterForBody`
(`egress_inject_filter.go:43-91`). Prepend-then-filter, and the gate runs ONLY
`facts.exportFilters` (`egress_inject_filter.go:50,76`), never the in-process
`orderedEgressSteps` that carry role/OTC (`role/register.go:22-31` registers via
`filterapi.Register`, i.e. the forward rail's `orderedEgressSteps` only).

**A-4 confirmed — evidence.** 378's duplicate is NOT two replays: it is one replay
plus one forward. `bgp-rs` marks a peer `Replaying` and excludes it from forward
targets until replay completes (`rs/server_handlers.go:157-162`), so the rs replay
is already race-free. adj-rib-in's OWN peer-up self-replay
(`rib.go:563-572`, `:732-744`) has no such gate, so under load it emits the route a
second time alongside the reactor forward.
→ Constraint: the dedupe is "rs owns replay, adj-rib-in stands down", not
   "collapse two replays into one".

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

1. **Phase 1 (Wiring)** — DONE (`83cf7da80`). `ze-plugin-engine:relay-stored-route` on both
   transports from one `engineOp` entry (JSON + typed DirectBridge slot); `rpc.StoredRoute` /
   `RelayStoredRouteInput`; `Plugin.RelayStoredRoute` SDK method; narrow
   `plugin.ReactorRelayCoordinator`; `reactorAPIAdapter.RelayStoredRoute`
   (`reactor_api_relay.go`) resolving destination + per-source `forwardSourceInfo` and calling
   `forwardUpdateCore`. `StoredRoute` carries the SOURCE peer — the field the old
   `update hex … add` replay dropped, and the reason the rails could diverge.
   5 wiring tests in `dispatch_relay_test.go`, mutation-verified (short-circuiting the
   dispatch turns 3 red). `TestPluginRPCRegistryCoversAllPaths` updated deliberately.
2. **Phase 2** — reconstruct received-shape `ReceivedUpdate` from `RawRoute` (IPv4 first, then
   per-family MP_REACH synthesis); validate A-1/A-2.
   → IN FLIGHT IN A CONCURRENT SESSION: `internal/component/bgp/reactor/relay_payload.go`
     (+ `relay_payload_test.go`) already provides the byte-level builders — `scanAttrBlock`,
     `isRelayStrippedAttr`, `relayNeedsNextHopAttr`, `writeMPReach`, `relayPayloadLen`,
     `writeRelayPayload` — buffer-first, honouring the A-1 constraint (strip types 14/15,
     re-synthesize a single-NLRI MP_REACH, preserve surviving attribute order).
   → REMAINING GLUE: `reactorAPIAdapter.buildRelayUpdate` (`reactor_api_relay.go`) is the
     seam. It currently returns `errRelayReconstruct`; it must take a pooled buffer, call
     `writeRelayPayload`, and wrap the result as a `*ReceivedUpdate`. Do NOT re-derive the
     payload builders — they exist.
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
| A-1: stored `AttrHex` excludes MP_REACH/UNREACH | it is the FULL attribute section, MP attributes included (`reactor_notify.go:345` → `wire_update.go:106` → `attribute/wire.go:52`) | read the producer chain during the audit, before any code | reconstruction gains a mandatory strip-14/15 step; without it an MP replay duplicates MP_REACH and re-announces the source UPDATE's whole NLRI set |
| 378's duplicate is two replays racing | it is ONE replay plus the reactor forward; rs already gates its own replay with `Replaying` | read both peer-up handlers (`rib.go:563`, `rs/server_handlers.go:157`) | dedupe is "rs owns replay, adj-rib-in stands down", not "merge two replays" |

## Critical Review Checklist

| # | What to verify | Method |
|---|----------------|--------|
| C-1 | Reconstructed payload is byte-identical to the received wire for IPv4 unicast (attribute order preserved, no re-ordered NEXT_HOP) | golden-byte unit test over a real received frame |
| C-2 | MP families: exactly one MP_REACH in the output, carrying only this route's NLRI | unit test with a 3-NLRI MP_REACH source frame |
| C-3 | Pool buffer returned exactly once — no leak, no double-return | buffer-accounting unit test; `make ze-race-reactor` |
| C-4 | Source peer gone/not-established fails CLOSED (no unfiltered send) | unit test with an unknown source address |
| C-5 | Originated/injected/redistribute routes STILL run `exportFilterForBody` (/1231 private-ASN leak stays fixed) | existing egress_inject_filter tests + private-asn `.ci` still green |
| C-6 | The relay path never re-enters the write gate (no double filtering, /1161) | `writeUpdatePreFiltered` is the only write reached; assert via the 372 golden bytes |
| C-7 | Standalone adj-rib-in (no bgp-rs) still replays on peer-up | `.ci` without the rs plugin |
| C-8 | No new communication mechanism: typed coordinator call mirroring `ForwardCached`, JSON RPC fallback for forked plugins | code read against `ai/rules/plugin-design.md` DirectBridge section |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `RelayStoredRoutes` reactor primitive | `grep -n "func (a \*reactorAPIAdapter) RelayStoredRoutes"` + unit tests |
| Coordinator / RPC / SDK / bridge plumbing | `grep -n RelayStored` across `internal/component/plugin`, `pkg/plugin` |
| adj-rib-in replay uses the relay rail | `grep -n formatHexCommand` returns no live replay caller |
| Replay-owner dedupe | unit test + `.ci` asserting one announce per peer-up |
| 372/378/394/395/351 green under stress | `scripts/dev/stress-repro.py bgp plugin` per test, 0 reproductions |
| Mutation-verified gates | revert the relay switch → the new `.ci` goes red |

## Security Review Checklist

| # | Concern | Check |
|---|---------|-------|
| S-1 | Attacker-controlled attribute bytes drive the reconstruction walker | bounds-check every header read; malformed block → reject the route, never a partial/OOB write |
| S-2 | Length fields (attr len, MP_REACH len, message len) can overflow 16-bit or the buffer | explicit range checks before backfill; oversize → reject |
| S-3 | Unbounded allocation from a large replay batch | batch is bounded by stored RIB size; buffers come from the bounded read pool, exhaustion → reject not grow |
| S-4 | Fail-open on missing source peer would send an unfiltered route | fail closed (C-4) |
| S-5 | Hex decode of untrusted plugin input | `hex.DecodeString` errors reject the route with a named error |

## Documentation Update Checklist

| # | Category | Applies | File / action |
|---|----------|---------|---------------|
| D-1 | Feature list | No | no new user-facing feature; a wire-correctness fix |
| D-2 | User guide | No | no config or CLI surface change |
| D-3 | Config syntax | No | no YANG change |
| D-4 | CLI reference | No | no new command (relay is a plugin-engine RPC, not a CLI verb) |
| D-5 | API/RPC docs | **Yes** | `docs/architecture/api/process-protocol.md` — add the relay-stored-routes engine RPC |
| D-6 | Plugin SDK | **Yes** | `docs/plugin-development/` — the new SDK call, if the SDK surface is documented there |
| D-7 | Wire format | No | no encoding change; the relay reproduces the received shape |
| D-8 | RFC compliance | **Yes (verify)** | `docs/features/rfc-status.md` — RFC 9234 OTC egress now applies on the replay path too |
| D-9 | Architecture | **Yes** | `docs/architecture/api/architecture.md` — the egress-rail note in `egress_inject_filter.go` header must stop claiming replay goes through that gate |
| D-10 | Test infrastructure | No | no new runner or format |

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

## Verification Evidence (2026-07-24, session 94f6df1c)

### Acceptance Criteria

| AC | Status | Fresh evidence |
|----|--------|----------------|
| AC-1 (372 AS_PATH) | **MET** | `ze-test bgp plugin --pattern remove-private-as-replace-peer` -> `pass 1/1`; `stress-repro.py bgp --test "plugin 372" --iterations 12 --any-failure` -> "not reproduced in 12 invocation(s) under load" (`tmp/stress-repro/bgp-plugin-372-20260724-233009.log`). Previously `AS_PATH [65002 ...]` (`bgp-plugin-372-20260724-183402.log`). |
| AC-2 (378 duplicate) | **MET** | `pass 1/1`; stress-repro 10 invocations, not reproduced (`bgp-plugin-378-20260724-233020.log`). |
| AC-3 (394/395 OTC) | **MET** | both `pass 1/1`; 394 stress-repro 10 invocations, not reproduced (`bgp-plugin-394-20260724-233021.log`). |
| AC-4 (351 multi-peer nexthop) | **MET, but NOT attributable to this spec** | `ze-test bgp plugin --pattern redistribute-l2tp-multi-peer-nexthop` -> `pass 1/1`; `stress-repro.py bgp --test "plugin 351" --iterations 12 --any-failure` -> "not reproduced in 12 invocation(s) under load" (`tmp/stress-repro/bgp-plugin-351-20260725-000715.log`). **351 does not exercise the replay rail at all**: it launches `ze -` with no `--plugin` flags (`test/plugin/redistribute-l2tp-multi-peer-nexthop.ci:204`) and its `plugin {}` block loads only `l2tp-nexthop-test`, `redistribute-orchestrator` and `fakel2tp` (`:121-131`) -- neither `bgp-adj-rib-in` nor `bgp-rs` is present, so no peer-up replay exists in that test and `RelayStoredRoute` is never reached. Its load-dependent failure was the RFC 6286 duplicate BGP Identifier race, fixed by commit `e4076920c` (which gave the second ze-peer a distinct identifier, `+option=open:value=router-id:id=1.2.3.6`). 351 was mis-triaged into this spec's cluster; the egress-rail change neither caused nor fixed it. |
| AC-5 (dedupe) | **MET** | `TestReplayOwnerDedupe` (both directions: standalone still replays, owned stands down). Mutation-verified: removing `!r.replayOwned.Load()` turns it RED. |

### Gates

| Gate | Result |
|------|--------|
| `make ze-race-reactor` (`-race -count=20`) | **clean** -- 0 data races, `ok ... reactor 118.140s` |
| `go vet ./internal/... ./pkg/...` | clean (only the pre-existing, deliberate `textbuf.go:161` noescape warning) |
| `make ze-test-bgp` | green after dropping the OTC fallback (see Deviations) |
| `make ze-test-plugins` | only `TestISISLSDBSync` red -- IS-IS LSDB, unrelated subsystem, untouched by this spec |
| `ze-test bgp plugin --all` (495 tests) | 470 pass. **Zero** `relay-stored-route` errors anywhere. 3 failures (223/254/351) carry the RFC 6286 subcode-3 signature; the remaining 22 predate this work or belong to other in-flight sessions and need a clean-tree baseline to attribute. |

### Deviations

| What | Why |
|------|-----|
| Dropped the `OTCEgressFilter` `src-role` config fallback from this spec | Not required by any AC: removing it and re-running 394/395 keeps both green, because ingress stamps OTC into the WIRE bytes before storage and `checkOTCEgress` sees it on the reconstruction. Landing it would require editing `TestOTCEgressNoStampProvider`, which carries `RFC requirement: RFC9234-5-4 negative` and needs explicit user approval. Homed in `plan/deferrals/fixit-bgp-egress-rail-divergence.md` -> `plan/spec-fixit-otc-src-role-meta-fallback.md`. |
| `plugin.Coordinator` gained `RelayStoredRoute`, and `ReactorLifecycle` now composes `ReactorRelayCoordinator` | Phase 1 left the Coordinator facade without the method, so `s.reactor.(plugin.ReactorRelayCoordinator)` failed at RUNTIME and every replay degraded to `relay-stored-route: no reactor available` (observed in the first 372 run). Composing the interface makes that a COMPILE error; doing so immediately surfaced 8 test mocks silently missing the method. |

### Remaining before closure

- [ ] `make ze-verify`
- [ ] Independent review gate (0 BLOCKER / 0 ISSUE)
- [ ] Learned summary + two-commit closure

## Review Gate -- Run 2 (after fixes)

Independent reviewers again (2, distinct lenses: fix-correctness; regression risk + test
integrity). Fixes applied for every BLOCKER and ISSUE from Run 1:

| Run 1 finding | Fix | Evidence |
|---------------|-----|----------|
| BLOCKER 1 -- add-path NLRI framing corrupts the peer session | `errRelayAddPath` refuses before any buffer is taken (`reactor_api_relay.go` `buildRelayUpdate`) | `TestRelayStoredRouteRefusesAddPathSource` |
| BLOCKER 2 -- `updateRoute` dead code, lint gate red | deleted | `make ze-lint-changed` exit 0 |
| ISSUE 3 -- relay error swallowed, bgp-rs told the RIB is complete | `relayRoutes` returns error; `replayCommand` returns `statusError` | `TestAdjRibInReplayArgsPassthrough` (which was passing on a swallowed error and now needs a real relayer) |
| ISSUE 4 -- partial relay reported as success | `errRelayIncomplete` when `relayed < eligible` | -- |
| ISSUE 5 -- ownership claim missed the FIRST peer | new hidden `request bgp adj-rib-in claim-replay`, claimed by bgp-rs from `OnAllPluginsReady` before any session establishes | `TestReplayOwnerDedupe/claim precedes the first peer-up`; mutation-verified (claim not setting the flag turns it red) |
| ISSUE 6 -- operator verb permanently latched ownership | plain `replay` no longer latches | `TestReplayOwnerDedupe/operator replay does not latch ownership` |
| ISSUE 7 -- 16 MB IPC frame ceiling for a forked plugin | `relayChunkSize` = 4096 routes per call | -- |
| ISSUE 8 -- RFC 5549 next hop emitted as a 16-byte legacy NEXT_HOP | `errRelayNextHopLen` | `TestRelayStoredRouteRejectsMalformedInput/ipv4 next-hop not 4 bytes` |
| NOTE 10 -- body bounded by the attribute limit | `maxUpdateBodyLen` = `message.ExtMsgLen - message.HeaderLen` | boundary test updated to the real last-valid/first-invalid pair |
| NOTE 11 -- stray legacy NEXT_HOP on MP reconstructions | `isRelayStrippedAttr` also strips type 3 when `fam != IPv4Unicast` | -- |
| NOTE 14 -- build-time Release not deferred | deferred inside a closure around `forwardUpdateCore` | -- |
| NOTE 16 -- stale comments incl. operator-visible YANG | updated | `grep` clean |

Still open and homed, not silently dropped:

| Item | Where |
|------|-------|
| Run 1 NOTE 12 (RFC 2545 32-byte next hop truncated), NOTE 13 (complex families store the whole MP_REACH block), NOTE 15 (no relay backpressure), NOTE 17 (no `Coordinator.RelayStoredRoute` test) | `plan/deferrals/fixit-bgp-egress-rail-divergence.md` |
| Run 1 ISSUE 9 (`adj-rib-in-replay-on-peerup.ci` replays to a non-peer, so it does not gate) | same shard |
| A-3 add-path storage normalisation (what `errRelayAddPath` refuses on) | same shard |

### Suite state

`ze-test bgp plugin --all`: **491/495**, up from 470 before the fixes, and equal to the
baseline commit `e4076920c` recorded. Zero `relay-stored-route` errors across the run.
Remaining 4: 37 and 145 (external-plugin tests, feature-tag build artifact), 460
(`plan/known-failures/bgp-plugin-show-l2tp-tunnel-detail.md`), 398 (new shard
`plan/known-failures/bgp-plugin-role-otc-export-unknown.md` -- 3/3 in isolation, not
reproduced in 10 stress-repro invocations).
# Review Gate -- spec-fixit-bgp-egress-rail-divergence

## Run 2 (fixes for Run 1), verdict: findings

Two independent reviewers again. Run 1's BLOCKERs and ISSUEs were fixed; Run 2 found that
one of those fixes INTRODUCED a blocker and that two others rested on false premises.

### Fixed in Run 2, then corrected again after Run 2 found them wrong

| # | Severity | Finding | Status |
|---|----------|---------|--------|
| R2-1 | BLOCKER | A route legitimately suppressed by egress policy was counted as a failed relay. `forwardUpdateCore` returns `errNoEstablishedPeersToForwardTo` when no destination was dispatched to (`reactor_api_forward.go:689-691`), and with the single peer this call targets that is exactly what correct suppression looks like -- RFC 7947 community policy (`:470-472`), RFC 4456 reflection (`:474-479`) and the RFC 9234 role step (`:513-514`) all `continue` past the peer. My `relayed < eligible` check then failed the WHOLE replay, `replayCommand` returned `statusError`, and bgp-rs skipped its delta-convergence loop -- strictly FEWER routes delivered than before the guard existed, and the common case on a route server. | **FIXED**: `errors.Is(fwdErr, errNoEstablishedPeersToForwardTo)` now counts as handled, not dropped. |
| R2-2 | ISSUE | The `eligible` counter fell open: `eligible++` came AFTER the unparseable-source `continue`, so a route dropped for a bad source was invisible to the completeness check and the call returned nil having relayed nothing. Also made `errRelayIncomplete` unreachable. | **FIXED**: `eligible++` moved above the parse guard; `source == destination` decrements it (never eligible). |
| R2-3 | ISSUE | Three comments asserted bgp-rs sends End-of-RIB only on a successful replay. FALSE: `rs/server_handlers.go:236-240` sends EOR on the failure path too ("Always send EOR when replay terminates"). The fail-closed claim those comments made was not honored by the consumer. | **FIXED**: all three corrected to state what error propagation actually buys (ERROR visibility + skipping the delta loop), and that EOR suppression lives in bgp-rs. |

### Open -- design decisions, NOT mechanical fixes

| # | Severity | Finding | Why not fixed here |
|---|----------|---------|--------------------|
| R2-4 | ISSUE | `OnAllPluginsReady` is NOT ordered before session establishment. `startup.go:220-242` spawns one goroutine per plugin and does not wait; `startup.go:207-212` then calls `SignalPluginStartupComplete` -> `bgpReactor.StartPeers()` (`bgp/plugin/register.go:174-180`). No happens-before edge, so the ownership claim races TCP connect + OPEN. Narrower than the old first-replay latch, but AC-5 rests on an unenforced race. | The fix is declarative ownership (bgp-rs declares it in Stage 1 `declare-registration`, or `signalStartupComplete` waits for post-startup delivery before starting peers). Both change plugin-startup semantics for every plugin. |
| R2-5 | ISSUE | The claim does not survive an adj-rib-in respawn (`SendPostStartup` has one call site, inside `signalStartupComplete`; `process/manager.go` `Respawn` gets no post-startup callback), so a respawned adj-rib-in resumes self-replay and the duplicate returns. Separately, event delivery is per-peer per-plugin (`reactor/config.go:723-800`), so a peer whose `process` block gives `state` to adj-rib-in but not bgp-rs leaves NOBODY replaying -- a config away, not a crash away. | Needs scoped (per-peer) ownership or a re-confirm/expiry protocol. |
| R2-6 | ISSUE | The ADD-PATH refusal removes behavior that previously WORKED. Both reviewers independently established that the old rail emitted `nlri <fam> add <hex>` with no `addpath` keyword, `parseWireNLRISection` defaults `addPath=false` (`cmd/update/update_wire.go:272-278`), and the structured ingest stores bare prefixes (`nlri/iterator.go:71-96`) -- so add-path-sourced routes replayed correctly, collapsed to path-id 0, for single-path prefixes. The refusal keys on the SOURCE context, so one add-path peer now kills replay of its routes to EVERY destination. | Reviewer's proposed fix -- tag the reconstruction with `bgpctx.EncodingContextForASN4` (ASN4 width, no add-path) instead of `src.ctxID` -- is plausible and would restore reach, but it silently collapses multi-path routes and needs its own correctness argument. Recorded as a functional regression accepted as an interim, per `ai/rules/no-parking.md`, not as a fix. |
| R2-7 | ISSUE | `relayChunkSize` chunks by ROUTE COUNT, which does not bound BYTES. `AttrHex` is hex (~2x the attribute block), so 4096 routes x a 4 KB block is ~33 MB, already over `rpc.MaxMessageSize` (16 MB). Fails closed, so availability not corruption. | Needs a byte-budget accumulator. |
| R2-8 | NOTE | `relayRoutes`' test seam `routeRelayer` has no error return, so `replayCommand`'s new `statusError` path cannot be driven by a test. | Small, but it is test-seam design. |
| R2-9 | NOTE | `rs.claimReplayOwnership` has no test; `relay_payload_test.go:33` asserts `n <= size` where `n == size` is the real contract; `test/plugin/adj-rib-in-replay-on-peerup.ci:5` still says "routeSender spy". | Cleanup. |

### Verified correct by Run 2

- Add-path guard PLACEMENT: precedes every buffer acquisition and cache insertion; the nil-context bypass needs `src.ctxID == 0`, only reachable when no add-path knowledge exists anywhere (benign) or on registry exhaustion (already logged). No establishment race -- `setEncodingContexts` runs before `setState(Established)` (`peer_run.go:344-346`).
- Payload arithmetic: `relayPayloadLen` matches `writeRelayPayload` byte-for-byte on all three branches; `maxUpdateBodyLen = 65516` correctly bounds the 64K pool buffer.
- Deferred Release: exactly-once on every path including early `forwardUpdateCore` error; no leak.
- Dead code: clean, `make ze-lint-changed` exit 0.
- `no-test-deletion` compliance: clean. No skips, no downgrades, no removed assertions except for genuinely removed functionality. `TestReplayCommandFormat` -> `TestReplayRouteCarriesSource` is strictly stronger (full struct equality through the real producer). The `TestAdjRibInReplayArgsPassthrough` fixture change is legitimate: at HEAD it passed only because `updateRoute` swallowed the failure, and the new version ADDS two assertions HEAD never made.
- RFC-tagged tests untouched.

### Gates

| Gate | Result |
|------|--------|
| `make ze-lint-changed` | exit 0, 0 issues |
| `make ze-race-reactor` | 0 data races, `ok ... reactor 101.873s` |
| `ze-test bgp plugin --all` | 491/495 (baseline `e4076920c` recorded the same), zero `relay-stored-route` errors |
| 372 / 378 / 394 / 395 | pass; not reproduced under stress-repro |
