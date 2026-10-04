# Spec: withdraw only the routes a peer was sent

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-04 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Bucket: immediate. An operator with an export filter on a full table sees
unbounded withdrawal traffic: every UPDATE the filter rejects reaches the peer
as a withdrawal, forever, including for routes the peer was never sent.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Since commit f93a30fbfc ("reactor: withdraw a held-back route at every withhold
gate", 2026-10-03) every forward withhold gate on both forward rails converts
the destination's copy of the UPDATE into a withdrawal of every route it names.
The gates are: next-hop self with no local address, the peer's own address as
next hop, the RR Link-Local-only reflection rule, a Link-Local-only next hop
towards a multihop peer, RFC 8950 without Extended Next Hop, a Link-Local-only
next hop without capability 77, RFC 1997 well-known communities, RFC 7947
control communities, and a genuine egress policy reject. The conversion is
unconditional because neither forward rail knows whether the destination holds
the route.

The consequence: a peer an export policy rejects receives a withdrawal for
every UPDATE that policy rejects, for as long as the session lives, even for
prefixes it never received. On a full table with a restrictive export policy
that is roughly one withdrawal per received UPDATE per filtered peer, all of it
wire traffic and receiver work that changes nothing.

Owner decision (Thomas, 2026-10-04, "Track exported routes"): ze tracks, per
destination peer, which paths it actually exported, and withdraws only those.
A gate that refuses a route the destination does not hold sends nothing. A gate
that refuses a route the destination does hold sends the withdrawal of that
route only. This is what BIRD does (see "Comparison with BIRD and FRR").

RFC 4271 Section 9.1.3 states the obligation in exactly those terms: "If a
route in Loc-RIB is excluded from a particular Adj-RIB-Out, the previously
advertised route in that Adj-RIB-Out MUST be withdrawn from service by means
of an UPDATE message (see 9.2)." The obligation is on the previously advertised
route, so a record of what was advertised is what discharges it without excess.

Goal: the withdrawal a destination receives names exactly the routes that
destination holds and must lose, on both forward rails, at every withhold gate,
for the source's own withdrawals as well as for converted announcements.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/bgp/structural-forwarding.md` - section "Withheld routes are withdrawn" describes today's unconditional withdrawal
  → Constraint: the page says "neither rail keeps a per-peer Adj-RIB-Out that could say it does not, so the withdrawal is unconditional". The implementing change makes that sentence false and MUST rewrite the section in the same piece of work (`ai/rules/documentation.md`).
  → Constraint: the two rails ask the next-hop gates through one function (`egressNextHopWithheld`) so they cannot answer differently; the export record MUST be consulted the same way, through one shared function, never spelled once per rail.
- [ ] `docs/architecture/update-building.md` - section "The Adj-RIB-Out on the API Rails"
  → Decision: an Adj-RIB-Out already exists per peer for the API origination rail only (`adjRIBOut`), keyed on the NLRI as written to the wire (ADD-PATH id included), emptied by withdrawal and by session teardown.
  → Constraint: that table's safe reading of "unknown" is "the peer does not hold it" (a failed write forgets), because its job is suppression. A withdrawal gate needs the opposite safe reading: "the peer may hold it, so withdraw". One structure serving both jobs MUST keep the two defaults distinct.
  → Constraint: config-driven initial sync (`peer_initial_sync.go`) and the `SendRoutes` transaction rail do not record into `adjRIBOut`. Any route that reaches a peer and is not recorded would, under this spec, never be withdrawn. Every send path MUST record.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - section "Adj-RIB-Out native inventory"
  → Decision: the bgp-rib plugin keeps its own per-destination sent inventory (`ribOut`), built from "sent" events after the reactor writes, used for replay, refresh and GR/LLGR lifecycle.
  → Constraint: that inventory lives across a plugin boundary and is fed after the send, so it cannot answer the reactor's per-destination question synchronously on the forward hot path. It also exists only when the rib plugin is loaded.
- [ ] `docs/architecture/testing/interop.md` - interop scenario rules
  → Constraint: a wire-visible change owes an interop scenario, named, with a recorded red phase.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Section 3.2 (Adj-RIB-Out), 4.3 (withdrawn routes), 9.1.3 (Phase 3), 9.2 (Update-Send)
  → Constraint: Section 4.3, a withdrawn route "unambiguously identifies the route in the context of the BGP speaker - BGP speaker connection to which it has been previously advertised". The record is per connection and dies with the connection (Section 6.3).
  → Constraint: Section 9.1.3, an excluded route that was previously advertised MUST be withdrawn. A route that was not previously advertised owes nothing.
- [ ] `rfc/short/rfc7606.md` - Section 2, treat-as-withdraw
  → Constraint: the RFC text is "the UPDATE message containing the path attribute in question MUST be treated as though all contained routes had been withdrawn just as if they had been listed in the WITHDRAWN ROUTES field". It governs the RECEIVER's Adj-RIB-In. The forward rails borrowed it for egress conversion. Under this spec the converted withdrawal still names every contained route the destination holds, and none it does not.
- [ ] `rfc/short/rfc7911.md` - ADD-PATH path identifiers
  → Constraint: the destination names a path by (prefix, path identifier ze sent). The record MUST key on the egress identifier ze minted (`fwdPathIDs`), never on the source's received identifier. A destination without ADD-PATH names a path by prefix alone.
- [ ] `rfc/short/rfc4724.md` - Graceful Restart
  → Constraint: when the SOURCE restarts, its stale routes stay exported to destinations; the purge at End-of-RIB or timer expiry withdraws them, and that withdrawal MUST consult the record like any other.
- [ ] `rfc/short/rfc9494.md` - LLGR
  → Constraint: the LLGR egress filter converts a stale route to a withdrawal for an EBGP peer without LLGR (`mods.SetWithdraw`, the same conversion the gates use). That withdrawal MUST also consult and update the record.
- [ ] `rfc/short/rfc2918.md` and `rfc/short/rfc7313.md` - ROUTE-REFRESH and Enhanced Route Refresh
  → Constraint: RFC 2918 Section 4 re-advertises the Adj-RIB-Out "based on its outbound route filtering policy". Under RFC 7313 the receiver purges at EoRR every route the BoRR marked stale that was not re-sent, so after an enhanced refresh the record MUST equal exactly what was re-sent in the window.

**Key insights:** (minimal context to resume after compaction)
- The reactor already has a per-peer Adj-RIB-Out (`adjRIBOut`), but for the API origination rail only. The forward rails neither read nor write it.
- The rib plugin's `ribOut` is a second per-peer sent inventory, downstream of the send and across a plugin boundary.
- BIRD keeps one bit per (channel, route id) in `export_map`; FRR keeps one `bgp_adj_out` per (destination node, update subgroup, addpath tx id). Both withdraw only what the record says was sent.
- No dense per-path id exists in the reactor today: `fwdPathIDs` mints one id per path only for sources that frame ADD-PATH, and one id for a whole session otherwise.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `forwardUpdateSection`: per destination, RFC 1997 and RFC 7947 set a local withhold flag, which becomes `mods.SetWithdraw()` after `mods.Reset()`; a genuine egress policy reject (not a failed step) sets `SetWithdraw` and drops the wire override; `egressNextHopWithheld` sets it for the next-hop gates; then a single `IsWithdraw` branch calls `buildWithdrawalPayload` and queues the result. A failed filter step is a drop. The comment above the loop states the withdrawal is unconditional because the rail keeps no per-peer Adj-RIB-Out.
- [ ] `internal/component/bgp/reactor/forward_rs.go` - `reactorForwardRS`: the route-server rail, same gate set (RFC 1997, RFC 7947, egress reject, `egressNextHopWithheld`), same `buildWithdrawalPayload` conversion, same unconditional comment.
- [ ] `internal/component/bgp/reactor/forward_build.go` - `buildWithdrawalPayload`: converts the whole UPDATE into one withdrawal naming every route it announces AND every route it already withdraws, in one Withdrawn Routes field and at most one MP_UNREACH_NLRI. Refuses a source whose MP_UNREACH and MP_REACH name different families. It has no per-destination input: it cannot name a subset.
- [ ] `internal/component/bgp/reactor/forward_next_hop.go` - `egressNextHopWithheld`: the ordered next-hop gates both rails call; returns which gate refused.
- [ ] `internal/component/bgp/reactor/adj_rib_out.go` - `adjRIBOut`: per peer, family to wire-NLRI key to attribute signature; `record`, `forget` (called for every withdrawal whether or not the write succeeded), `reset` (session teardown), `withheld` counter. Bound stated as "one entry for each route this session has advertised and not withdrawn".
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `withdrawBatchFromPeers` and `splitOnAdvertised`: on the API rail a withdrawal is withheld only when the connection has advertised nothing at all (`hasAdvertised`, `withdrawBehindForwards`); a per-route check is not made there either.
- [ ] `internal/component/bgp/reactor/peer.go` - field `adjOut`, and `clearEncodingContexts`, which resets it on session teardown.
- [ ] `internal/component/bgp/reactor/forward_path_id.go` - `fwdPathIDTable`: the egress ADD-PATH identifier per ingress path. Unframed sources get one identifier per session (`bySource`); framed sources one per (source, received id, family, prefix key) (`byPath`), released on withdraw and on peer removal (not session down).
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `RIBManager.ribOut` and `handleSent`: the plugin's per-destination sent inventory, keyed per family by `ribOutKey` (prefix, path id, native key), fed from sent events, skipping replay and lifecycle events. Its comment says it stores routes "for replay on reconnect"; no clear on session down was found in `rib.go` (deletes occur on withdrawal, in `rib_structured.go` and `rib_sent_lifecycle.go`). Not traced further.
- [ ] `internal/component/bgp/plugins/rib/rib_sent_lifecycle.go` - GR/LLGR lifecycle reconciliation over `ribOut` against received ownership.
- [ ] `internal/component/bgp/reactor/forward_withhold_withdraw_test.go` and `draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go` - assert today's unconditional withdrawal at every gate on both rails.

**What per-peer sent state exists today:**

| State | Where | Rail | Keyed on | Cleared | Usable by the forward gates |
|-------|-------|------|----------|---------|------------------------------|
| `adjRIBOut` | reactor, `Peer.adjOut` | API origination only | family, NLRI wire bytes incl. ADD-PATH id | per route on withdraw, whole on session teardown | not today: the forward rails never record into it, so it is empty for forwarded routes |
| `hasAdvertised` / `withdrawBehindForwards` | reactor, `Peer` | API withdraw guard | the session, not the route | session teardown | no: one bit per session |
| `fwdPathIDs` | reactor, package global | both forward rails | ingress path | per path on relayed withdraw, per source on peer removal | it is an id allocator, not a sent record |
| `RIBManager.ribOut` | bgp-rib plugin | whatever the reactor reports as sent | destination, family, `ribOutKey` | per route on withdrawal | no: plugin boundary, fed after the send, absent when the plugin is not loaded |

**Behavior to preserve:** (unless the user explicitly said to change it)
- A destination that DOES hold a route a gate now refuses receives its withdrawal (the f93a30fbfc behavior, RFC 4271 Section 9.1.3).
- A failed filter step stays a drop and sends nothing.
- The two rails answer identically at every gate.
- The withdrawal of a source's own withdrawn routes still reaches every destination that holds them.
- The MP_UNREACH and MP_REACH single-attribute rule (RFC 7606 Section 3(g)) and the two-family refusal.
- The API rail's Section 9.2 suppression and its counters.

**Behavior to change:** (only what the user asked for)
- A destination is never sent a withdrawal for a route it was not sent on the current session: not by a withhold gate, not by a relayed source withdrawal, not by the LLGR conversion.
- A withdrawal that does go names only the routes the destination holds.

## Comparison with BIRD and FRR

Read from BIRD v2.19.0-48-g0de7b8235 (`nest/rt-table.c`, `nest/proto.c`,
`nest/protocol.h`) and FRR master (`bgpd/bgp_updgrp_adv.c`, `bgpd/bgp_route.c`,
`bgpd/bgp_advertise.h`), fetched 2026-10-04.

**BIRD.** Every channel carries `export_map`, a bitmap documented as "Keeps
track which routes passed export filter". The deciding function is
`rt_notify_basic`: it runs the export filter on the new route; then, if there
is an old route whose id is not set in the channel's `export_map`, it forgets
the old route; and if neither a new nor an old route remains, it returns
without notifying the protocol. So a rejected route whose predecessor was never
exported produces no message at all, and a rejected route whose predecessor was
exported produces a withdrawal of that predecessor. `do_rt_notify` maintains
the map: it clears the old route's bit and sets the new route's bit at the
moment it hands the change to the protocol. `rt_notify_accepted` and
`rt_notify_merged` apply the same test for the other export modes.

The ids are dense small integers: `rte_recalculate` takes the first zero bit of
the table's `id_map` for a new route and keeps the old id when a route is
replaced, and clears it when the route goes. So the per-channel cost is one bit
per route in the table, about 150 KB for 1.2 million routes. The map is
created at 1024 bits when the channel starts and reset by
`channel_stop_export`, which is the session-down path.

**FRR.** FRR keeps, per destination node, an RB tree of `struct bgp_adj_out`,
one per (update subgroup, addpath tx id), holding the advertised attribute.
`subgroup_process_announce_selected` calls `bgp_adj_out_unset_subgroup` when
the selected path fails `subgroup_announce_check` (the export policy), or when
nothing is selected. The deciding function is `bgp_adj_out_unset_subgroup`: it
looks up the adjacency for (node, subgroup, addpath tx id); only if one exists
AND it carries an advertised attribute does it queue a withdrawal on the
subgroup's withdraw FIFO; an adjacency with no attribute (a pending, never-sent
advertisement) is simply freed; no adjacency means nothing is queued. Memory is
amortized by update groups: peers with the same outbound policy share one
subgroup and so one adjacency per route.

**What this says for ze.**

| Question | BIRD | FRR | Consequence for the design |
|----------|------|-----|----------------------------|
| What is recorded | one bit per exported route | one adjacency with attributes per sent route | ze's forward rail needs membership only; the API rail additionally needs the signature for Section 9.2 |
| Key | table-wide dense route id | (node, subgroup, addpath tx id) | ze has no dense per-route id in the reactor today; one would have to be introduced, or the key is the wire NLRI as `adjRIBOut` uses |
| Granularity of sharing | per channel | per update subgroup | ze forwards per destination peer; sharing across peers with equal policy is out of scope unless the design needs it for memory |
| Reset | channel stop | subgroup peer removal | ze's `clearEncodingContexts` is the existing hook |
| Unknown state | not representable | not representable | a failed write in ze must leave the bit SET (peer may hold it) for the withdrawal question |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A received UPDATE from a source peer, forwarded by `ForwardUpdate` (reactor rail, driven by a plugin's forward command) or `reactorForwardRS` (route-server rail, on the source's read path).
- The API origination rails, initial sync, the `SendRoutes` transaction rail, refresh replay and LLGR lifecycle sends, each of which puts routes on a destination's wire.

### Transformation Path
1. Destination selection and source classification (both rails).
2. Per destination: the withhold gates set `mods.SetWithdraw`, or the route passes.
3. Today: `buildWithdrawalPayload` converts the whole UPDATE. Proposed (design open): the record is consulted per route; the withdrawal is built only for routes the destination holds; a destination holding none is skipped.
4. The per-peer forward pool queues the bytes; the writer puts them on TCP.
5. Proposed: the record is updated for every route announced to and withdrawn from that destination, in order with the send.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | the rib plugin learns sends from "sent" events; the forward command arrives from a plugin | No |
| Reactor rail ↔ per-peer forward pool | queued `fwdItem` carrying the built bytes | No |

### Integration Points
- `adjRIBOut` (`adj_rib_out.go`) - candidate home for the record, or the thing the record replaces (no-layering applies).
- `egressNextHopWithheld` and the two rails' gate branches - the consumers.
- `buildWithdrawalPayload` - must learn to emit a per-destination subset, or be bypassed when nothing is held.
- `fwdPathIDs` - the egress ADD-PATH id the key needs.
- `Peer.clearEncodingContexts` - the session-reset hook.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | design not done |
| No unintended coupling (components stay isolated) | No | design not done |
| No duplicated functionality (extends existing, does not recreate) | No | design must decide the relation to `adjRIBOut` and the rib plugin's `ribOut` |
| Zero-copy preserved where applicable (refs, not copies) | No | design not done; see R-2 |
| Registration over hardcoding, outbound | No | design not done |
| Registration over hardcoding, inbound | No | design not done |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The owner's decision covers the source's own relayed withdrawals, not only the gate conversions | BIRD's `rt_notify_basic` applies the same test to a plain withdrawal; RFC 4271 Section 4.3 ties a withdrawal to a previously advertised route | scope narrower: only gate conversions filtered | owner confirmation at design | unvalidated |
| A-2 | Every path that writes a route to a destination can record it | `update-building.md` names two paths (initial sync, `SendRoutes`) that do not record into `adjRIBOut` today | an unrecorded route is never withdrawn: a stuck prefix at the peer | grep of every UPDATE writer at design | unvalidated |
| A-3 | The record can be updated in order with the send on the per-peer forward pool | `forward_pool.go` per-peer workers | a race between a queued announce and a later gate decision withdraws nothing for a route that is about to arrive | design review of the pool ordering | unvalidated |
| A-4 | For a destination without ADD-PATH the key is the prefix alone | RFC 7911; FRR keys addpath tx id 0 in that case | a best-path change to a rejected path leaves the old one at the peer | unit test at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Memory: a per-peer set keyed on NLRI bytes costs tens of bytes per route; 1.2 million routes times 100 peers is several GB | `./le perf` RSS with a full table and many peers | a dense interned path id plus a per-peer bitmap, as BIRD does (about 150 KB per peer per 1.2 million routes); the id allocator is new code shared by both rails |
| R-2 | Zero-copy loss: today a relayed withdrawal and a converted withdrawal can share bytes across destinations; a per-destination subset forces a per-destination build | forward benchmarks (`forward_update_bench_test.go`) | build a subset only when membership differs from the whole UPDATE; skip the destination entirely when it holds nothing |
| R-3 | A record that misses a send never withdraws that route: a stale route at the peer until session reset | an interop scenario where a route is sent by initial sync then rejected by policy change | every UPDATE writer records (A-2); failed writes leave the bit set |
| R-4 | Three records of what a peer holds (`adjRIBOut`, the new record, the rib plugin's `ribOut`) drift | disagreement between `show` output and the wire | one fact declared once (`ai/rules/principles.md`): the design picks one reactor-side record; the rib plugin's inventory stays a replay source, or derives |
| R-5 | GR and LLGR: a stale route converted to a withdrawal for a non-LLGR peer must clear its bit; re-announced on source return must set it | `llgr_sent_lifecycle_test.go` style tests | route LLGR conversion through the same record update |
| R-6 | RFC 7313 Enhanced Route Refresh: routes in the record that are not re-sent before EoRR are purged by the receiver | `test/plugin/plugin-refresh.ci` | clear the family's record at BoRR, rebuild it from what is re-sent |
| R-7 | RFC 7606 treat-as-withdraw on INGRESS (a malformed UPDATE from a source becomes withdrawals) relays withdrawals downstream; those must also be filtered | not traced: where ze relays an ingress treat-as-withdraw | trace the ingress treat-as-withdraw relay at design |
| R-8 | Session reset must clear the record, or the next session's re-advertisement is withheld or its withdrawals skipped | reconnect tests | clear in `clearEncodingContexts`, as `adjRIBOut.reset` does today |
| R-9 | Policy change (soft reconfiguration outbound, `clear bgp rib out`): a route exported under the old policy and rejected under the new one must be withdrawn | config reload test | the record answers this directly; it is the case the record exists for |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a route the peer holds is never withdrawn (blackhole or leak at the peer), or a route is withheld on a new session |
| How is it reverted? | single commit revert restores unconditional withdrawal |
| Who else touches this path? | the forward rails (`reactor_api_forward.go`, `forward_rs.go`, `forward_next_hop.go`), the ADD-PATH and link-local specs in `plan/immediate/`, the rib plugin's sent lifecycle |

## Wiring Test (MANDATORY -- NOT deferrable)

Names are planned at skeleton and fixed at design.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| received UPDATE, export policy rejects, destination never held the route | → | reactor rail gate branch consulting the record | `TestForwardRejectSendsNothingForRouteNeverExported` (planned) |
| received UPDATE, export policy rejects, destination holds the route | → | reactor rail gate branch consulting the record | `TestForwardRejectWithdrawsRouteExportedEarlier` (planned) |
| same two cases on the route-server rail | → | `reactorForwardRS` gate branch | `TestRouteServerRejectWithdrawsOnlyExported` (planned) |
| `.ci` with an export filter over many UPDATEs | → | whole daemon | `test/plugin/bgp-export-filter-no-spurious-withdraw.ci` (planned) |

## Acceptance Criteria

Draft at skeleton; fixed at design.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | an export policy rejects every UPDATE for a prefix the destination was never sent | the destination receives no UPDATE for that prefix |
| AC-2 | a route was sent to the destination, then a later UPDATE for it is rejected by any withhold gate | the destination receives the withdrawal of that route, once |
| AC-3 | one UPDATE names routes the destination holds and routes it does not, and a gate refuses it | the withdrawal names only the held routes |
| AC-4 | the source withdraws a route the destination was never sent | the destination receives nothing for it |
| AC-5 | ADD-PATH destination: two paths for one prefix, one held | only the held (prefix, egress path id) is withdrawn |
| AC-6 | the destination session resets and re-establishes | the record is empty; the first rejected UPDATE sends nothing; the first accepted one is sent |
| AC-7 | LLGR: a stale route is converted to a withdrawal for a non-LLGR EBGP destination | the withdrawal goes once; a later withdrawal of the same route sends nothing |
| AC-8 | RFC 7313 refresh with a now-rejecting policy | after EoRR the record matches what was re-sent |
| AC-9 | both rails, every gate listed in Task | identical answers |
| AC-10 | full table, many filtered peers | memory per peer within a bound the design states and `./le perf` measures |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures an export filter towards a peer and receives a full table | wire, forward rail, gate, record, nothing on the wire | `test/plugin/bgp-export-filter-no-spurious-withdraw.ci` (planned) |
| 2 | tightens the export filter on a running session | reload, refresh out, gate, record, withdrawal of previously exported routes only | name at design |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| names at design | `internal/component/bgp/reactor/` | AC-1 to AC-9 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| interned path id (if the design adds one) | design decides | design decides | N/A | design decides |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-export-filter-no-spurious-withdraw` | `test/plugin/` | rejected routes never reach the peer as withdrawals | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-export-filter-withdraw-only-exported-frr` | `test/interop/scenarios/` | FRR | FRR's received-update counters show no withdrawal for a never-exported prefix, and one withdrawal for an exported prefix later rejected | |

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_forward.go` - gate branch consults the record
- `internal/component/bgp/reactor/forward_rs.go` - same
- `internal/component/bgp/reactor/forward_build.go` - per-destination withdrawal subset
- `internal/component/bgp/reactor/adj_rib_out.go` - extend or replace (design decides)
- `internal/component/bgp/reactor/peer.go` - reset hook
- `internal/component/bgp/reactor/forward_withhold_withdraw_test.go`, `draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go` - assertions move from "always withdraws" to "withdraws what was exported"
- `docs/architecture/bgp/structural-forwarding.md` - "Withheld routes are withdrawn"
- `docs/architecture/update-building.md` - the Adj-RIB-Out section, if the table changes

## Files to Create
- `test/plugin/bgp-export-filter-no-spurious-withdraw.ci` - functional test
- `test/interop/scenarios/bgp-export-filter-withdraw-only-exported-frr/` - interop scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no new config; behavior change only |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | N-A | none planned |
| CLI grammar (keyword before value) | N-A | none planned |
| Editor autocomplete | N-A | none planned |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | none planned |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | design decides | a per-peer count of withdrawals not sent would let an operator see the saving |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | behavior fix |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | design decides | rib plugin only if its inventory changes |
| 6 | Has a user guide page? | design decides | |
| 7 | Wire format changed? | No | the messages are the same shape, fewer of them |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` Section 9.1.3 row |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | design decides | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/update-building.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | design decides | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | design decides | |
| 16 | Any changed source file referenced by existing doc source anchors? | design decides | run `./le spec citation anchors` at design |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

Not written at skeleton. Phase 1 is wiring, as the template requires.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | no UPDATE writer escapes the record; a failed write leaves the route recorded as held |
| Data flow | one function decides "held" for both rails |
| Rule: no-layering | one reactor-side record, not `adjRIBOut` plus a second table beside it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| design not done | |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | the record is bounded by what the peer holds; a peer cannot grow it beyond the routes ze sends |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The API rail's `adjRIBOut` and the withdrawal question have opposite safe defaults for an unknown outcome: suppression must assume "not held", withdrawal must assume "held".
- The forwarding unit is the UPDATE message, the record's unit is the route. Every decision this spec adds sits at the seam between the two.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| open: record shape | per-peer set keyed on wire NLRI (as `adjRIBOut`); per-peer bitmap over a dense interned path id (as BIRD); shared adjacency per policy group (as FRR) | to be decided against R-1 and R-2 with a measured `./le perf` result |
| open: relation to `adjRIBOut` | extend it to the forward rails; replace it with one record serving both | no-layering forbids two |

## Known Limitations
- None recorded at skeleton.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The withdrawal decision quotes RFC 4271 Section 9.1.3 and Section 4.3 at the
function that decides "held".

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
