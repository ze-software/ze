# Spec: redistribute-late-join-replay

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-07-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/learned/634-bgp-redistribute.md` ("Open Question" central-store idea)
4. Precedent: `internal/component/bgp/plugins/rib/events/events.go:29,88-111` (ReplayRequest + Replay flag), `rib_bestchange.go:1102` (replayBestPaths)
5. Source: `internal/core/redistevents/events.go`, `internal/component/config/redistribute/{route.go,evaluator.go}`, `internal/component/config/loader_redistribute.go`, `internal/component/bgp/plugins/redistribute_egress/redistribute.go`, `internal/component/bgp/plugins/rib/{rib.go,rib_replay.go}`, `internal/component/bgp/reactor/peer_initial_sync.go`

## Task

Close a confirmed structural gap in the generic BGP redistribute path: a route
originated by a redistribute source (l2tp, connected, static-via-redistribute, and the
proposed as112 source in `spec-as112-bgp-redistribute`) is delivered ONLY to peers
present in the reactor's peer map at injection time. A peer that **first establishes
after** the injection (dynamic/passive/template peers accepted on inbound connect, or
peers added to config after the emit) never receives the route, because the redistribute
path has no prefix-keyed source-of-truth store to feed a genuinely new peer on
establishment.

Goal: redistribute-injected routes are replayed to any BGP peer that establishes later,
the same way received/best-path and config-static routes are, without regressing the
existing already-up-peer behavior or the value-typed/pool hot-path contract.

**Chosen mechanism: re-emit on request, modeled on `ribevents.ReplayRequest`.** Add a
`redistevents.ReplayRequest` typed event carrying an opaque `ReplayID` correlation token
(payload-carrying `events.Register[T]`, not the payload-less `RegisterSignal` `ribevents`
uses). The batch stays **peer-agnostic**: the orchestrator keeps the `ReplayID -> peer`
mapping; the producer only echoes the token. Producers (static, connected, l2tp today;
as112 when `spec-as112-bgp-redistribute` lands) re-emit their current `RouteChangeBatch`
set tagged with the `ReplayID` on receipt; the redistribute orchestrator fires the request
when a BGP peer establishes and directs the re-emitted (token-tagged) batches to that one
new peer, gated by the same `Accept` evaluator, and scoped to the sources named under
`destination bgp`. (Targeting/concurrency correlation is PINNED in "Key design point"; the
`ReplayID` generation token follows the rs `ReplayGen` precedent.) This reuses the existing
producer->orchestrator->consumer pipeline (one code path for snapshot + incremental)
rather than adding a parallel store or a generator-callback interface. See "Chosen
Approach" for the full rationale and the rejected alternatives.

**Scope: BGP consumer only.** OSPF and IS-IS redistribute consumers originate into the
flooded/synchronized link-state DB (IS-IS TLV 135 in the node's LSP set,
`isis/redistribute/consumer.go:186-192`; OSPF external LSAs), so a new adjacency receives
them via database exchange -- they do NOT have this gap. Only the BGP consumer sends
per-peer via `UpdateRoute` and bypasses the Adj-RIB-Out/best-path table that would feed a
new peer. The fix gives the BGP path the equivalent of the LSDB's built-in sync.

**Blocks:** `spec-as112-bgp-redistribute` (its R-6). That spec is not production-complete
until this lands. This gap already affects l2tp/connected today.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/learned/634-bgp-redistribute.md` - the redistribute egress pattern + the deferred central-store idea
  → Decision: the "Open Question" central-store model (engine-owned per-protocol store; producers push, consumers read by reference) is exactly the shape that feeds late joiners; this spec decides whether to adopt it.
- [ ] `docs/architecture/core-design.md` (redistribution + RIB sections) - where locally-injected routes live vs received/best-path routes
  → Constraint: locally-injected redistribute routes bypass `bestPrev`/Loc-RIB today; any fix must decide whether to keep that or route them through best-path replay.

### RFC Summaries (MUST for protocol work)
- [ ] N/A for the mechanism itself (no wire-format change); AS_PATH/attributes unchanged. Confirm during DESIGN.

**Key insights:**
- The gap is structural: point-in-time fan-out, per-peer-only `ribOut`, config-only initial sync, and (verified) NO core Loc-RIB->new-peer advertisement -- full-table replay to a new peer exists only in optional rs/rr from `adj-rib-in` (received-only). Redistribute is the one locally-originated class with no peer-up path.
- **Fix = re-emit on request**, mirroring `ribevents.ReplayRequest` (`rib/events/events.go:29,88-111`): orchestrator fires `redistevents.ReplayRequest{replayID}` on BGP peer-up; producers re-emit their current set tagged with the echoed `ReplayID`; orchestrator maps `replayID->peer` and injects to that target peer, destination-scoped. Reuses the existing `RouteChangeBatch` pipeline; no store, no new callback.
- The redistribute evaluator is destination-AGNOSTIC today (`ImportRule` drops destination); tightening it to per-destination is part of this spec (R-3).

## Current Behavior (MANDATORY)

**Source files read (agent trace, 2026-07-04):**
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `AnnounceNLRIBatch` snapshots `peers := a.getMatchingPeersSel(sel)` under lock (`:30-32`) -- only peers in the reactor map at call time. Established peers sent immediately; existing-but-unestablished peers get `peer.QueueAnnounce(...)` (`:141-151`). A peer not yet in the map gets neither. No persistent per-prefix store written here.
  → Constraint: fan-out is point-in-time; the only `OutgoingRIB` mention is the unrelated `SendRoutes` path (`:587`).
- [ ] `internal/component/bgp/plugins/rib/rib.go` - RIB plugin subscribes to `update direction sent` (~`:620`); `handleSent` stores `r.ribOut[peerAddr][fam][key]` (`:789-823`), per-peer. `ribOut[C]` exists only if C was actually sent the UPDATE. Locally-injected routes never enter `bestPrev`/Loc-RIB.
  → Constraint: `ribOut` is per-peer, populated only for peers that received a send.
- [ ] `internal/component/bgp/plugins/rib/rib_replay.go` - `collectGroupedRibOutRoutesFiltered(peerAddr, ...)` reads `r.ribOut[peerAddr]`, returns `nil` when nil (`:52-56`). A brand-new peer has no entry -> establishment replay yields nothing.
  → Constraint: peer-up replays only that peer's own prior ribOut (reconnect replay, not first-join).
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - `sendInitialRoutes` sends only config `StaticRoutes` (`:65-150`), `DefaultOriginate` (`:155`), `PluginRoutes` (`:326`), then drains opQueue. No dynamic replay of injected redistribute routes.
  → Constraint: initial sync is config-sourced.
- [ ] `internal/component/bgp/plugins/persist/server.go` - same shape: per-peer `ribOut` from `update direction sent` (`:168,236`); `replayForPeer` reads `ps.ribOut[peerAddr]`, empty for a new peer -> only EOR (`:608-623`).
  → Constraint: persist plugin does not close the gap either.
- [ ] `internal/component/bgp/plugins/watchdog/{watchdog.go,server.go}` - **the in-tree precedent for the fix.** Watchdog tracks `peerUp map[string]bool` (`server.go:26`), subscribes to `state` events (`watchdog.go:103`), and on peer establishment `handleStateUp` re-sends all announced routes to that single peer via the same text `UpdateRoute` boundary (`server.go:268-327`, "watchdog resent on reconnect").
  → Constraint: this is the precedent for the ORCHESTRATOR'S peer-up trigger -- the orchestrator adds the SAME `state` subscription and fires on the down->up edge. (Unlike the watchdog it does NOT keep a route store; it requests a re-emit from producers instead -- see the ReplayRequest entry below.)
  → Constraint: watchdog fires only on the down->up edge (`wasUp` check, `server.go:270,275`) so an already-up peer is not re-sent -- the orchestrator's trigger must use the same edge guard, and target only the triggering peer (selector = peer addr, not `*`).
- [ ] `internal/plugins/isis/redistribute/consumer.go` (and `internal/plugins/ospf/redistribute/consumer.go`) - **the gap is BGP-consumer-specific.** The IS-IS consumer's `InjectRoute` writes a TLV 135 entry into the node's own LSP set (`SetRedistPrefix`, `:186-187`) and re-originates (`Originate`, `:192`) so flooding carries it to peers (`:17-18`). OSPF originates external LSAs the same way.
  → Constraint: OSPF/ISIS redistribute routes live in the flooded/synchronized link-state DB, so a NEW adjacency receives them automatically via database exchange -- they do NOT have the late-join gap. This spec is therefore BGP-consumer-scoped: only the BGP consumer sends per-peer via `UpdateRoute` and bypasses the mechanism that would feed a new peer. The LSDB is OSPF/ISIS's built-in "source-of-truth store"; the fix gives the BGP path an equivalent.
- [ ] `internal/component/bgp/plugins/rib/rib.go` + `internal/component/bgp/plugins/rs/server_handlers.go` - **CRITICAL: Ze BGP is a forwarding engine; the best-path/Loc-RIB does NOT feed peers.** Core rib peer-up handler (`rib.go:1061,1077-1080`) does ONLY `collectGroupedRibOutRoutes(peerAddr)` = that peer's own (empty-for-new) `ribOut`. `bestPrev`/Loc-RIB are written only on the RECEIVED path (change-detection + FIB mirror), never advertised to a peer. Full-table replay to a NEW peer exists ONLY in the optional `rs`/`rr` plugins, sourced from the separate `adj-rib-in` store (received routes only): `rs/server_handlers.go:149-178` -> `adj-rib-in replay <peer> 0`, soft-dep no-op if `bgp-adj-rib-in` absent.
  → Constraint: **Approach C is INCOHERENT** -- routing redistribute routes "through the best-path RIB" would not reach any peer, because that table does not advertise to peers, and redistribute routes are not in it (`handleSent` writes only `ribOut`).
  → Constraint: **every route class that reaches a late peer has its OWN source-of-truth store/replay on peer-up**: config-static via `sendInitialRoutes`/`PeerSettings.StaticRoutes` (core, always on, `peer_initial_sync.go:23,79,131`); received via `adj-rib-in` replay (rs/rr, optional); watchdog via its pools (`watchdog/server.go:268-327`). Redistribute is the only locally-originated class with NO peer-up path. The fix must be always-on (in the redistribute path), like config-static, NOT dependent on rs/rr.
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - **THE PRECEDENT for the chosen mechanism: "ask a source to re-emit its data" already exists.** `EventReplayRequest = "replay-request"` -- "downstream consumer asking for full table replay" (`:29`); `ReplayRequest = events.RegisterSignal(...)` -- a payload-less SIGNAL event (`:108-111`); the producer answers by re-emitting its whole dataset flagged `Replay: true` (`BestChangeBatch.Replay`, `:88-92`), which the RIB does in `replayBestPaths` (`rib_bestchange.go:1102`). Used today for RIB->sysrib/FIB startup sync.
  → Decision: mirror this into `redistevents` -- a `ReplayRequest` event + a SINGLE `ReplayID uint64` field on `RouteChangeBatch` (0 = incremental, nonzero = replay for request N) + an `IsReplay()` helper; NO separate `Replay` bool (derivable, a consistency footgun). Use payload-carrying `events.Register[*ReplayRequest]` (carries the opaque `ReplayID` token, NOT the peer) rather than `ribevents`' payload-less `RegisterSignal` (`typed.go:143,215`): `ribevents` broadcasts to all consumers at startup, we target one new peer and correlate returning batches by token. See "new-peer targeting (PINNED)".
- [ ] `internal/component/config/redistribute/{route.go,evaluator.go}` + `internal/component/config/loader_redistribute.go` - **the evaluator is destination-AGNOSTIC today.** `ExtractRedistributeRules` drops the `destination` key, storing only `ImportRule{Source, Families}` (`loader_redistribute.go:66-70`). `Accept(route, importingProtocol)` (`route.go:34-42`) scopes only by source-match + loop-prevention (`route.Origin != importingProtocol`); it carries NO source->destination binding, so a source imported under `destination bgp` is also accepted by `Accept(route, "ospf")`.
  → Constraint: to fire generators/replay per destination (`destination bgp { import as112 }` must mean "feed BGP specifically"), `ImportRule` must record the destination. This tightening is part of this spec. Flag the current cross-destination acceptance as a defect to confirm/fix (`R-3`).
- [ ] `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - the orchestrator subscribes ONLY to producer `RouteChangeBatch` events today (`:139-149`), not to peer `state`. It must additionally subscribe to `state` (as the watchdog does, `watchdog.go:103`) to fire the replay-request on BGP peer-up.
  → Constraint: the orchestrator is the natural home for the peer-up trigger + new-peer targeting; producers stay peer-agnostic.

**Behavior to preserve:**
- Already-up-peer delivery, reconnect replay, config-static initial sync, and the value-typed/pool redistevents hot-path contract.

**Behavior to change:**
- A peer establishing for the first time after a redistribute injection receives the current redistribute route set.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Peer session establishment (first-time join of a dynamic/passive/config-added peer).

### Transformation Path
1. A BGP peer reaches Established -> reactor emits a `state` up event.
2. The redistribute orchestrator (subscribed to `state`) sees the down->up edge, allocates a monotonic `replayID`, records `replayID -> peer`, and emits `redistevents.ReplayRequest{replayID}` (for the sources under `destination bgp`). The peer stays orchestrator-side; the request carries only the opaque token.
3. Each producer (static/connected/l2tp; as112 when it lands) subscribed to `ReplayRequest` re-runs its enumeration and re-emits its current `RouteChangeBatch` set with `ReplayID = replayID` (echoes the token; never sees the peer).
4. The orchestrator receives the `ReplayID`-tagged batches, maps `replayID -> peer`, applies `Accept(route, "bgp")` (destination-scoped), and injects them to that ONE peer via `UpdateRoute(<peer>, ...)` (not `*`). An unknown/expired `replayID` is dropped with a warn.
5. Reactor advertises to that peer; already-up peers are untouched (they already have the routes from the original incremental emit).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| reactor peer-up -> orchestrator | `state` event subscription (watchdog precedent) | [ ] |
| orchestrator -> producers | `redistevents.ReplayRequest{ReplayID}` typed event (opaque token, NOT the peer) | [ ] |
| producers -> orchestrator | `RouteChangeBatch{ReplayID:N}` on the existing pipeline (producer echoes the token) | [ ] |
| orchestrator -> reactor | `UpdateRoute(<peer from ReplayID map>, ...)` (single-peer selector) | [ ] |

### Integration Points
- `internal/core/redistevents/` - new `ReplayRequest` typed event (payload `ReplayID`) + `ReplayID` field on `RouteChangeBatch` (mirrors `ribevents`, payload-carrying).
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - orchestrator gains a `state` subscription, peer-up trigger, and target-peer correlation of the replay batches.
- producers (`internal/plugins/static`, `internal/plugins/connected`, `internal/component/l2tp`) - subscribe to `ReplayRequest`, re-emit current set. (as112 joins as a producer in `spec-as112-bgp-redistribute`, which depends on this spec.)
- `internal/component/config/redistribute/{route.go,loader_redistribute.go}` - `ImportRule` records destination; replay + `Accept` scoped to `destination bgp`.

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality (reuses existing replay machinery where possible)
- [ ] Zero-copy preserved where applicable
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The late-join gap is fully described by the points above (fan-out, per-peer ribOut, config-only initial-sync, no core Loc-RIB->new-peer path) | two independent code traces 2026-07-04 | fix misses a delivery path | design review + late-join `.ci` per source | **confirmed** (both Current-Behavior traces enumerate every delivery path; late-join `.ci` per source is the implement-time proof) |
| A-2 | Correlation uses an opaque `ReplayID` token (via payload-carrying `events.Register[*ReplayRequest]`), echoed by the producer; the orchestrator holds `ReplayID -> peer` so the batch stays peer-agnostic | `typed.go:143,215` (SignalEvent payload-less) vs `Register[T]` (`:82`); rs `ReplayGen` precedent (`rs/server_handlers.go:151-158`) | targeting falls back to `*` (thundering herd) or breaks the peer-agnostic contract | PINNED in "new-peer targeting"; targeting + concurrency `.ci` | **confirmed (ReplayID token, `Register[T]`)** |
| A-3 | Producers (static, connected, l2tp) can cheaply re-enumerate their current set on demand | each holds a mutex-guarded current-set map (`static routeManager.routes`, `inject.go:36-41`; `connected routeObserver.prefixes`, `connected.go:45-50`; `l2tp subscriberRouteObserver.records`, `route_observer.go:59-64`, comment "future RIB injection path can read the live set") plus existing per-entry emit methods | re-emit is expensive or races live state | producer re-emit unit tests | **confirmed** (current-set map + emit method present in all three; re-emit = iterate under lock + emit tagged with the echoed `ReplayID`) |
| A-4 | Recording `destination` in `ImportRule` does not break existing redistribute `.ci` (BGP still accepts its imports) | additive field; `ImportRule` is `{Source, Families}` today (`route.go:24-26`), loader drops destination (`loader_redistribute.go:66-70`) | existing redistribute tests regress | run all `bgp-redistribute-*.ci`, `redistribute-l2tp-*.ci` | **confirmed** (zero-value `Destination`; loader populates from the existing `destination` key, `Accept` matches same-destination; full `.ci` regression is the implement-time proof) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Re-emit targeting leaks: replay batch for peer A delivered to peer B (or to `*`) -> thundering herd / wrong-peer routes | duplicate UPDATEs to up peers on every peer-up; `.ci` sees route on wrong peer | **RESOLVED (see "new-peer targeting" PINNED):** `ReplayID` token -> orchestrator maps to target peer, injects to that peer only; unknown-ID batch dropped. Assert single-peer targeting in `redistribute-late-join-targeting.ci` |
| R-2 | Concurrent peer-ups cross-deliver replay batches (async re-emit not correlated to requestor) | flaky multi-peer `.ci`; wrong route counts | **RESOLVED:** distinct `ReplayID` per peer-up (generation counter, rs `ReplayGen` precedent) -> no cross-delivery; bounded map + TTL for slow/async producers. Multi-peer concurrency `.ci` |
| R-3 | **Destination-agnostic evaluator (confirmed):** `import` under one destination is accepted by another (`route.go:34-42`, `loader_redistribute.go:66-70` drops destination) | `Accept(route,"ospf")` true for a bgp-only import in a unit test | record destination in `ImportRule`; scope `Accept` + replay per destination; regression test both directions |
| R-4 | Adding `ReplayID uint64` to `RouteChangeBatch` touches every producer/consumer struct literal | build/lint break in fib/fakeredist/l2tp | additive zero-value field (0 = incremental, existing behavior); grep all `RouteChangeBatch{` literals; run all redistribute `.ci` |
| R-5 | Re-emit of a large table (static/connected) on every peer-up is a per-peer cold-path cost | latency spike on peer-up with big tables | same order as `sendInitialRoutes` / rs full-replay already pay; bound + measure; not worse than status quo for those classes |
| R-6 | **Out-of-process producer replies async** (`typed.go:97-99`): dropping `replayID->peer` right after `Emit` discards its late replay batch via the unknown-ID drop -> the new peer silently misses that source's routes | new peer missing an out-of-process source's routes; `.ci` with an external fake producer joining after injection | hold the `replayID->peer` mapping until a TTL sized to the slowest producer; apply the drop-immediately-after-`Emit` fast path ONLY when all subscribed producers are in-process (as112 and future external producers make async the norm, so default to TTL) |

## Wiring Test (MANDATORY)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP peer establishes after a redistribute injection | → | orchestrator `state`-up -> `ReplayRequest` -> producer re-emit -> targeted `UpdateRoute(<peer>)` | `redistribute-late-join.ci` |
| producer receives `ReplayRequest{replayID}` | → | producer re-emits current `RouteChangeBatch{ReplayID:N}` | `TestProducerEchoesReplayIDOnReplayRequest` (per producer) |
| `destination bgp { import X }` only | → | `Accept(route,"bgp")` true, `Accept(route,"ospf")` false | `TestImportRuleDestinationScoped` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | redistribute source injects a route (via `destination bgp { import <src> }`), then a NEW peer (dynamic or config-added) establishes | the new peer receives the injected route (via ReplayRequest -> re-emit -> targeted inject) |
| AC-2 | already-up peers at injection time | unchanged delivery; NOT re-sent to on every other peer's establishment (targeting = new peer only) |
| AC-3 | reconnect of a previously-sent peer (incl. rapid flap) | receives the current set once on re-establishment; a fresh `replayID` per peer-up supersedes any older in-flight replay for the same peer (rs `ReplayGen` supersession precedent: `rs/peer.go:14`, `rs/server_handlers.go:169`), so no duplicate/parallel replay |
| AC-4 | source withdraws the route before the new peer joins | the new peer does NOT receive it (re-emit reflects current live state) |
| AC-5 | a source imported under `destination bgp` only (no ospf import) | `Accept(route, "bgp")` true; `Accept(route, "ospf")` for the same source is NOT true solely because of the bgp import (destination-scoped, R-3 fix). (as112-specific re-emit-reaches-BGP delivery lives in `spec-as112-bgp-redistribute`.) |
| AC-6 | producer receives `ReplayRequest{replayID}` | re-emits its current `RouteChangeBatch` set tagged with the echoed `ReplayID`; a producer not yet updated ignores the event with no behavior change |
| AC-7 | two peers establish concurrently | each receives the current set; no cross-delivery of one peer's replay batch to the other |
| AC-8 | existing redistribute producers (l2tp/connected) with no ReplayRequest handler | wire output unchanged (additive `ReplayID` defaults 0 = incremental; event ignored) |

## End-to-End User Stories (MANDATORY for new features)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Adds a dynamic peer after enabling a redistribute source | peer up -> replay -> UPDATE | `redistribute-late-join.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReplayRequestEventRegistered` | `internal/core/redistevents/*_test.go` | `ReplayRequest` typed event + `ReplayID uint64` field exist; zero-value `ReplayID=0` | |
| `TestRouteChangeBatch_ReplayIDBackCompat` | `internal/core/redistevents/*_test.go` | existing producers' batches default `ReplayID=0`; no wire change | |
| `TestImportRuleDestinationScoped` | `internal/component/config/redistribute/route_test.go` | `import` under `destination bgp` -> `Accept(_,"bgp")` true, `Accept(_,"ospf")` false (R-3) | |
| `TestOrchestratorFiresReplayOnPeerUp` | `internal/component/bgp/plugins/redistribute_egress/*_test.go` | down->up edge allocates a `replayID`, records `replayID->peer`, emits `ReplayRequest` for `destination bgp` sources only | |
| `TestOrchestratorTargetsNewPeerOnly` | `internal/component/bgp/plugins/redistribute_egress/*_test.go` | `ReplayID`-tagged batches injected to the mapped peer, not `*`; distinct `replayID` per concurrent peer-up -> no cross-delivery; unknown `replayID` dropped (R-1/R-2) | |
| `TestProducerEchoesReplayIDOnReplayRequest` (static/connected/l2tp) | each producer `*_test.go` | re-emits current set with `ReplayID` echoed from the request; unhandled event = no-op | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ReplayID` | 0 = incremental; nonzero = replay | monotonic uint64; wraps are not reused within TTL window | 0 is "not a replay" (valid) | N/A (uint64) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-late-join` | `test/plugin/*.ci` | peer connects AFTER a redistribute injection, still gets the route | |
| `redistribute-late-join-withdrawn` | `test/plugin/*.ci` | route withdrawn before peer joins -> new peer does not get it (AC-4) | |
| `redistribute-late-join-targeting` | `test/plugin/*.ci` | second peer joining does not re-send to the first peer (AC-2) | |
| `redistribute-existing-producers-unchanged` | reuse `bgp-redistribute-*.ci` | l2tp/connected wire output unchanged (AC-8) | |

## Files to Modify
- `internal/core/redistevents/events.go` - add `ReplayRequest` typed event (`events.Register[*ReplayRequest]`, payload = `ReplayID uint64`) + a SINGLE `ReplayID uint64` field on `RouteChangeBatch` (0 = incremental, nonzero = replay-for-request-N) plus an `IsReplay() bool` helper (`return b.ReplayID != 0`). NO separate `Replay bool` -- it is derivable from the ID and a second field is a consistency footgun. Mirrors `rib/events/events.go:88-111` but payload-carrying (not `RegisterSignal`) and single-field (ribevents' bare `Replay` bool fits its untargeted broadcast; ours needs the ID).
- `internal/component/config/redistribute/route.go` - `ImportRule` gains `Destination`; `Accept` scoped per destination (R-3)
- `internal/component/config/loader_redistribute.go` - record `dest.Key` into `ImportRule.Destination` (currently dropped, `:66-70`)
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - subscribe to `state`; on down->up allocate `replayID`, map `replayID->peer` (bounded + TTL), emit `ReplayRequest{replayID}` for `destination bgp` sources; on a `ReplayID`-tagged batch, look up the peer and inject via `UpdateRoute(<peer>)`; drop unknown IDs
- `internal/plugins/static/inject.go` - subscribe to `ReplayRequest`, re-emit current kernel-static set tagged with the echoed `ReplayID`
- `internal/plugins/connected/connected.go` - subscribe to `ReplayRequest`, re-emit current interface-address set (echoed `ReplayID`)
- `internal/component/l2tp/route_observer.go` - subscribe to `ReplayRequest`, re-emit current session routes (echoed `ReplayID`)
- (NOT modified here) the as112 producer's `ReplayRequest` handler lands in `spec-as112-bgp-redistribute` (which `Depends` on this spec). This spec wires only the three producers that exist today: static, connected, l2tp.
- doc: `docs/architecture/core-design.md` (redistribution replay-on-request), + the route-origination-model doc (bgp{} advertise-only vs system-known, shared with as112 spec)

## Files to Create
- `test/plugin/redistribute-late-join.ci`, `redistribute-late-join-withdrawn.ci`, `redistribute-late-join-targeting.ci` - the missing late-join coverage (no such test exists today for any redistribute source)

## Implementation Steps

### Chosen Approach: re-emit on request (`redistevents.ReplayRequest`)

**Decision (verified architecture + user direction, 2026-07-04):** extend the redistribute
protocol with the same "consumer asks a source to re-emit its data" mechanism the RIB
already exposes for sysrib/FIB (`ribevents.ReplayRequest` + its `Replay`-flagged batch). On a
BGP peer's down->up edge, the orchestrator allocates a `replayID`, records `replayID->peer`,
and emits `redistevents.ReplayRequest{replayID}` (the peer stays orchestrator-side; the
request carries only the opaque token); each producer re-runs its enumeration and re-emits its
current `RouteChangeBatch` set tagged with the echoed `ReplayID`; the orchestrator maps the ID
back to the peer, applies the destination-scoped `Accept` filter, and injects to that one peer
via `UpdateRoute(<peer>, ...)`. Reuses the existing `RouteChangeBatch` pipeline (snapshot and
incremental are the same code path, differing only in `ReplayID` and the target), matches an
established in-tree pattern, and requires no synced store and no new callback interface. Producers already
know how to enumerate (startup/change) -- "re-emit on request" is that enumeration run
again on demand.

**Why this over the alternatives considered during design:**
- **vs. a redistribute-owned store + peer-up replay ("Approach B"):** no second source of
  truth to keep synced (add/remove bookkeeping, staleness). The producer re-enumerates.
- **vs. a generator-callback registry:** no new abstraction; reuses `RouteChangeBatch` and
  the `events.Register[T]` primitive, both existing. (The generator model is behaviorally equivalent;
  re-emit is the more Ze-consistent implementation of it.)

**Rejected:**
- **Route through best-path/Loc-RIB ("Approach C") -- INCOHERENT.** Verified that Ze's
  best-path/Loc-RIB does not advertise to peers (change-detection + FIB mirror only;
  `rib.go` handleSent writes only `ribOut`), and redistribute routes are not in it; it would
  reach no peer. (Recommended earlier in error on a false "Loc-RIB feeds new peers" premise;
  corrected after the new-peer-advertisement trace.)
- **Engine-owned per-protocol central store now -- deferred as premature** (634 guidance:
  build the shared store only when a second consumer makes it pay).

### Key design point: new-peer targeting (PINNED -- R-1/R-2)

**The problem:** the orchestrator emits `ReplayRequest`; producers re-emit
`RouteChangeBatch`, which is the generic, **peer-agnostic** producer payload (the returning
batch carries no peer, and MUST NOT -- tagging it with a peer would break the producer
contract). So the orchestrator must correlate each returning replay batch back to the peer
whose establishment triggered the request, and concurrent peer-ups must never cross-deliver.
Correlation therefore lives in the orchestrator, not the producer.

**The mechanism (chosen): an opaque `ReplayID` correlation token, echoed by the producer.**
- Add `ReplayID uint64` to `RouteChangeBatch` (0 = normal incremental; nonzero = a replay
  keyed to one request). It is a value type and **NOT a peer** -- an opaque token the producer
  echoes blind, so the peer-agnostic contract holds (the producer never learns the peer).
- Orchestrator, on a BGP peer down->up edge: allocate a monotonic `replayID` (generation
  counter -- in-tree precedent: rs `ReplayGen`, `rs/server_handlers.go:151-158`), record
  `replayID -> targetPeer` in a bounded map, then emit `ReplayRequest{replayID}`.
- Producers, on `ReplayRequest{replayID}`: re-emit their current set with `batch.ReplayID =
  replayID` (echo the token). No peer knowledge.
- Orchestrator's `RouteChangeBatch` handler: `ReplayID == 0` -> incremental -> `UpdateRoute("*")`
  (unchanged); `ReplayID == N` -> look up `targetPeer`, `Accept(route,"bgp")`,
  `UpdateRoute(<targetPeer>)`. A batch with an unknown/expired `ReplayID` is dropped with a
  warn -- never mis-delivered.
- **Concurrency:** each peer-up gets a distinct `replayID`, so concurrent replays never
  cross-deliver; no serialize/single-flight bottleneck needed.
- **Lifecycle (see R-6):** engine (in-process) subscribers deliver synchronously, but
  plugin-process producers deliver ASYNC (`typed.go:97-99` -- "engine subscribers always
  deliver synchronously", plugin-process counted separately). So "drop `N->peer` right after
  `Emit` returns" is safe ONLY when every subscribed producer is in-process. With an
  out-of-process producer, its replay arrives AFTER `Emit`, hits the unknown-ID drop, and
  those routes silently never reach the peer (missing routes, NOT harmless). Therefore: hold
  `N->peer` until a TTL sized to the slowest producer (default), and apply the
  drop-immediately-after-Emit fast path ONLY when no out-of-process producer is subscribed.
  Under that rule the unknown-ID drop only ever discards a genuinely stale batch (one
  superseded by a newer `replayID` for the same peer), which IS harmless.

This resolves R-1 (targeting) and R-2 (concurrency) via a token the producer echoes, keeping
the batch peer-agnostic. Rejected alternative: serialize replays per peer (single-flight) --
correct but a bottleneck on a reconnect burst, and it relied on synchronous-`Emit`-as-completion
which `typed.go:97-99` shows breaks for out-of-process producers; the token approach does not.

### Implementation Phases
1. **Phase: redistevents ReplayRequest + ReplayID** -- add the `ReplayRequest` typed event (payload `ReplayID uint64`) + `ReplayID uint64` on `RouteChangeBatch`, mirroring `ribevents` but payload-carrying. Existing producers ignore the event (ReplayID stays 0, no behavior change) until updated.
2. **Phase: destination-scoped ImportRule** -- record `destination` in `ImportRule`; `Accept` (and the replay trigger) scoped per destination protocol. Fix/confirm the cross-destination acceptance defect (R-3).
3. **Phase: orchestrator peer-up trigger + targeting** -- subscribe to `state`, on down->up allocate `replayID` + map to peer (bounded + TTL, R-6), emit `ReplayRequest{replayID}` for `destination bgp` sources, correlate returning `ReplayID`-tagged batches back to the peer and inject via `UpdateRoute(<peer>)`; drop unknown IDs.
4. **Phase: producer re-emit** -- static, connected, and l2tp re-emit their current set on `ReplayRequest`. (The as112 producer gains the same subscription in `spec-as112-bgp-redistribute`.)
5. **Phase: functional + late-join tests** -- the missing coverage (peer joins after injection).

## Integration Checklist
| Integration Point | Needed? | File / Note |
|-------------------|---------|-------------|
| YANG schema (new config) | No | no new leaf; reuses existing `redistribute { destination bgp { import <src> } }` grammar |
| YANG validation constraints | No | `ImportRule.Destination` is populated from the already-validated `destination` key |
| CLI commands/flags | No | no new verb; replay fires automatically on peer-up |
| CLI grammar (action before identifier) | N/A | no new command grammar |
| Editor autocomplete | N/A | no new leaf |
| Functional test for new behavior | Yes | `test/plugin/redistribute-late-join*.ci` |
| Env var registration | N/A | no new env var |
| Doctor check for runtime dependencies | No | adds no runtime dependency (no file/socket/port/module/cert/binary); uses the existing EventBus + reactor `UpdateRoute` boundary |
| Prometheus counters/metrics | Yes | add `ze_bgp_redistribute_replay_total{source}` (replays fired on peer-up) so late-join delivery is observable; register in telemetry |
| Registration over hardcoding | Yes | `ReplayRequest` via `events.Register[*ReplayRequest]`; producers subscribe via their existing registration; orchestrator adds a `state` subscription like the watchdog; no core switch |

## Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (redistribute row `:16` -- note routes replay to peers that establish after injection) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (`:619` -- `import <src>` under a `destination` is now destination-scoped: an import under `destination bgp` no longer satisfies `Accept(_, "ospf")`) |
| 3 | CLI command added/changed? | No | grep `docs/guide/command-reference.md` -- no new verb |
| 4 | API/RPC added/changed? | No | reuses `UpdateRoute`; no new RPC |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (`:355` -- redistribute sources now replay their current set to newly-establishing peers) |
| 6 | Has a user guide page? | No | redistribute lives in `configuration.md`; no dedicated page |
| 7 | Wire format changed? | No | no wire change; AS_PATH/attributes unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `redistevents` gains `ReplayRequest` + `ReplayID` field -> `ai/rules/plugin-design.md` and the process-protocol doc (producers may be external processes; async delivery + TTL correlation per R-6) |
| 9 | RFC behavior implemented? | No | internal delivery mechanism; no RFC behavior |
| 10 | Test infrastructure changed? | Maybe | if a fake-producer re-emit path is added for `.ci`, note in `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Maybe | `docs/comparison.md` (`:131` redistribute row) if late-join delivery is a comparison point |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (`:604` "replay-on-new-peer invariants" -- add redistribute replay-on-request) |
| 13 | Route metadata keys added/changed? | No | no meta keys |
| 14 | Prometheus counters added/changed? | Yes | `ze_bgp_redistribute_replay_total` -> telemetry doc |
| 15 | Registered plugin/source/producer changed? | No | no new source; existing producers gain a `ReplayRequest` subscription |
| 16 | Changed source referenced by doc source anchors? | Yes | grep `docs/` anchors on `redistribute_egress/redistribute.go`, `redistevents/events.go`, `config/redistribute/route.go`, `loader_redistribute.go`; update stale claims |
| 17 | Existing docs show config/CLI examples for this area? | Yes | verify redistribute examples in `configuration.md` / `quickstart.md:125` still valid under destination-scoping |

## Discovery (Mechanical Checklist)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md` redistribute row; `docs/architecture/core-design.md` "replay-on-new-peer invariants" (`:604`) |
| What rule prevents regression? | back-compat AC-8 + `TestRouteChangeBatch_ReplayIDBackCompat`; the value-type/pool contract (`ai/rules/memory-architecture.md`) |
| What registry/inventory prevents drift? | the `redistevents` producer registry (`registry.go`) + `redistribute.RegisterSource`; a new producer subscribing to `ReplayRequest` is discovered the same way |
| What verification proves it? | `redistribute-late-join*.ci` + the per-producer re-emit unit tests |

## Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has implementation with `file:line` |
| Correctness (targeting) | Replay targets ONLY the triggering peer (single-peer selector, never `*`); the down->up edge guard prevents re-send to already-up peers (R-1) |
| Concurrency | Concurrent peer-ups do not cross-deliver; each `ReplayID`-tagged batch is correlated to its request's target peer via the ID (R-2) |
| Liveness | Re-emit reflects CURRENT live state -- a route withdrawn before the peer joins is NOT replayed (AC-4) |
| Back-compat | `ReplayID` defaults 0 (incremental); a producer with no `ReplayRequest` handler is unchanged on the wire (AC-8, R-4) |
| Destination scoping | `Accept` is destination-scoped; an `import` under `destination bgp` does NOT satisfy `Accept(_, "ospf")` (R-3) |
| Async delivery / lifecycle | out-of-process producers deliver async (`typed.go:97-99`); the `replayID->peer` map is held for a TTL, not dropped right after `Emit`, so an async producer's late replay still reaches the peer (R-6) |
| Value-type/pool contract | single `ReplayID uint64` value field (+ `IsReplay()` helper, no separate bool); no per-event alloc added; batch released after Emit; subscribers do not retain the payload past dispatch |
| Registration over hardcoding | `ReplayRequest` registered via `events.Register[T]`; orchestrator `state` subscription mirrors the watchdog; no core switch |
| Naming | no producer-specific spelling in generic `redistevents`/orchestrator; `ReplayID`/`ReplayRequest` mirror `ribevents` (single field, not a bool) |
| Data flow | the orchestrator is the only peer-aware layer; producers stay peer-agnostic (re-emit their whole set; the orchestrator targets the peer) |
| Tests | late-join `.ci` proves delivery to a peer joining after injection; targeting `.ci` proves no re-send to the first peer |

## Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `redistevents.ReplayRequest` + `ReplayID` field + `IsReplay()` | `TestReplayRequestEventRegistered`, `TestRouteChangeBatch_ReplayIDBackCompat` pass |
| destination-scoped `ImportRule` | `TestImportRuleDestinationScoped` pass (both directions) |
| orchestrator peer-up trigger + targeting | `TestOrchestratorFiresReplayOnPeerUp`, `TestOrchestratorTargetsNewPeerOnly` pass |
| producer re-emit (static/connected/l2tp) | `TestProducerEchoesReplayIDOnReplayRequest` per producer pass |
| late-join delivery | `redistribute-late-join.ci` pass |
| withdrawn-before-join | `redistribute-late-join-withdrawn.ci` pass |
| targeting (2nd peer not re-sent to 1st) | `redistribute-late-join-targeting.ci` pass |
| existing producers unchanged | `bgp-redistribute-*.ci`, `redistribute-l2tp-*.ci` pass |
| replay observability | `ze_bgp_redistribute_replay_total{source}` increments on peer-up replay |

## Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Cross-session leakage | a `ReplayID`-tagged batch is delivered only to the peer its `replayID` maps to; a peer never receives another peer's targeted replay (R-1/R-2) |
| Resource exhaustion / DoS | the bounded resource is the `replayID->targetPeer` map; cap its size and TTL-evict stale entries (R-6), and drop unknown/expired IDs, so peer flapping / rapid reconnect cannot grow it without bound; a fresh `replayID` supersedes an older in-flight replay for the same peer (rs `ReplayGen`); re-emit itself is bounded by the producer's current-set size, no worse than `sendInitialRoutes` / rs full-replay (R-5) |
| Input validation | `destination`/`source` names are validated by the existing loader against the registry; no new untrusted input path; a `ReplayID` not in the orchestrator's map is dropped (an external producer cannot inject to an arbitrary peer by fabricating an ID) |
| Use-after-release | the batch is consumed within dispatch (in-process synchronous; out-of-process copied on decode); `ReleaseBatch` zeroes after Emit; no pointer retained across the plugin boundary |
| Filter bypass / route leak | the replay path honors the SAME destination-scoped `Accept` filter as the live path; destination-scoping prevents an import meant for one protocol reaching another (R-3) |

## Known Limitations
- Until this ships, `spec-as112-bgp-redistribute` and existing redistribute sources (l2tp/connected) do not deliver to late-joining peers.
- **Config-add of a `destination bgp` while a peer is already up does not replay to that peer** (the trigger is the peer's down->up edge, not a config change). The peer marks up before the `HasDestination` gate, so a later config-add fires no new replay until the peer flaps. This is a config-reload concern, out of scope for peer-up late-join; a config reload re-applies routes through the normal path. (Adversarial review, area 4.)
- **Mass replay is synchronous on the orchestrator's event-delivery goroutine:** the replay issues one `UpdateRoute` RPC per route inline in the peer-up handler, so a very large live set head-of-line-blocks other peers' state handling during that peer's replay. This matches the incremental path and the watchdog's `handleStateUp`, and is the R-5 accepted cost ("not worse than status quo for those classes"). An async per-peer replay worker (rs `replayForPeer` precedent) is a future optimization if mass-reconnect latency becomes a problem. (Adversarial review, resource concern.)

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The targeting `.ci` (AC-2/R-1) could prove "replay goes ONLY to the triggering peer" by observing that an already-up peer does NOT receive a duplicate | The reactor SUPPRESSES a duplicate announce to a session that already holds the route, so `UpdateRoute("*")` vs a targeted `UpdateRoute(<peer>)` is WIRE-INDISTINGUISHABLE for an already-up peer | Mutation test: forcing `""` (→ `"*"`) in `handleReplayBatch` did NOT fail `redistribute-late-join-targeting.ci` | Per-peer targeting isolation is NOT functionally observable; it is guarded by the unit test `TestOrchestratorTargetsNewPeerOnly` (inspects `RouteEntry.Peer`). The `.ci` was reframed as a multi-peer replay-coexistence test with a TARGETING NOTE. This mitigates R-1 (an already-up peer cannot receive spurious duplicates even under a `*` fan-out). Deviation from Goal Gate "targeting `.ci` proves R-1". |
| A late-join `.ci` could use a genuinely dynamic/inbound peer (not in the reactor map at injection) | Not expressible in the check-mode harness: `ze-peer` dials in only under `--mode inject` (no per-message expect), and a dynamic-only group opens no listener | Explore-agent trace of `internal/test/peer/*` + `reactor.go` listener setup | Used the reconnect isolation (`tcp_connections=2`, no RIB plugin, no config route) instead — connection B can only receive the fakeredist route via replay. Precedent: `watchdog-reconnect.ci`, `adj-rib-in-replay-on-peerup.ci`. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Inject before a CONFIGURED peer establishes to test late-join | A configured-but-unestablished peer gets `QueueAnnounce`, so the queue (not the replay) delivers on establishment -- does not isolate the replay path | Reconnect isolation: inject AFTER conn=1 establishes (incremental), then the peer reconnects (conn=2) where only the replay can resend a non-config route with no RIB plugin |

## Review Gate

### Run 1 (adversarial review of replay/concurrency, 8 scrutiny areas)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `evictLocked` would spin if `maxSize==0` (unreachable; hardcoded 4096) | `replay.go:181` | FIXED: added `c.maxSize > 0` guard |
| 2 | NOTE | `peerUp[peer]=true` set before the `HasDestination` gate: a config-add of `destination bgp` does not replay to an already-up peer until it flaps | `replay.go:111,116` | Documented as Known Limitation (config-reload concern, out of peer-up scope) |
| 3 | NOTE | Mass replay issues N synchronous `UpdateRoute` RPCs on the delivery goroutine (head-of-line blocking) | `redistribute.go:238-245`, `consumer.go:38` | Documented as Known Limitation (R-5 accepted cost; matches incremental/watchdog; async worker deferred) |
| 4 | NOTE | Closing debug log counts `len(b.Entries)` incl. skipped removes; `replayTotal` per-entry | `replay.go:246` | Cosmetic; left as-is |

Areas with NO defect: deadlock/reentrancy (Emit outside lock; onPeerUp runs on the plugin delivery goroutine, not the reactor emit goroutine), lookupTarget no-delete (monotonic gen, no ID recycle), pool hygiene (ReplayID cleared on acquire+release), destination scoping (empty=agnostic, loader populates, Rules() deep-copies), handleReplayBatch (nil coord safe, unknown ID dropped, add-only, loop-prevention), goroutine ordering (coord.mu/evaluator RWMutex/atomics only).

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE  (adversarial review: 0 BLOCKER, 0 ISSUE, 4 NOTE)
- [ ] All NOTEs recorded above (or explicitly "none")  (4 NOTEs recorded; 1 fixed, 2 documented as limitations, 1 cosmetic)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/redistevents/events.go` | Yes | `ReplayRequest`/`ReplayRequestEvent`/`ReplayID`/`IsReplay` added |
| `internal/component/bgp/plugins/redistribute_egress/replay.go` | Yes | new file: `replayCoordinator`, `handleReplayBatch` |
| `internal/component/bgp/plugins/redistribute_egress/replay_test.go` | Yes | `TestOrchestratorFiresReplayOnPeerUp`, `TestOrchestratorTargetsNewPeerOnly`, `TestOrchestratorReplaySkipsRemoveEntries`, `TestOrchestratorIncrementalUnchangedByReplayID` |
| `internal/core/redistevents/replay_test.go` | Yes | `TestReplayRequestEventRegistered`, `TestRouteChangeBatch_ReplayIDBackCompat` |
| `internal/test/plugins/fakeredist/store.go` | Yes | new file: current-set + `reemitAll` |
| `test/plugin/redistribute-late-join.ci` | Yes | PASS |
| `test/plugin/redistribute-late-join-withdrawn.ci` | Yes | PASS |
| `test/plugin/redistribute-late-join-targeting.ci` | Yes | PASS |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | new peer (fresh down->up edge) receives injected route via replay | `redistribute-late-join.ci` PASS (conn B has no config route / no RIB plugin, so only replay can deliver) |
| AC-2 | already-up peer not disturbed; replay coexists | `redistribute-late-join-targeting.ci` PASS; strict per-peer targeting via `TestOrchestratorTargetsNewPeerOnly` (see Mistake Log: reactor dedup makes `*` wire-indistinguishable) |
| AC-3 | reconnect gets current set; fresh replayID supersedes | `redistribute-late-join.ci` (conn B is a reconnect); `TestOrchestratorFiresReplayOnPeerUp` (down->up re-fires after onPeerDown) |
| AC-4 | route withdrawn before join not replayed | `redistribute-late-join-withdrawn.ci` PASS; `TestFakeredistReemitsReplayID`, `TestOrchestratorReplaySkipsRemoveEntries` |
| AC-5 | destination-scoped Accept | `TestImportRuleDestinationScoped` (both directions) |
| AC-6 | producer echoes ReplayID on ReplayRequest | `TestProducerEchoesReplayIDOnReplayRequest` = `TestConnectedReemitsReplayID`/`TestStaticReemitsReplayID`/`TestRouteObserverReemitsReplayID`/`TestFakeredistReemitsReplayID` |
| AC-7 | concurrent peer-ups: no cross-delivery | `TestOrchestratorTargetsNewPeerOnly` (distinct replayID per peer, unknown ID dropped) |
| AC-8 | producers unchanged on wire (ReplayID defaults 0) | `TestRouteChangeBatch_ReplayIDBackCompat`; all 18 existing redistribute `.ci` PASS |

### Verification summary
- `make ze-lint-changed`: PASS (0 issues).
- Unit tests for all changed packages: PASS (redistevents, config/redistribute, config, bgp/redistribute, redistribute_egress, static, connected, l2tp, fakeredist).
- `make ze-unit-test` (full, `-race`): fails ONLY on the pre-existing load-sensitive race `l2tp.TestPeerTeardownWithdrawsSubscriberRoute` (`plan/known-failures.md:76`; racing fields untouched by this spec; PASS on 3/3 isolated re-runs).
- Functional: 3 new late-join `.ci` PASS; 18 existing redistribute `.ci` PASS (no regression, A-4).
- `make ze-doc-test`: source-anchor check PASS (all references valid); 2 pre-existing count drifts (`functional-tests.md` 17->18 FIXED here; `docs/DESIGN.md` interop 69->96 untouched/unrelated).

### Assumptions Resolved
| ID | Status | Evidence |
|----|--------|----------|
| A-1 | confirmed | late-join `.ci` per delivery path; no missed path found |
| A-2 | confirmed | `ReplayID` token via `events.Register[*ReplayRequest]`; `TestReplayRequestEventRegistered` (not a signal) |
| A-3 | confirmed | all four producers re-enumerate current set: `reemitAll` tests pass |
| A-4 | confirmed | `TestRouteChangeBatch_ReplayIDBackCompat` + all 18 existing redistribute `.ci` PASS |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Late-join `.ci` proves delivery to a peer that joins after injection
- [ ] Targeting `.ci` proves a second peer-up does not re-send to the first (R-1)
- [ ] Destination-scoping regression test both directions (R-3)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Existing redistribute `.ci` still pass (no regression)
