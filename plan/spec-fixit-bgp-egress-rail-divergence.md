# Spec: fixit-bgp-egress-rail-divergence

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-fixit-stored-route-relay-hardening.md |
| Phase | 7/7 |
| Updated | 2026-08-14 |

## Unblocked 2026-08-14: the ordering fix landed, so the decision is moot

This spec was blocked on a choice between closing with AC-5 partial and holding
it open until the startup ordering became deterministic. Neither is needed. The
ordering fix landed in the engine, and AC-5 is met at the producer.

Replay ownership is no longer decided on the post-startup fan-out. It is
DECLARED in the plugin's static registration and delivered on the Stage-2
configure callback:

- `bgp-rs` declares the token: `Claims: []string{ClaimPeerUpReplay}`
  (`internal/component/bgp/plugins/rs/register.go`), with the token defined as
  `bgp-peer-up-replay` (`rs/server_handlers.go` `ClaimPeerUpReplay`).
- The engine resolves the claim set from the STATIC registration of every
  prospective plugin, so the claimant does not have to have handshaked yet
  (`Server.advertiseClaims`, `internal/component/plugin/server/startup_claims.go`).
- It is delivered on Stage 2, from `Server.deliverConfigRPC`
  (`internal/component/plugin/server/startup.go`), which calls
  `advertiseClaims(proc.Name())` and passes the result to `SendConfigure`.
- `bgp-adj-rib-in` stands its self-replay down there:
  `AdjRIBInManager.applyStartupClaims`
  (`internal/component/bgp/plugins/adj_rib_in/rib_claims.go`), called from the
  configure handler (`adj_rib_in/rib.go`).

The ordering is structural, not probabilistic. Stage 2 is one step of the
sequential handshake `runStartupHandshake` drives
(`internal/component/plugin/server/startup_driver.go`), so it returns before the
plugin sends Stage 5 ready, which is before its startup phase completes, which
is before `signalStartupComplete` calls `SignalPluginStartupComplete` ->
`StartPeers` (`startup.go`). `advertiseClaims`'s own doc comment states it:
"There is no window."

`sendPostStartupToAll` still does not wait, and that is now correct rather than
a gap: its doc comment records the 2026-07-25 deadlock that made waiting
impossible, and directs any pre-peer-up decision to the claim channel instead.

**What the claim channel does NOT cover, found by the closure review.** Stage 2
runs once per handshake, so a plugin that is ALREADY configured is never re-told.
The backstop `claimReplayOwnership` (`rs/server_handlers.go`) does not close that:
its only caller is `OnAllPluginsReady` (`rs/server.go`), which the engine produces
once, from `sendPostStartupToAll` inside `signalStartupComplete`. Two cases stay
open, and each is a criterion of `spec-fixit-stored-route-relay-hardening`:

| Case | Why the claim misses it | Owner |
|------|------------------------|-------|
| bgp-rs joins mid-life, auto-loaded by a reload that adds the `bgp` root | bgp-adj-rib-in is already configured, so nothing re-tells it, and both replay on the next peer-up | AC-12, added by this closure |
| bgp-adj-rib-in RESPAWNS | `ProcessManager.Respawn` calls `StartWithContext` and runs no startup handshake, so no Stage 2 happens | AC-5 |

A bgp-adj-rib-in AUTO-LOADED later is NOT one of them: `autoLoadForNewConfigPaths`
starts it through `runPluginPhase`, so it gets its own Stage 2 and
`advertiseClaims` tells it there. The comment on `claimReplayOwnership` claimed
mid-life coverage it does not have, and this closure corrects it to exactly the
two rows above.

AC-5 therefore closes on the STARTUP path, which is the path the four reproduced
failures took and the only path this spec's Task describes.

Follow-up R6-1 (`filterapi.EgressFilterFunc` returns a bare bool, so an OTC
lookup failure cannot be told apart from a policy decision) stays homed in
`spec-fixit-stored-route-relay-hardening`. It is a separate defect class with
its own AC set, and it does not gate any AC here.

> **Concurrency note (2026-07-24).** Two sessions worked this spec at once. Phase 1
> (RPC/SDK/coordinator plumbing) and Phase 2 (`relay_payload.go` byte builders) were
> built in parallel by different sessions and are complementary, not duplicated — the
> join point is `buildRelayUpdate`. Check `git log` and the working tree before
> starting any phase here.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. The export-filter-applied-twice, private-ASN-leak and BGP-concurrency-races fixits (records retired with the learned corpus)
3. Reproduction captures: `tmp/stress-repro/bgp-plugin-{372,378,394}-*.log`
4. Origin: split out of the load-dependent-functional-failures fixit, closed 2026-07-24 (owner decision — the forward-rail fix is a spec-sized new primitive, not a redirect).

## Task

Four multi-peer forward tests fail under load with WRONG WIRE BYTES to a peer that establishes
concurrently with an incoming UPDATE being relayed (verified via stress-repro, 32 burners/16 cores):
- `372` remove-private-as-replace-peer: `AS_PATH [65002 64496 65002 64497]` (local AS 65000 rewritten to peer AS 65002).
- `378` rfc7606-relay-one-field: a DUPLICATE announce frame.
- `394` role-otc-egress-filter / `395` role-otc-egress-stamp: a SPURIOUS `WITHDRAWN` / OTC not suppressed.
- `351` redistribute-l2tp-multi-peer-nexthop: multi-peer forward mismatch (same class; not individually reproduced yet).

**Verified root cause — two egress rails disagree.** A route relayed from peer A to peer B reaches B via:
- **Forward rail (correct):** `forwardUpdateCore` (`reactor_api_forward.go`) runs `orderedEgressSteps`
  (export policy chain + in-process role/OTC/community filters) on the RECEIVED wire, THEN prepends
  local AS, writes PRE-FILTERED (`forward_pool.go`, `forward_rs.go`).
- **Replay rail (buggy):** on B's peer-up, adj-rib-in replays the stored RAW route as an announce
  `update hex … add` (`adj_rib_in/rib.go`, `:736-743`, `formatHexCommand:774-778`). The
  announce builder prepends local AS FIRST, THEN the session write gate runs ONLY `facts.exportFilters`
  on the already-prepended wire (`egress_inject_filter.go`, esp. `:76`, `:66`), skipping the
  in-process role/OTC filters (`role/register.go` registers OTC via `filterapi`, not
  `facts.exportFilters`). Wrong order (prepend→filter) + incomplete filter set.
- **372:** replay prepends 65000 → remove-private-as REPLACE rewrites private 65000 → 65002.
- **378:** no export filter → replay bytes == forward bytes → duplicate; amplified by a DOUBLE replay
  trigger (adj-rib-in self-replay + bgp-rs `request bgp adj-rib-in replay`, `rs/server_handlers.go`).
- **394:** in-process OTC egress step never runs on the replay rail (exact withdraw byte UNVERIFIED).

**Fix (owner decision 2026-07-24): route the peer-up replay through the FORWARD rail** so a relayed
route has ONE egress transform, and dedupe the double trigger. NOT option (b) "make the two rails
identical". Never widen a test to green (these are real product bugs — `no-parking`).

## Required Reading

### Architecture Docs
- [ ] The three fixits named in the reading list above
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
- [ ] `internal/component/plugin/types_bgp.go` — `ReactorCacheCoordinator` (:361); `pkg/plugin/sdk/sdk_engine.go` (`ForwardCached`); `plugin/server/dispatch_cached.go`; `pkg/plugin/rpc/bridge.go`.
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` — `replayForPeer` (:149-208), needs a synchronous completion signal for EOR (:212-263).

**Behavior to preserve:**
- Wire correctness of forwarded routes unloaded (the passing case must not change).
- Originated/injected/redistribute routes STILL egress-filtered via `exportFilterForBody` (/1231).
- Standalone adj-rib-in (no bgp-rs) still replays on peer-up.

## Data Flow (MANDATORY)

### Entry Point
adj-rib-in `handleState`/`handleStructuredState` on a peer `isUp` event (`rib.go`).

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
- `forwardUpdateCore` (`reactor_api_forward.go`) — the single egress transform the replay must reuse.
- `ReactorCacheCoordinator` (`types_bgp.go`) / DirectBridge (`bridge.go`) — where `RelayStoredRoute` plugs in beside `ForwardCached`.
- `exportFilterForBody` (`egress_inject_filter.go`) — stays for originated/injected/redistribute; the replay stops using it.

### Architectural Verification
- [ ] One egress transform for relayed routes (no rail divergence)
- [ ] No new communication mechanism (typed coordinator call, not a new pattern — `plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `AttrsWire.Packed()` excludes MP_REACH/UNREACH but includes type-3 NEXT_HOP | comment `rib.go`; MP/legacy nhop split `rib.go` vs `:282` | MP reconstruction step changes | read the wire splitter that populates `AttrsWire` | **BROKEN** |
| A-2 | the replay rail prepends local AS before the egress gate | inferred from `SendAnnounce` (`session_write.go,514`) | order claim wrong | read the announce builder body | **confirmed** |
| A-3 | add-path path-id is lost on structured ingest (`rib.go`) vs legacy | two ingest paths differ | add-path replay wrong through forward rail | add-path replay test | unvalidated |
| A-4 | on peer-up BOTH adj-rib-in self-replay and bgp-rs `replayForPeer` fire; only rs gates the concurrent forward (`Replaying`) | `rib.go` + `rs/server_handlers.go` | 378 needs a different dedupe | read both peer-up handlers | **confirmed** |

**A-1 BROKEN — evidence.** `RawMessage.AttrsWire` is assigned `wireUpdate.Attrs()`
(`internal/component/bgp/reactor/reactor_notify.go,345`), and `WireUpdate.Attrs()`
builds it over `u.sections.Attrs(u.payload)` — the WHOLE path-attribute section
(`internal/component/bgp/wireu/wire_update.go`). `Packed()` returns those bytes
verbatim (`internal/core/bgp/attribute/wire.go`). So a stored `AttrHex` for an
MP family CONTAINS the entire MP_REACH_NLRI attribute, including every NLRI of the
originating UPDATE — not just this route's. The `rib.go` doc comment asserting
otherwise is its author's belief, not a producer fact (`evidence.md`).
→ Constraint: the reconstruction MUST strip attribute types 14/15 from the stored
   block and re-synthesize a single-NLRI MP_REACH, otherwise a replayed MP route
   emits a duplicate MP_REACH and re-announces the whole original NLRI set.
→ Constraint: attribute ORDER of the surviving attributes must be preserved —
   the `.ci` expectations are exact hex, and a real forward preserves source order.

**A-2 confirmed — evidence.** The replay rail is `update hex … add` →
`handleUpdateWire`/`DispatchNLRIGroups`
(`internal/component/bgp/plugins/cmd/update/update_wire.go`,
`update_text.go`) → `AnnounceNLRIBatch`
(`internal/component/bgp/reactor/reactor_api_batch.go`), whose AS_PATH builder
prepends local AS for a non-RS-client eBGP peer
(`buildBatchASPath` :317-345, `packedWithLocalASPrepended` :385) BEFORE
`sendUpdateWithSplit` → `writeUpdateGated(update, gate=true)`
(`session_write.go,268-289`) runs `exportFilterForBody`
(`egress_inject_filter.go`). Prepend-then-filter, and the gate runs ONLY
`facts.exportFilters` (`egress_inject_filter.go,76`), never the in-process
`orderedEgressSteps` that carry role/OTC (`role/register.go` registers via
`filterapi.Register`, i.e. the forward rail's `orderedEgressSteps` only).

**A-4 confirmed — evidence.** 378's duplicate is NOT two replays: it is one replay
plus one forward. `bgp-rs` marks a peer `Replaying` and excludes it from forward
targets until replay completes (`rs/server_handlers.go`), so the rs replay
is already race-free. adj-rib-in's OWN peer-up self-replay
(`rib.go`, `:732-744`) has no such gate, so under load it emits the route a
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
| "route via forward rail" = point replay at `ForwardCached` | `ForwardCached`/`recentUpdates` is cache-keyed; adj-rib-in routes never cache-resident | mechanism investigation (`recent_cache.go`) | needs a NEW `RelayStoredRoute` primitive, not a redirect |
| A-1: stored `AttrHex` excludes MP_REACH/UNREACH | it is the FULL attribute section, MP attributes included (`reactor_notify.go` → `wire_update.go` → `attribute/wire.go`) | read the producer chain during the audit, before any code | reconstruction gains a mandatory strip-14/15 step; without it an MP replay duplicates MP_REACH and re-announces the source UPDATE's whole NLRI set |
| 378's duplicate is two replays racing | it is ONE replay plus the reactor forward; rs already gates its own replay with `Replaying` | read both peer-up handlers (`rib.go`, `rs/server_handlers.go`) | dedupe is "rs owns replay, adj-rib-in stands down", not "merge two replays" |

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
| C-8 | No new communication mechanism: typed coordinator call mirroring `ForwardCached`, JSON RPC fallback for forked plugins | code read against `ai/rules/plugins.md` DirectBridge section |

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

Re-verified 2026-08-14 by an independent closing session, against the producers.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/reactor_api_relay.go` | Yes | `ls -1` 20K |
| `internal/component/bgp/reactor/relay_payload.go` | Yes | `ls -1` 13K |
| `internal/component/bgp/plugins/adj_rib_in/rib_claims.go` | Yes | `ls -1` 2.7K |
| `internal/component/plugin/server/startup_claims.go` | Yes | `ls -1` 11K |
| `test/plugin/adj-rib-in-replay-on-peerup.ci` | Yes | `ls -1` 13K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-4 | 372 / 378 / 394 / 395 / 351 green, no reproduction under stress | Recorded 2026-07-24 and last run on 2026-07-25 through Run 5 (`ze-test bgp plugin 460 372 380 396 397` `pass 5/5`). Not re-run at closure. The four `.ci` files are unchanged since, and the closure diff is two comments plus this spec. The relay and claim code around them is NOT unchanged: 102 commits have touched `internal/component/bgp/reactor/`, `adj_rib_in/`, `rs/` and `internal/component/plugin/server/` since 2026-07-26, the claim mechanism among them |
| AC-5 (dedupe) | Exactly one replay when both plugins load; standalone adj-rib-in still replays | MET. `bgp-rs` declares the role (`rs/register.go` `Claims: []string{ClaimPeerUpReplay}`), the engine reads it from the static registration and delivers it on Stage 2 (`Server.advertiseClaims`, `Server.deliverConfigRPC` calling `SendConfigure`), and adj-rib-in stands down there (`AdjRIBInManager.applyStartupClaims`). Tests green 2026-08-14: `ok .../adj_rib_in 1.693s`, `ok .../rs 2.018s`, `ok .../plugin/server 13.920s`, `ok pkg/plugin/sdk`, `ok .../bgp/reactor 24.344s` |
| AC-5 (ordering) | The claim is in place before any session can establish | Two tests, one per half, and neither covers the other. SDK half: `TestStartupClaimsPrecedeReady` (`pkg/plugin/sdk/claims_test.go`) asserts `ClaimActive` is true inside the configure handler while the engine has NOT yet received Stage 5 ready; its red-mutation is moving the claim assignment in `sdk_dispatch.go` `handleConfigure` below the callback. Engine half: `TestDeliverConfigCarriesClaimToPlugin` (`plugin/server/startup_claims_test.go`), which drives `deliverConfigRPC` itself. The closure review found the first test's header claiming both halves; the header is corrected in this commit |
| AC-5 (mid-life) | bgp-rs joins after bgp-adj-rib-in is configured, or bgp-adj-rib-in respawns | NOT covered, and homed rather than closed over: `spec-fixit-stored-route-relay-hardening` AC-12 and AC-5. A bgp-adj-rib-in auto-loaded later IS covered, by its own Stage 2. See the "What the claim channel does NOT cover" table at the top of this spec |
| AC-5 (standalone) | adj-rib-in with no bgp-rs still replays | `TestReplayOwnerDedupe/standalone self-replays` and `/unclaimed role leaves self-replay on` (`adj_rib_in/rib_test.go`); nil `claimActive` resolves to "not claimed" (`rib_claims.go`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| adj-rib-in peer-up replay -> `RelayStoredRoute` -> `forwardUpdateCore` | `test/plugin/adj-rib-in-replay-on-peerup.ci` | Yes. Read at closure: it replays to an ESTABLISHED dest on 127.0.0.2 and asserts exact wire bytes twice (seq=1 the live forward, seq=2 the relayed copy). The vacuous version this replaced is recorded in the deferral shard, with the mutation run that reproduced the vacuity |
| both adj-rib-in and bgp-rs loaded -> replay-owner dedupe | `TestReplayOwnerDedupe` plus `TestStartupClaimsPrecedeReady` | Yes. The dedupe is now decided in the startup handshake, so the ordering gate is a Go test rather than a `.ci` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | **broken** | `RawMessage.AttrsWire` carries the WHOLE attribute section (`reactor_notify.go` -> `wireu.WireUpdate.Attrs` -> `attribute.Packed`). Mistake Log row above; the strip-14/15 step in `relay_payload.go` `isRelayStrippedAttr` is the consequence |
| A-2 | **confirmed** | The old rail prepended local AS in `buildBatchASPath` before `writeUpdateGated` ran `exportFilterForBody`. That rail is gone: `formatHexCommand` and `buildReplayCommands` are deleted from `internal/`, `pkg/` and `cmd/` |
| A-3 | **confirmed** | The two ingest paths do store different NLRI framings, so add-path replay is refused rather than guessed (`errRelayAddPath`). Homed in `spec-fixit-stored-route-relay-hardening` through the deferral shard |
| A-4 | **confirmed** | Read at both peer-up handlers in Run 2. Superseded in the fix: ownership no longer depends on which handler runs first |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| D-5 API/RPC docs | `docs/architecture/api/process-protocol.md` carries the `relay-stored-route` row; `docs/architecture/api/ipc_protocol.md` names `ze-plugin-engine:relay-stored-route` with a source anchor on `pkg/plugin/rpc/types.go` | Yes, already landed |
| D-6 Plugin SDK | `docs/plugin-development/protocol.md` lists `RelayStoredRoute` with its input type | Yes, already landed |
| D-8 RFC compliance | `docs/features/rfc-status.md` RFC 9234 row states OTC egress with its producer `role/otc.go` `payloadAdvertisesNLRI`. The relay reproduces the received wire, so the row needs no change for the replay path | Yes, no edit needed |
| D-9 Architecture | The `egress_inject_filter.go` header listed "bgp-adj-rib-in replay" among the routes that reach that gate. Stale since the relay switch. CORRECTED in this commit: the header now states the replay goes through `RelayStoredRoute` -> `forwardUpdateCore` and never reaches the gate. `docs/architecture/api/architecture.md`'s replay text is about the separate `plugins/rib/` plugin and its `update text` rail, so it is untouched | Yes, fixed here |
| D-1..D-4, D-7, D-10 | No user-facing feature, config, CLI, wire encoding or runner change | Yes |

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
| AC-4 (351 multi-peer nexthop) | **MET, but NOT attributable to this spec** | `ze-test bgp plugin --pattern redistribute-l2tp-multi-peer-nexthop` -> `pass 1/1`; `stress-repro.py bgp --test "plugin 351" --iterations 12 --any-failure` -> "not reproduced in 12 invocation(s) under load" (`tmp/stress-repro/bgp-plugin-351-20260725-000715.log`). **351 does not exercise the replay rail at all**: it launches `ze -` with no `--plugin` flags (`test/plugin/redistribute-l2tp-multi-peer-nexthop.ci`) and its `plugin {}` block loads only `l2tp-nexthop-test`, `redistribute-orchestrator` and `fakel2tp` -- neither `bgp-adj-rib-in` nor `bgp-rs` is present, so no peer-up replay exists in that test and `RelayStoredRoute` is never reached. Its load-dependent failure was the RFC 6286 duplicate BGP Identifier race, fixed by commit `e4076920c` (which gave the second ze-peer a distinct identifier, `+option=open:value=router-id:id=1.2.3.6`). 351 was mis-triaged into this spec's cluster; the egress-rail change neither caused nor fixed it. |
| AC-5 (dedupe) | **PARTIAL on 2026-07-25, SUPERSEDED 2026-08-14 -- now MET, see the Implementation Audit below** | The GATE is proven: `TestReplayOwnerDedupe` covers both directions (standalone still replays, owned stands down) and is mutation-verified (removing `!r.replayOwned.Load()` turns it RED). The AC's full text -- "both plugins loaded, peer-up: exactly one replay" -- is NOT proven, and a third review pass (2026-07-25) downgraded this row from MET. The test's own premise is that "bgp-rs claims ownership at startup, before any session establishes" (`rib_test.go`), which is exactly the ordering R2-4 shows is unenforced: `sendPostStartupToAll` does not wait before `StartPeers` (`plugin/server/startup.go`), so a peer establishing immediately can still be replayed twice, as `claimReplayOwnership` itself documents (`rs/server_handlers.go`). The test asserts the flag mechanism and therefore cannot fail when the race is lost. Deterministic ownership is homed in `plan/spec-fixit-stored-route-relay-hardening.md`. |

### Gates

| Gate | Result |
|------|--------|
| `make ze-race-reactor` (`-race -count=20`) | **clean** -- 0 data races, `ok ... reactor 118.140s` |
| `go vet ./internal/... ./pkg/...` | clean (only the pre-existing, deliberate `textbuf.go` noescape warning) |
| `make ze-test-bgp` | green after dropping the OTC fallback (see Deviations) |
| `make ze-test-plugins` | only `TestISISLSDBSync` red -- IS-IS LSDB, unrelated subsystem, untouched by this spec |
| `ze-test bgp plugin --all` (495 tests) | 470 pass. **Zero** `relay-stored-route` errors anywhere. 3 failures (223/254/351) carry the RFC 6286 subcode-3 signature; the remaining 22 predate this work or belong to other in-flight sessions and need a clean-tree baseline to attribute. |

### Deviations

| What | Why |
|------|-----|
| Dropped the `OTCEgressFilter` `src-role` config fallback from this spec | Not required by any AC: removing it and re-running 394/395 keeps both green, because ingress stamps OTC into the WIRE bytes before storage and `checkOTCEgress` sees it on the reconstruction. Landing it would require editing `TestOTCEgressNoStampProvider`, which carries `RFC requirement: RFC9234-5-4 negative` and needs explicit user approval. Homed in `plan/deferrals/fixit-bgp-egress-rail-divergence.md` -> spec-fixit-otc-src-role-meta-fallback, which landed it and closed 2026-08-11. |
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
| R2-1 | BLOCKER | A route legitimately suppressed by egress policy was counted as a failed relay. `forwardUpdateCore` returns `errNoEstablishedPeersToForwardTo` when no destination was dispatched to (`reactor_api_forward.go`), and with the single peer this call targets that is exactly what correct suppression looks like -- RFC 7947 community policy, RFC 4456 reflection and the RFC 9234 role step all `continue` past the peer. My `relayed < eligible` check then failed the WHOLE replay, `replayCommand` returned `statusError`, and bgp-rs skipped its delta-convergence loop -- strictly FEWER routes delivered than before the guard existed, and the common case on a route server. | **FIXED**: `errors.Is(fwdErr, errNoEstablishedPeersToForwardTo)` now counts as handled, not dropped. |
| R2-2 | ISSUE | The `eligible` counter fell open: `eligible++` came AFTER the unparseable-source `continue`, so a route dropped for a bad source was invisible to the completeness check and the call returned nil having relayed nothing. Also made `errRelayIncomplete` unreachable. | **FIXED**: `eligible++` moved above the parse guard; `source == destination` decrements it (never eligible). |
| R2-3 | ISSUE | Three comments asserted bgp-rs sends End-of-RIB only on a successful replay. FALSE: `rs/server_handlers.go` sends EOR on the failure path too ("Always send EOR when replay terminates"). The fail-closed claim those comments made was not honored by the consumer. | **FIXED**: all three corrected to state what error propagation actually buys (ERROR visibility + skipping the delta loop), and that EOR suppression lives in bgp-rs. |

### Open -- design decisions, NOT mechanical fixes

| # | Severity | Finding | Why not fixed here |
|---|----------|---------|--------------------|
| R2-4 | ISSUE | `OnAllPluginsReady` is NOT ordered before session establishment. `startup.go` spawns one goroutine per plugin and does not wait; `startup.go` then calls `SignalPluginStartupComplete` -> `bgpReactor.StartPeers()` (`bgp/plugin/register.go`). No happens-before edge, so the ownership claim races TCP connect + OPEN. Narrower than the old first-replay latch, but AC-5 rests on an unenforced race. | The fix is declarative ownership (bgp-rs declares it in Stage 1 `declare-registration`, or `signalStartupComplete` waits for post-startup delivery before starting peers). Both change plugin-startup semantics for every plugin. |
| R2-5 | ISSUE | The claim does not survive an adj-rib-in respawn (`SendPostStartup` has one call site, inside `signalStartupComplete`; `process/manager.go` `Respawn` gets no post-startup callback), so a respawned adj-rib-in resumes self-replay and the duplicate returns. Separately, event delivery is per-peer per-plugin (`reactor/config.go`), so a peer whose `process` block gives `state` to adj-rib-in but not bgp-rs leaves NOBODY replaying -- a config away, not a crash away. | Needs scoped (per-peer) ownership or a re-confirm/expiry protocol. |
| R2-6 | ISSUE | The ADD-PATH refusal removes behavior that previously WORKED. Both reviewers independently established that the old rail emitted `nlri <fam> add <hex>` with no `addpath` keyword, `parseWireNLRISection` defaults `addPath=false` (`cmd/update/update_wire.go`), and the structured ingest stores bare prefixes (`nlri/iterator.go`) -- so add-path-sourced routes replayed correctly, collapsed to path-id 0, for single-path prefixes. The refusal keys on the SOURCE context, so one add-path peer now kills replay of its routes to EVERY destination. | Reviewer's proposed fix -- tag the reconstruction with `bgpctx.EncodingContextForASN4` (ASN4 width, no add-path) instead of `src.ctxID` -- is plausible and would restore reach, but it silently collapses multi-path routes and needs its own correctness argument. Recorded as a functional regression accepted as an interim, per `ai/rules/completion.md`, not as a fix. |
| R2-7 | ISSUE | `relayChunkSize` chunks by ROUTE COUNT, which does not bound BYTES. `AttrHex` is hex (~2x the attribute block), so 4096 routes x a 4 KB block is ~33 MB, already over `rpc.MaxMessageSize` (16 MB). Fails closed, so availability not corruption. | Needs a byte-budget accumulator. |
| R2-8 | NOTE | `relayRoutes`' test seam `routeRelayer` has no error return, so `replayCommand`'s new `statusError` path cannot be driven by a test. | Small, but it is test-seam design. |
| R2-9 | NOTE | `rs.claimReplayOwnership` has no test; `relay_payload_test.go` asserts `n <= size` where `n == size` is the real contract; `test/plugin/adj-rib-in-replay-on-peerup.ci` still says "routeSender spy". | Cleanup. |

### Verified correct by Run 2

- Add-path guard PLACEMENT: precedes every buffer acquisition and cache insertion; the nil-context bypass needs `src.ctxID == 0`, only reachable when no add-path knowledge exists anywhere (benign) or on registry exhaustion (already logged). No establishment race -- `setEncodingContexts` runs before `setState(Established)` (`peer_run.go`).
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

## Review Gate -- Run 3 (independent, fresh session, 2026-07-25)

A third pass from a session that did not author the implementation, reviewing the LANDED
code at `f0e4d8c4e` rather than a diff. Run 2's findings were re-verified against the
producing functions rather than taken on trust; three of its records turned out to
contradict the code they cite.

| # | Severity | Finding | Status |
|---|----------|---------|--------|
| R3-1 | ISSUE | Run 2's own R2-1 fix reopened the fail-open on a different axis. `errors.Is(fwdErr, errNoEstablishedPeersToForwardTo)` counted as handled, but `forwardUpdateCore` returns that same error whenever `dispatchedCount == 0` (`reactor_api_forward.go`), which at least five NON-suppression branches reach with a single destination: EBGP wire build incl. read-pool exhaustion, transcode buffer exhaustion, `buildFwdBody` failure, and both dispatch attempts failing. Several are silent. Under pool exhaustion -- the load condition this spec exists for -- a dropped route was counted as relayed and `relayed < eligible` could never fire. | **FIXED** at the owning layer: new sentinel `errAllDestinationsSuppressed` returned only when EVERY matching peer was skipped by policy; the relay now tests for it. Two tests, both mutation-verified (reverting to the old sentinel turns them RED). |
| R3-2 | ISSUE | `plan/deferrals/fixit-bgp-egress-rail-divergence.md` claimed "the startup ORDERING race ... was made deterministic here (`sendPostStartupToAll` now waits before `StartPeers`)". The landed function does NOT wait (`plugin/server/startup.go`), and its own comment says so. R2-4 agrees. A future reader of the shard would believe the race is closed. | **FIXED**: row corrected to state the wait was tried and reverted (deadlock), with the correction dated. |
| R3-3 | ISSUE | AC-5 was marked MET on `TestReplayOwnerDedupe`, whose own premise is "bgp-rs claims ownership at startup, before any session establishes" (`rib_test.go`) -- exactly the ordering R2-4 shows is unenforced. The test asserts the flag mechanism and cannot fail when the race is lost, so it does not prove the AC's text ("exactly one replay"). | **FIXED**: AC-5 downgraded to PARTIAL, with the gate-vs-AC distinction stated. |
| R3-4 | ISSUE | `plan/known-failures/bgp-plugin-role-otc-export-unknown.md` sent the next investigator to `mods.IsWithdraw()`. `SetWithdraw` has exactly one producer (`gr/gr_egress.go`) and 398 loads neither `bgp-gr` nor `bgp-rs` (`role-otc-export-unknown.ci`), so that path cannot fire there. Its exculpatory evidence was also weak: "zero `relay-stored-route` log lines" rules out relay ERRORS, not relay involvement (a successful relay logs nothing; suppression logs at Debug, `reactor_api_relay.go`), and 398 DOES load adj-rib-in without bgp-rs, so the new rail runs in it by construction. | **FIXED**: candidate marked ruled-out with its reason, attribution softened to what the evidence supports. |
| R3-5 | NOTE | The learned summary described replay ownership as claimed "on the first explicit `request bgp adj-rib-in replay`" -- the superseded latch design, not the shipped explicit `claim-replay` command. The summary is the artifact that survives the spec. | **FIXED**: rewritten to the shipped mechanism, keeping why the latch was dropped. |
| R3-6 | NOTE | The summary recorded neither the ADD-PATH refusal (an accepted functional regression, R2-6) nor AC-5's unenforced race. | **FIXED**: both added under Consequences. |
| R3-7 | NOTE | ~~`hex.Decode(dst, []byte(route.AttrHex))` allocated three times per stored route~~ **WITHDRAWN, the premise was false** -- see R4-2. `hex.Decode` does not leak `src`, so the conversion is elided and the old form allocates zero. | **REVERTED** in `b1bcaacc4`. The finding itself was wrong, not just its fix. |

### Verified correct by Run 3 (re-checked against producers, not taken from Run 2)

- Buffer lifecycle is exactly-once. The pending flag blocks eviction between `Add` and
  `RetainN` (`recent_cache.go`), the deferred `Release` survives a panic in
  `forwardUpdateCore` (`reactor_api_relay.go`), and the scratch slices are copied
  into `out.Buf` before `defer ReturnReadBuffer` fires.
- `maxUpdateBodyLen = 65516` (`relay_payload.go`) does bound the 65535-byte extended
  pool buffer (`session.go`, `header.go`).
- `AddPath` and `AddPathFor` are the same function (`context.go`), so the add-path
  guard reads what it intends.
- Fail-closed source resolution costs nothing: peer-down purges the store
  (`adj_rib_in/rib.go`).
- No `.ci` or Go test asserts on the `no established peers to forward to` string, so
  splitting the sentinel breaks no expectation.

### Gates (Run 3)

| Gate | Result |
|------|--------|
| `ze-lint-changed` | exit 0, 0 issues |
| `ze-test-bgp` | exit 0 (`reactor` 7.976s; `adj_rib_in`, `rs`, `reactor/filter` ok) |
| `ze-race-reactor` | exit 0, 0 data races, `ok ... reactor 101.515s` |
| `ze-test bgp plugin 372 380 396 397` | `pass 4/4 100.0% 19.1s` (ids shifted from 378/394/395 as tests were added since the spec was written) |
| New tests mutation-verified | reverting the relay to the old sentinel turns both RED ("An error is expected but got nil") |

## Review Gate -- Run 4 (independent subagents over Run 3's own code, 2026-07-25)

Run 3 was authored and reviewed by the same session, so its fix went to two
independent reviewer subagents with distinct lenses (classification correctness;
input-handling equivalence + test quality). They reviewed the LANDED commits
`ce2bc3fa3` and `94cf37b8a`. They found a BLOCKER in Run 3's own fix.

| # | Severity | Finding | Status |
|---|----------|---------|--------|
| R4-1 | BLOCKER | Run 3's fix left the fail-open on a THIRD axis. `accept == false` out of the ordered egress pass is overloaded: four producers reach it with no filter having decided anything -- a filter-plugin IPC error under the default fail-closed `FilterOnError` (`filter_chain.go` `policyFilterFunc`; `plugin/server/server.go` returns `OnErrorReject` for a missing process or an undeclared filter), an unparseable filter response, the nil-API guard (`filter_ordered.go`, whose own comment calls it a guard MISS), and a filter panic recovered by `safeEgressFilter` (`reactor_notify.go`). A forked export-filter plugin timing out under load therefore dropped every route of a replay while `RelayStoredRoute` reported it complete. | **FIXED** in `b1bcaacc4`: the reason is threaded from each producer (`PolicyResponse.Failed` -> `PolicyChainResult.Failed` -> `egressStepResult.failed`; `safeEgressFilter` returns `panicked`), and `forwardUpdateCore` counts suppression only when the step decided. Regression test drives the panic path, mutation-verified. |
| R4-2 | ISSUE | The justification for `decodeHexInto` was FALSE. `hex.Decode(dst, []byte(s))` does not allocate: `hex.Decode` never leaks `src`, so the conversion is elided (`-gcflags=-m` reports "zero-copy string->[]byte conversion"; `AllocsPerRun` over the pre-commit form gives 0). The premise was never traced to a producer -- the failure `ai/rules/evidence.md` describes, committed by the session that had just flagged the same class in someone else's work. | **FIXED**: decoder reverted to `encoding/hex`; the hand-rolled parser is gone from an untrusted-input path. |
| R4-3 | ISSUE | `TestDecodeHexIntoDoesNotAllocate` could not fail: the form it claimed to guard against also allocates zero, so it pinned nothing. | **FIXED**: deleted (not ported), with the reason recorded in a `test-relax:` note. The malformed-input boundaries were KEPT, re-pointed at `encoding/hex`, and strengthened with a high-bit case the old table never reached -- its "unicode digit" case was rejected on LENGTH, never on alphabet. |
| R4-4 | ISSUE | The dispatch-failure test drives only the `facts == nil` branch, not the pool-exhaustion / body-build branches the commit message names. A future `suppressedCount++` added to those paths would reintroduce the hole with both tests green. | Partly addressed: the R4-1 regression test adds a second, genuinely different failure branch (a failing egress step). The named branches are not reachable from a unit fixture; recorded as a coverage gap in `plan/deferrals/fixit-bgp-egress-rail-divergence.md`. |
| R4-5 | NOTE | The new tests had been inserted INTO `TestRelayStoredRouteFailsClosedWithoutSource`'s godoc block, orphaning its header sentence. | **FIXED**. |
| R4-6 | NOTE | Every `file:line` citation added by Run 3's test comments pointed at pre-diff line numbers. | **FIXED** against the current file. |
| R4-7 | NOTE | The `.ci` EOR gate added 100x0.1s on top of an existing 100x0.1s poll and the peer's 10s of SCCRQ retries, exceeding the file's own 20s budget, so a degraded run would die on the runner timeout instead of emitting the named `runtime_fail`. | **FIXED**: takes the 20x0.25s default. |
| R4-8 | NOTE | Reviewer claimed `# // test-relax:` inside a `.ci` Python block is a Go-hook spelling with no meaning there. | **REVIEWER WRONG**, verified: `scripts/dev/audit-test-relaxation.py` parses `.ci` files and reported this exact file with its reason in the Run 4 pre-check output. |

### Confirmed by Run 4, independently of the author

- The two Run 3 tests DO gate: three separate overlay mutations, each caught.
- `TestRelayStoredRouteTreatsEgressSuppressionAsHandled` really traverses the RFC
  4456 skip; it is not reaching "handled" via nil facts or unresolved peers.
- `decodeHexInto` was byte-equivalent to `hex.Decode` over a 200,000-case
  differential run (its removal was about a false premise, not a wrong decoder).
- The `.ci` gate fails closed, `json` is in scope, and `eor-sent` implies the
  bytes were flushed (`IncrEORSent` runs after `SendUpdateHeld`, which flushes).
- No BLOCKER in the hex path: bounds are checked before slicing, failures
  short-circuit before the wire.

### Gates (Run 4, after the fix)

| Gate | Result |
|------|--------|
| `ze-lint-changed` | exit 0, 0 issues |
| `ze-test-bgp` | exit 0 |
| `ze-race-reactor` | exit 0, 0 data races, `ok ... reactor 101.314s` |
| `ze-test bgp plugin 460 372 380 396 397` | `pass 5/5 100.0%` |
| `ze-validate` | all checks passed |
| R4-1 regression test | mutation-verified: pre-fix behaviour turns it RED |

## Review Gate -- Run 5 (independent refutation of Run 4's fix, 2026-07-25)

Run 4's fix was itself authored by the session that ran Run 4's triage, so it went
to a third independent subagent with one instruction: REFUTE the claim that
`b1bcaacc4` fixes what it says it fixes. It did.

| # | Severity | Finding | Status |
|---|----------|---------|--------|
| R5-1 | BLOCKER | A FIFTH producer of a non-decided `PolicyReject` was missed: the AC-13 undeclared-attribute override (`filter_chain.go`). The filter decided `PolicyModify` and ze overrode it, so by the fix's own definition it is not the filter's decision -- yet `Failed` was unset, so it propagated as a policy suppression all the way to `relayed++`. Deterministic, no race, no load: a plugin version-skew that modifies an undeclared attribute drops every route of a replay while the replay reports complete. | **FIXED**: `Failed: true` on the AC-13 reject. |
| R5-2 | ISSUE | "A missing API server" was fixed at one of the TWO sites implementing it. `policyFilterFunc`'s own `r.api == nil` guard still returned an unflagged reject. Normally shadowed by the earlier check in `runEgressPolicyChainASN4`, but not totally: `r.api` is written to nil during `abortStartup` and read unsynchronized from the forward path, and the inner branch is fully live on the ingress chain and the default-originate gate. | **FIXED**: same flag on the sibling guard. |
| R5-3 | ISSUE | The Run 4 regression test gated ONE of the four producers (the panic). Reverting the whole of `filter_chain.go`'s plumbing, or either `filter_ordered.go` assignment, left it GREEN -- including the headline "forked export-filter plugin times out under load" scenario. | **FIXED**: added `TestRelayStoredRouteCountsFailedPolicyChainAsIncomplete` (drives the policy chain end to end through the relay) and `TestPolicyFilterChainPropagatesFailed` / `TestPolicyFilterFuncFlagsNonDecisions`. Both new gates mutation-verified: reverting either `filter_ordered.go` assignment reddens the first, reverting either chain exit reddens the second. The IPC-error and AC-13 producers still need a live plugin server to drive; homed in the deferral shard. |
| R5-4 | NOTE | `PolicyFilterChain`'s teardown exit dropped `Failed` while the reject exit two lines below carried it. Currently unreachable on egress (teardown is import-only), but it is a newly-added propagation with one of three exits unwired, in an EXPORTED function. | **FIXED**, and the test table now covers the teardown exit in both polarities. |
| R5-5 | NOTE | `panicked` is discarded at two of three `safeEgressFilter` call sites. The reviewer traced both consumers: neither counts outcomes the way the relay does, and one already over-reports (the safe direction), so no fail-open. | No change. Recorded so the asymmetry is deliberate rather than forgotten. |

### Confirmed by Run 5, by reading the producers

- Every `return false` in the three registered in-process egress filters
  (`role/otc.go`, `gr/gr_egress.go`, `filter_community`) is a genuine policy
  decision; none fails false on a parse or lookup error.
- Every non-policy `continue` in `forwardUpdateCore` correctly leaves
  `suppressedCount` untouched (nil facts, EBGP wire build, both read-buffer
  exhaustion sites, both transcode failures, withdrawal conversion, body build,
  pool-stopped dispatch).
- The arithmetic is sound: each increment site is immediately followed by
  `continue`, so a peer counts at most once, and a mixed forward (one suppressed,
  one failed) correctly falls through to the failure sentinel.
- `stepFailed` cannot misattribute: it is declared inside the per-peer body and
  the loop breaks on the first non-accept.
- Other consumers of the chain (`exportFilterForBody`, the ingress chain, the
  default-originate gate, `ForwardUpdatesDirect`) ignore the new field without
  counting, so no new fail-open.
- The `decodeHexInto` revert is clean: no references remain, `go vet` clean.

### Gates (Run 5, after the fix)

| Gate | Result |
|------|--------|
| `ze-lint-changed` | exit 0 |
| `ze-test-bgp` | exit 0, 0 failures |
| `ze-race-reactor` | exit 0, 0 data races, `ok ... reactor 102.146s` |
| `ze-test bgp plugin 460 372 380 396 397` | `pass 5/5 100.0%` |
| Both new gates | mutation-verified in both directions |

## Review Gate -- Run 6 (independent completeness check, 2026-07-25)

Rounds 4 and 5 had each found a producer the previous round's enumeration missed,
so round 6 was given ONE question: is the enumeration now complete, or is there a
sixth? It found one, and it is structural rather than a missed line.

| # | Severity | Finding | Status |
|---|----------|---------|--------|
| R6-1 | ISSUE | **The in-process egress half has no failure channel.** `filterapi.EgressFilterFunc` returns a bare `bool` (`filterapi/filterapi.go`), so `safeEgressFilter` can only report the one failure it causes itself (a recovered panic). Already live in one filter: `OTCEgressFilter` rejects on the destination's role not being in the source's export set (`role/otc.go`), reading a map whose empty value means BOTH "advertised no role" (decision) and "never recorded" (failure). A validate-open RPC timeout (`bgp/server/validate.go`), a plugin conn not yet up, a missing process manager, or a plugin respawn nulling the map (`role/role.go`) all produce the second. Consequence: with a source configured `role { export ... }` and a destination whose role went unrecorded, every stored route is suppressed and counted as handled. | **HOMED, NOT FIXED** in `plan/spec-fixit-stored-route-relay-hardening.md`. Pre-existing and separable; the fix needs a signature change across three filter plugins AND role-recording semantics that separate "no role" from "not recorded". Recorded here rather than closed over: this spec's mechanism cannot see a failure the filter has no way to report. |
| R6-2 | NOTE | Two non-decision ACCEPTS in `runEgressPolicyChainASN4` (`filter_ordered.go`, `:302-308`): a raw override of 1..3 bytes is discarded by `decodeFilterRawOverride`, and a `buildModifiedPayload` failure on a malformed payload is discarded, both falling through to `accept: true` -- so the route goes out UNMODIFIED and indistinguishable from "the filter accepted it as-is". Same family, opposite polarity, and the RFC 6996 private-ASN leak class this file exists to prevent. | **HOMED** in the same spec. |

### Confirmed complete by Run 6

Every producer with a failure channel is now correctly classified. The reviewer
enumerated and checked each: all six `PolicyResponse` exits, both
`PolicyChainResult` exits, all three `egressStepResult` producers, the RFC 7947
community suppression (`ParseCommunityPolicy` fails OPEN on malformed input, so it
cannot fabricate a suppression), the RFC 4456 reflection skip (both inputs
settings-derived), `checkOTCEgress`'s wire-bytes rule (fails open on malformed
attributes), and every uncounted `continue` in `forwardUpdateCore`. The arithmetic
can only err toward "drop", never toward "handled". `LLGREgressFilter` and the
community egress filter never return false at all, so role is the only in-process
filter that can reach R6-1 today. All three Run 5 tests gate what they claim.

### Final status

- [x] Run 3 findings all fixed or homed
- [x] Run 4 (independent) BLOCKER fixed, ISSUEs fixed or homed
- [x] Run 5 (independent refutation) BLOCKER fixed, ISSUEs fixed or homed
- [x] Run 6 (independent completeness check) 0 BLOCKER; both findings homed
- [x] AC-5 restated honestly as PARTIAL rather than closed over
- [x] **Closed 2026-08-14.** The 2026-07-25 verdict above stood on AC-5 being
      unenforceable. It is enforced now: the claim channel replaced the racing
      post-startup callback, so AC-5 is MET at the producer (see "Unblocked
      2026-08-14" at the top and the Implementation Audit below). R6-1 keeps its
      own home in `spec-fixit-stored-route-relay-hardening`; it gates no AC here.

## Implementation Summary

### What Was Implemented
- `RelayStoredRoute` on the reactor (`reactor_api_relay.go`) with the byte
  builders in `relay_payload.go`, plus the coordinator, RPC, SDK and bridge
  plumbing. The adj-rib-in replay calls it (`adj_rib_in/rib.go` `relayRoutes`),
  so a relayed route gets ONE egress transform: filter, then prepend.
- Replay-owner dedupe by DECLARED claim. `bgp-rs` puts `ClaimPeerUpReplay` in
  its static registration; the engine resolves the claim set
  (`Server.advertiseClaims`) and delivers it on the Stage-2 configure callback
  (`Server.deliverConfigRPC`); adj-rib-in stands its self-replay down there
  (`AdjRIBInManager.applyStartupClaims`).
- `verifyAdvertisedClaims` speaks when a plugin stood down for a claimant that
  never reached Running. It does not fail startup; the comment records why
  making it fatal was reverted.

### Bugs Found/Fixed
- Six review rounds, each finding a producer the previous enumeration missed.
  The chain is recorded in Runs 1 to 6 above. The last product fix was R5-1
  (`Failed: true` on the AC-13 undeclared-attribute reject).

### Documentation Updates
- `internal/component/bgp/reactor/egress_inject_filter.go`: header no longer
  claims the adj-rib-in replay reaches that gate (D-9).
- D-5 and D-6 landed with the implementation. See the Documentation Verified
  table.

### Deviations from Plan
- The OTC `src-role` fallback was dropped from this spec and landed in
  `spec-fixit-otc-src-role-meta-fallback` (closed 2026-08-11).
- ADD-PATH sources are refused (`errRelayAddPath`) rather than replayed. The
  functional regression is recorded and homed in the deferral shard.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Route the peer-up replay through the forward rail | Done | `reactor_api_relay.go` `RelayStoredRoute`, called from `adj_rib_in/rib.go` `relayRoutes` | The old rail is DELETED, not merely unused: `formatHexCommand` and `buildReplayCommands` exist nowhere under `internal/`, `pkg/` or `cmd/` |
| Dedupe the double replay trigger | Done | `rs/register.go` `Claims`, `startup_claims.go` `advertiseClaims`, `rib_claims.go` `applyStartupClaims` | Declared, not raced |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | 372 `pass 1/1`, 12 stress invocations, no reproduction | |
| AC-2 | Done | 378 `pass 1/1`, 10 stress invocations, no reproduction | |
| AC-3 | Done | 394 / 395 `pass 1/1`, 10 stress invocations, no reproduction | |
| AC-4 | Done, not attributable | 351 `pass 1/1` | 351 loads neither adj-rib-in nor bgp-rs, so it never reaches the relay. Fixed by `e4076920c` (RFC 6286 duplicate identifier). Mis-triaged into this cluster |
| AC-5 | Done on the startup path | `TestStartupClaimsPrecedeReady`, `TestDeliverConfigCarriesClaimToPlugin`, `TestReplayOwnerDedupe` (seven subtests, one of them table-driven), `TestRouteServerClaimsPeerUpReplay`, nine tests in `startup_claims_test.go`, and `test/plugin/rfc7606-relay-one-field.ci`, which loads both plugins and is the only test driving the stand-down from adj-rib-in's own `OnConfigure` | Was PARTIAL on 2026-07-25 because the decision rode `sendPostStartupToAll`. The claim channel replaced it. The mid-life join is homed at hardening AC-12, not claimed here |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRelayStoredRouteEgress` | Done, renamed | `internal/component/bgp/reactor/reactor_api_relay_test.go` and siblings | Shipped as a family: fails-closed, add-path refusal, malformed input, suppression versus failure |
| `TestReplayOwnerDedupe` | Done | `adj_rib_in/rib_test.go` | Seven subtests, including both startup-claim polarities |
| 372/378/394/395/351 `.ci` | Done | `test/plugin/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `reactor_api_forward.go`, `types_bgp.go`, `sdk_engine.go`, `dispatch_cached.go`, `bridge.go` | Done | Plus `reactor_api_relay.go` and `relay_payload.go`, which the plan did not name |
| `adj_rib_in/rib.go`, `rib_commands.go` | Done | Plus `rib_claims.go`, the declared-claim receiver |
| `rs/server_handlers.go` | Done | Plus `rs/register.go`, which carries the declaration |

### Audit Summary
- **Total items:** 13
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A relayed route has ONE egress transform, so the two rails cannot diverge | functional, wire bytes | `test/plugin/adj-rib-in-replay-on-peerup.ci` asserts the live forward (seq=1) and the relayed copy (seq=2) are the same 51-byte UPDATE. Mutation-verified in both directions: with `RelayStoredRoute` dead the dest never receives the copy |
| The four load-dependent wire failures stop reproducing | functional under load | `stress-repro.py` per test, 10 to 12 invocations each, no reproduction (logs named in the Verification Evidence table) |
| Exactly one plugin replays on peer-up | unit, ordering, plus one functional | `TestStartupClaimsPrecedeReady` (`pkg/plugin/sdk/claims_test.go`) proves the claim is readable at Stage 2 before the engine has the Stage-5 ready, so the decision precedes `StartPeers`. `TestDeliverConfigCarriesClaimToPlugin` covers the engine half it does not. The functional test whose duplicate this prevents is `test/plugin/rfc7606-relay-one-field.ci`, which loads both plugins (bgp-rs pulls bgp-adj-rib-in through `OptionalDependencies` and `ExpandDependencies`). Scoped to the startup path; the mid-life join is hardening AC-12 |
| The egress gate for originated and injected routes is unchanged | unit + functional | `exportFilterForBody` keeps its callers; `ok .../bgp/reactor 24.344s` on 2026-08-14 |

## Deferrals Resolved

The shard is `plan/deferrals/fixit-bgp-egress-rail-divergence.md`. It is NOT
removed: nine rows, two terminal and seven live, and every live row names
`spec-fixit-stored-route-relay-hardening`, which is open.

The closure review checked that the destination spec actually ACCEPTS them, not
merely that the cell names it. Two did not land: the zero-dispatch failure
branches and the two untested non-decided `PolicyReject` producers appeared in
no section and no criterion there. They are now AC-10 and AC-11 of that spec,
added by this closure. Naming a destination is not homing a row.

**A FOREIGN shard pointed here, and it is removed with this closure.**
`plan/deferrals/fixit-load-dependent-functional-failures.md` held exactly one
row, `deferred`, whose Destination was this spec: it is what carved this work
out on 2026-07-24. That row is now terminal, so its shard is residue and this
closure removes it (`ai/rules/planning.md`). Its source spec closed on
2026-07-24. What the row deferred, resolved:

| Part of the row | Resolution |
|-----------------|------------|
| `RelayStoredRoute` primitive, RPC and SDK | Done, this spec |
| Per-family MP_REACH reconstruction | Done, `relay_payload.go` |
| Replay-owner dedupe | Done on the startup path, this spec. Mid-life at hardening AC-12 |
| ADD-PATH path-id gap | NOT done. The relay refuses an add-path source (`errRelayAddPath`), and normalizing the stored framing continues in this spec's own shard, homed at hardening AC-1 and AC-2. `errRelayAddPath`'s comment is repointed at those, so shipped code names a tracker that outlives this spec |
| 351 redistribute-l2tp-multi-peer-nexthop | Mis-triaged into the cluster. Fixed by `e4076920c` (RFC 6286 duplicate BGP Identifier), not by this work |

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| OTC `src-role` meta fallback | done | `spec-fixit-otc-src-role-meta-fallback`, closed 2026-08-11 |
| `adj-rib-in-replay-on-peerup.ci` was vacuous | done | Rewritten 2026-08-07 with an established destination and exact wire bytes; mutation-verified |
| ADD-PATH stored NLRI framing (A-3) | deferred | `spec-fixit-stored-route-relay-hardening` |
| RFC 2545 32-byte next hop, complex-family MP_REACH storage, relay backpressure, `Coordinator.RelayStoredRoute` untested | deferred | same |
| Replay ownership is process-global while events are per-peer; claim does not survive a respawn | deferred | same. Narrowed by this spec: the startup RACE is closed, the SCOPE questions are not |
| `relayChunkSize` bounds routes, not bytes | deferred | same |
| Zero-dispatch failure branches of `forwardUpdateCore` untested | deferred | same |
| The two remaining non-decided `PolicyReject` producers untested | deferred | same |
| R6-1: `filterapi.EgressFilterFunc` has no failure channel | deferred | same |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-bgp-egress-rail-divergence-ca112cd4-8337-4992-b4e1-e0d7bbff5820.md`, 11 files, verdict clean |
| `review_gate.py check` | clean |
| Rounds | 8. Rounds 1 to 6 are recorded above, each with the product defect it found. Round 7 (2026-08-14) is the closure review: two independent subagents, one refuting "AC-5 is now met" at the producers, one auditing the closure record. Round 8 reviewed round 7's fixes only, and refuted half of one of them |
| Reviewer lenses used | Runs 1 to 6: fix correctness, regression risk, test integrity, classification correctness, refutation, enumeration completeness. Run 7: refute the closure claim; record integrity. Run 8: are the fixes true at the producer |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| R7-1 | ISSUE | The header still named the adj-rib-in replay as a caller of that gate. D-9 asked for this and it was never done. Found by the closing session's own documentation review, not by a reviewer | `internal/component/bgp/reactor/egress_inject_filter.go` | Header corrected. The same word survived in two siblings, and both are corrected with it: `filter_ordered.go` `runEgressPolicyChainASN4` and `peer_run.go` |
| R7-2 | ISSUE | `claimReplayOwnership`'s doc claims it covers the mid-life case. It cannot: its only caller is `OnAllPluginsReady`, which the engine produces once at startup. A comment asserting coverage the code does not have is the shield that stops the next reader asking | `internal/component/bgp/plugins/rs/server_handlers.go` `claimReplayOwnership` | Doc corrected. Round 8 then caught the correction over-reaching: an auto-loaded bgp-adj-rib-in needs no backstop, because `autoLoadForNewConfigPaths` runs it through `runPluginPhase` and it gets its OWN Stage 2. Two cases are genuinely untold, and the doc now names exactly those: a RESPAWN (`ProcessManager.Respawn` calls `StartWithContext` and runs no handshake), which is hardening AC-5, and bgp-rs joining mid-life, which is hardening AC-12 |
| R7-3 | ISSUE | A claimant joining MID-LIFE reopens the duplicate: `autoLoadForNewConfigPaths` can start bgp-rs while bgp-adj-rib-in already runs, and neither channel re-tells it | `internal/component/plugin/server/startup_autoload.go`, `startup.go` `deliverConfigRPC` | Not fixed here: it is a lifecycle change, and it is now `spec-fixit-stored-route-relay-hardening` AC-12. AC-5 is closed on the startup path only, and this spec says so |
| R7-4 | ISSUE | Two live deferral rows named the hardening spec as their destination while no criterion there enumerated them | `plan/deferrals/fixit-bgp-egress-rail-divergence.md` | Added as hardening AC-10 and AC-11 |
| R7-5 | ISSUE | `TestStartupClaimsPrecedeReady`'s header claimed it catches the engine-side regression. It does not: it feeds the Stage-2 payload itself, so deleting `advertiseClaims` from `deliverConfigRPC` leaves it green. It also named a `.ci` that loads neither plugin | `pkg/plugin/sdk/claims_test.go` | Header corrected to name each half's real red-mutation and the engine-side gate `TestDeliverConfigCarriesClaimToPlugin`. Assertions untouched |
| R7-6 | NOTE | Closure record defects: a false "no code changed since" (102 commits had), "no live caller" where the symbols are deleted, a wrong subtest count, and a wrong audit total | This spec | All corrected. Per `ai/rules/planning.md` a record defect earns no further round |
| R7-7 | ISSUE | SHIPPED CODE cited this spec by full path in six places, so commit B would have left six dangling citations in Go comments. `make ze-spec-citation-check` cannot see them: `find_dangling` reads `plan/spec-*.md` citers only | `adj_rib_in/rib.go` (2), `adj_rib_in/rib_test.go`, `reactor/reactor_api_relay.go` (2), `pkg/plugin/rpc/types.go` | Five restated as the bare stem, which keeps the name and drops a promise the tree cannot keep. The sixth, `errRelayAddPath`, tracks LIVE work, so it repoints at the two documents that survive: this spec's deferral shard and hardening AC-1 and AC-2. Six other sites already used the bare stem and needed nothing |
