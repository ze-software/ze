# Spec: fixit-stored-route-relay-hardening

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/learned/1271-fixit-bgp-egress-rail-divergence.md` — the change this hardens
3. The Run 2 review artifact recorded under `tmp/review/` for that spec (findings R2-4..R2-9)
4. `plan/deferrals/fixit-bgp-egress-rail-divergence.md` — the rows this spec owns

## Task

`spec-fixit-bgp-egress-rail-divergence` routed the Adj-RIB-In peer-up replay through
the reactor's forward rail so a relayed route has ONE egress transform. That fixed the
four wrong-wire-bytes failures it targeted (372, 378, 394, 395) and shipped.

Two rounds of independent review found further defects in and around that path. The
ones that were mechanical were fixed there. The ones remaining are **investigations**,
not known fixes: each needs its behaviour established before code is written, because
the previous round proved that guessing at this layer produces worse outcomes than the
bug being fixed (a "partial relay must fail" guard was added that made a correctly
suppressed route fail an entire replay).

This spec investigates and closes them.

### I-1 (headline) — ADD-PATH replay is refused, and refusing removed working behaviour

`RelayStoredRoute` currently returns `errRelayAddPath` for any route whose SOURCE peer
negotiated ADD-PATH (`internal/component/bgp/reactor/reactor_api_relay.go`,
`buildRelayUpdate`). That is a deliberate fail-closed interim, but it is also a
functional regression, and both reviewers established the baseline independently:

- The old rail emitted `nlri <fam> add <hex>` with **no** `addpath` keyword (deleted
  `formatHexCommand`), and `parseWireNLRISection` defaults `addPath=false`
  (`internal/component/bgp/plugins/cmd/update/update_wire.go:272-278`).
- The structured ingest stores bare prefixes: `nlri.NLRIIterator.Next` advances past
  the 4-byte path-id and returns only the prefix (`internal/core/bgp/nlri/iterator.go:71-96`),
  which `installStructuredNLRIs` hex-encodes verbatim
  (`internal/component/bgp/plugins/adj_rib_in/rib.go`).

So add-path-sourced routes **did** replay correctly before, collapsed to path-id 0,
for single-path prefixes — and silently collapsed multi-path ones.

Worse, the refusal keys on the SOURCE context, so one add-path peer now kills peer-up
replay of its routes to EVERY destination, including non-add-path ones that worked
before. On a route server — the deployment that drives this replay — that is the
common case.

The root ambiguity: the two ingest paths store **different framings** and nothing
records which. Structured strips the path-id; legacy `prefixToWireHex` prepends it,
and only when non-zero although RFC 7911 permits path-id 0.

Investigate and decide, with evidence:
1. Whether to normalise storage (carry the path-id as a typed field on
   `rpc.StoredRoute`, or store the source framing consistently), or
2. Whether to tag the reconstruction with a context matching the source's ASN4 width
   but WITHOUT add-path (`bgpctx.EncodingContextForASN4`), which restores the old
   reach — but must be argued for multi-path routes, which it collapses.

Whichever lands must handle multi-path (several path-ids for one prefix), which
`compactRouteKey` already distinguishes but the old rail flattened.

### I-1b — the ownership claim's ordering is not deterministic, and the naive fix deadlocks

bgp-rs claims replay ownership from `OnAllPluginsReady`
(`internal/component/bgp/plugins/rs/server_handlers.go`, `claimReplayOwnership`). That
guarantees the dispatcher's command registry is frozen, but NOT that the claim lands
before the first session establishes: `sendPostStartupToAll`
(`internal/component/plugin/server/startup.go`) fans out one goroutine per plugin and
returns, and `signalStartupComplete` then calls `SignalPluginStartupComplete` ->
`StartPeers`. So the FIRST peer can still race the claim and be replayed twice.

**Making `sendPostStartupToAll` wait was tried on 2026-07-25 and DEADLOCKS.** That
function runs immediately before peers start, so a handler that waits on peer activity
blocks the peers it is waiting for until `postStartupTimeout`. Three functional tests
failed that way (394, 395, and the adj-rib-in replay test, with the observer reporting
"no routes stored"). The wait was reverted and the finding recorded in the function's
own doc comment.

So determinism here needs a DECLARATIVE route: ownership carried through the ordered
startup stages (a registration field surfaced to adj-rib-in at configure time, which is
strictly before ready and before peers start) rather than a callback racing them.
Investigate that shape; do not re-attempt the wait.

### I-2 — `relayChunkSize` bounds route count, not bytes

`relayRoutes` chunks at 4096 routes to stay under the 16 MB IPC frame ceiling
(`pkg/plugin/rpc/framing.go`). Route count does not bound bytes: `AttrHex` is hex, so
roughly twice the attribute block, and 4096 x a 4 KB block is already ~33 MB. A forked
adj-rib-in with large communities or AS_PATH blocks still loses a whole chunk as one
oversized frame. It fails closed, so this is availability, not corruption.
Replace with a byte-budget accumulator.

### I-3 — replay ownership is process-global, but replay driving is per-peer

`replayOwned` is a process-wide `atomic.Bool`. Event delivery is per-peer, per-plugin
(`internal/component/bgp/reactor/config.go`, `parseOneReceiveFlag` case `"state"`), so
a peer whose `process` block gives `state` to `bgp-adj-rib-in` but not to `bgp-rs`
leaves NOBODY replaying: adj-rib-in stood down globally, bgp-rs never sees that peer.
That is a config away, not a crash away. Scope the stand-down to peers the owner
actually drives, or reject the config combination.

### I-4 — the ownership claim does not survive an adj-rib-in respawn

`SendPostStartup` has one call site, inside `signalStartupComplete`
(`internal/component/plugin/server/startup.go`). A mid-life respawn
(`internal/component/plugin/server/reload_tx.go` -> `internal/component/plugin/process/manager.go`
`Respawn`) receives no post-startup callback, and `replayOwned` resets with the
process, so a respawned adj-rib-in resumes self-replay and the duplicate announce
returns. Re-deliver post-startup on respawn, or make the claim re-confirmable.

### I-5 — `test/plugin/adj-rib-in-replay-on-peerup.ci` does not gate

It replays to `10.0.0.99`, which is not a configured peer, so `RelayStoredRoute`
returns at destination resolution and the test asserts on the SELECTED route count.
It passes with the relay entirely dead — it fails the mutation check in
`ai/rules/functional-test-gate.md`. Give it a second source peer and assert on wire
bytes reaching an established destination, then mutation-verify it.

### I-6 — smaller relay gaps

- RFC 2545 32-byte next hop (global + link-local) is truncated to 16 by
  `nhopHexFromAddr` (`adj_rib_in/rib.go`), so a replay diverges from what a live
  forward relays verbatim and an on-link peer loses the link-local next hop.
- Complex families (VPN, EVPN, Flowspec) store the WHOLE MP_REACH NLRI block for the
  first NLRI and skip the rest (`adj_rib_in/rib.go`), so a replay re-announces every
  NLRI of the originating UPDATE — the failure the strip-and-resynthesize design
  claims to prevent, confined to these families.
- No backpressure: each in-flight relay pins a read-pool buffer with no bound, so a
  slow destination pins many and then fails the remainder.
- `routeRelayer` (the test seam) has no error return, so `replayCommand`'s
  `statusError` path cannot be driven by a test.
- `Coordinator.RelayStoredRoute` has no test — neither the `ErrNoReactor` branch nor
  the delegation is exercised.
- `relay_payload_test.go` asserts `n <= size` where `n == size` is the real contract.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/1271-fixit-bgp-egress-rail-divergence.md` — what was built and why
  → Constraint: a relayed route must keep exactly ONE egress transform; do not
     reintroduce a second rail while fixing add-path.
  → Constraint: the stored attribute block is the WHOLE attribute section including
     MP_REACH/MP_UNREACH (assumption A-1, verified), so any reconstruction strips 14/15.
- [ ] `ai/rules/fail-closed-guards.md`
  → Constraint: a guard that cannot deny must speak. The Run 2 blocker was a guard
     that denied something legitimate; both failure modes are in scope.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7911.md` — ADD-PATH; path-id 0 is legal, and multi-path means
      several path-ids per prefix
- [ ] `rfc/short/rfc4760.md` — MP_REACH_NLRI encoding
- [ ] `rfc/short/rfc2545.md` — 32-byte IPv6 next hop (global + link-local)

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing the design)
- [ ] `internal/component/bgp/reactor/reactor_api_relay.go` — `RelayStoredRoute`,
      `buildRelayUpdate`, `resolveRelaySource`, the `errRelay*` sentinels
- [ ] `internal/component/bgp/reactor/relay_payload.go` — the byte builders
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` — `RawRoute`, both ingest
      paths, `buildReplayRoutes`, `relayRoutes`, `replayOwned`
- [ ] `internal/core/bgp/nlri/iterator.go` — where the path-id is stripped
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` — `replayForPeer`,
      `claimReplayOwnership`, and the EOR-on-failure path

**Behavior to preserve:**
- The four target `.ci` tests (372, 378, 394, 395) stay green and non-reproducing
  under `scripts/dev/stress-repro.py`.
- One egress transform per relayed route.
- Originated / injected / redistribute routes keep going through
  `exportFilterForBody` (learned 1231, the private-ASN leak).

**Behavior to change:**
- Add-path sources must become replayable rather than refused (I-1).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The legacy (non-structured) ingest path is still reachable in a supported deployment | `handleReceived` exists for external text/JSON plugins | If dead, storage normalisation is far simpler — one framing only | grep for a config that delivers events as JSON to adj-rib-in | unvalidated |
| A-2 | Multi-path (several path-ids for one prefix) is representable end to end today | `compactRouteKey` carries a path-id | The old rail's collapse was masking a storage gap, widening I-1 | store two paths for one prefix and inspect the seqmap | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A fix for I-1 re-breaks the byte-identity the four target tests assert | 372/378/394/395 go red | Run all four plus stress-repro on every change |
| R-2 | Chunking by bytes changes the `last-index` bgp-rs uses for delta convergence | rs delta loop never terminates, or replays forever | Assert `last-index` across a multi-chunk replay |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer-up replay from an ADD-PATH source | Routes are relayed, not refused; wire is well-formed for both add-path and non-add-path destinations |
| AC-2 | Multi-path source (two path-ids, one prefix) | Both paths survive the replay, or the limitation is explicit and tested |
| AC-3 | Forked adj-rib-in, replay whose routes exceed 16 MB | Chunked by bytes; every route arrives |
| AC-4 | Peer gives `state` to adj-rib-in but not bgp-rs | Either that peer is still replayed, or the config is rejected — never silently unreplayed |
| AC-5 | adj-rib-in respawns mid-life with bgp-rs loaded | Ownership is re-established; no duplicate announce on the next peer-up |
| AC-6 | `adj-rib-in-replay-on-peerup.ci` with the relay stubbed to error | Test goes RED (mutation-verified) |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Refusing add-path was purely a safety improvement | The old rail replayed add-path routes correctly (collapsed to path-id 0) for single-path prefixes, so refusing removed working behaviour | Independent review traced `parseWireNLRISection` defaulting `addPath=false` and the iterator storing bare prefixes | The refusal must be recorded as an accepted interim regression, and lifting it is this spec's headline |
| "No peers accepted" means the relay failed | `forwardUpdateCore` returns `errNoEstablishedPeersToForwardTo` for a correctly egress-SUPPRESSED route too | Independent review, Run 2 | A "partial relay must fail" guard made one suppressed route fail a whole replay; fixed before shipping |

## Data Flow

### Entry Point
A peer establishes. Either bgp-rs dispatches `request bgp adj-rib-in replay <peer> <index>`
(`internal/component/bgp/plugins/rs/server_handlers.go`, `replayForPeer`), or — when no
plugin has claimed ownership — adj-rib-in's own peer-up handler fires
(`internal/component/bgp/plugins/adj_rib_in/rib.go`, `handleStructuredState` / `handleState`).

### Transformation Path
1. `buildReplayRoutes` selects stored routes for the target peer and attaches each one's
   SOURCE peer, yielding `[]rpc.StoredRoute` (hex attribute block, hex next-hop, hex NLRI).
2. `relayRoutes` chunks them and calls the engine's `relay-stored-route` RPC (DirectBridge
   for internal plugins, JSON framing for forked ones).
3. `RelayStoredRoute` resolves the destination and, per route, resolves the source peer's
   forwarding facts and receive encoding context.
4. `buildRelayUpdate` rebuilds a received-shape UPDATE body into a pooled buffer
   (`relay_payload.go`), registers it in the recent-UPDATE cache for buffer ownership, and
   hands it to `forwardUpdateCore` — the same egress transform a live forward uses.

**Where this spec intervenes:** step 4 currently REFUSES when the source negotiated
ADD-PATH, because step 1's stored NLRI framing does not record whether it carries a
path-id. Step 2's chunking bounds route count rather than bytes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| adj-rib-in plugin ↔ engine | `relay-stored-route` RPC, both DirectBridge and forked JSON | [ ] |
| engine ↔ session write | forward rail writes pre-filtered (no re-gate) | [ ] |
| plugin startup ↔ peer startup | post-startup fan-out completes before `StartPeers` | [ ] |

### Integration Points
- `forwardUpdateCore` (`internal/component/bgp/reactor/reactor_api_forward.go`) — the single
  egress transform; must stay single.
- `nlri.NLRIIterator` (`internal/core/bgp/nlri/iterator.go`) — where the path-id is dropped.
- `rpc.StoredRoute` (`pkg/plugin/rpc/types.go`) — the wire contract that may need a path-id field.
- `sendPostStartupToAll` (`internal/component/plugin/server/startup.go`) — the ordering that
  makes ownership deterministic.

## Wiring Test
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer-up replay from an ADD-PATH source | → | `buildRelayUpdate` add-path handling | `.ci` in `test/plugin/` asserting well-formed wire at an add-path destination (mutation-verified) |
| peer-up replay, `state` delivered to adj-rib-in but not bgp-rs | → | ownership scoping | `.ci` asserting the peer is still replayed exactly once |
| forked adj-rib-in, replay exceeding one IPC frame | → | byte-budget chunking in `relayRoutes` | unit test over the chunker + `.ci` with a large stored RIB |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRelayAddPathRoundTrip` | `internal/component/bgp/reactor/relay_payload_test.go` | an add-path route reconstructs to wire the destination parses, for both add-path and non-add-path destinations | |
| `TestRelayMultiPathPreserved` | `internal/component/bgp/reactor/relay_payload_test.go` | two path-ids for one prefix both survive, or the limit is explicit | |
| `TestRelayChunkByteBudget` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | chunks stay under `rpc.MaxMessageSize` for large attribute blocks | |
| `TestReplayOwnershipRespawn` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | ownership re-established after respawn; no duplicate | |
| `TestCoordinatorRelayStoredRouteNoReactor` | `internal/component/plugin/coordinator_test.go` | `ErrNoReactor` branch and the delegation | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| add-path replay | `test/plugin/` | an add-path peer establishes late and receives its routes | |
| `adj-rib-in-replay-on-peerup.ci` rewrite | `test/plugin/` | replay to an ESTABLISHED peer, asserted on wire bytes | |

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_relay.go` — lift `errRelayAddPath`; context choice
- `internal/component/bgp/reactor/relay_payload.go` — path-id framing; RFC 2545 next hop
- `internal/component/bgp/plugins/adj_rib_in/rib.go` — storage framing, byte-budget chunking, ownership scope
- `pkg/plugin/rpc/types.go` — `StoredRoute` path-id field, if that is the chosen route
- `internal/component/plugin/server/startup.go` / `process/manager.go` — post-startup on respawn
- `test/plugin/adj-rib-in-replay-on-peerup.ci` — make it gate

## Implementation Steps

1. **Investigate I-1 first, and write the finding before any code.** Establish what the two
   ingest paths actually store (A-1, A-2), then choose normalisation vs context-tagging with
   evidence. Nothing else in this spec is blocked on it, so it goes first while context is fresh.
2. Lift the add-path refusal behind the chosen design; prove with the new `.ci` plus the four
   inherited target tests still green under `scripts/dev/stress-repro.py`.
3. Byte-budget chunking (I-2), with the `last-index` contract asserted across chunks.
4. Ownership scope and respawn (I-3, I-4).
5. Make `adj-rib-in-replay-on-peerup.ci` gate (I-5); mutation-verify.
6. The smaller gaps (I-6).
7. `make ze-verify`, `make ze-race-reactor`, per-test stress-repro; independent review to clean.

## Checklist
### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
### Completion (BLOCKING — before ANY commit)
- [ ] Every AC has working code + test
- [ ] 372 / 378 / 394 / 395 still green and non-reproducing under stress
- [ ] `make ze-race-reactor` green
- [ ] `make ze-test` passes
- [ ] Independent review clean
- [ ] Learned summary written
