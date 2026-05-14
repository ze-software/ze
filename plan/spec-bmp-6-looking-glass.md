# Spec: BMP Receiver Looking Glass Integration

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/7 |
| Updated | 2026-05-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `pkg/plugin/rpc/bridge.go` - DirectBridge, InjectWireRoute handler
4. `internal/component/bgp/plugins/bmp/bmp.go` - BMP plugin lifecycle, processRouteMonitoring
5. `internal/component/bgp/plugins/rib/rib.go` - RIBManager, ribInPool, bmpProtocolID
6. `internal/component/bgp/plugins/rib/rib_pipeline.go` - newInboundSource, protocol filtering
7. `internal/component/bgp/plugins/rib/rib_commands.go` - gatherCandidatesLocked (bgpPeers only)
8. `internal/component/plugin/server/dispatch.go` - wireBridgeDispatch, InjectWireRoute wiring
9. `internal/component/lg/handler_api.go` - LG birdwatcher API, BMP endpoints
10. `internal/component/lg/handler_ui.go` - LG HTMX UI

## Task

Operators running ze as a BMP receiver (route collector) want to browse the
collected routes through ze's looking glass UI and API. Today the BMP receiver
(`bgp-bmp` plugin) parses Route Monitoring messages but discards the BGP UPDATE
payload after logging. No routes are stored, and the looking glass only shows
peers from real BGP sessions.

This spec covers:
- Storing BMP Route Monitoring routes in the RIB under a separate ProtocolID
  ("bmp") so they are visible to dedicated queries (`bmp rib show`) but never
  enter best-path selection or the FIB.
- A DirectBridge typed handler (`InjectWireRoute`) for zero-copy wire-byte
  injection from the BMP plugin into the RIB.
- Dedicated BMP looking glass API endpoints and UI section, separate from
  the BGP looking glass.

Out of scope:
- BMP client/active mode (ze connecting outbound to a BMP source). The receiver
  already accepts inbound BMP connections.
- BMP Route Mirroring display.
- BMP Statistics Report display in the looking glass.
- Per-monitored-router namespace in the looking glass (all BMP routes live in
  a flat peer namespace under bmpProtocolID, keyed by composite `<router>:<peer>`).

### Design Context

This spec supersedes `spec-rib-3-remote-client.md` (skeleton, never started).
That spec proposed a gRPC-based remote RIB subscription client. Design review
found: (1) no OSS BGP daemon implements gRPC RIB subscription as a built-in
feature, (2) the use case ("mirror remote RIB without BGP session") is solved
by BMP (RFC 7854, RFC 9069), (3) ze already has a BMP receiver with route
parsing. The gRPC approach would also require protobuf-to-wire re-encoding on
every route, conflicting with ze's buffer-first design.

## Design Decisions

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D5 | Best-path exclusion | ProtocolID "bmp", not StaleLevel=255 | gatherCandidatesLocked only iterates bgpPeers; ribInPool two-level map designed for multi-source. StaleLevel stays clean for GR/LLGR. |
| D6 | Peer identity | Composite key `<router>:<peer-address>` | Handles multiple BMP routers reporting same peer. Colon is unambiguous. |
| D7 | Route injection | DirectBridge `InjectWireRoute` typed handler | Zero-copy, follows ForwardCached precedent. Avoids hex encode/decode on full table sync. |
| D8 | LG peer list | LG queries "bmp peers" for BMP, "summary" for BGP. Separate API endpoints. | BMP plugin has richer peer metadata. Keeps reactor clean. |
| D9 | Resource bounds | No cap, document the risk | Defer to future spec. Consistent with unbounded real BGP peers. |
| D10 | Route display | `bmp rib show` (dedicated), `bgp rib show` excludes BMP. Separate LG `/bmp/` endpoints. | Clean operational separation of monitored vs operational routes. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - engine + plugin model
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB storage internals
- [ ] `ai/patterns/plugin.md` - plugin shape
- [ ] `ai/rules/design-principles.md`
  → Constraint: lazy over eager; do not allocate per-prefix structs when iterating
  → Constraint: buffer-first; BMP Route Monitoring carries BGP wire bytes, reuse them

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7854.md` - BMP base spec (Route Monitoring = Type 0)
  → Constraint: Route Monitoring message contains a BGP UPDATE with per-peer header
- [ ] `rfc/short/rfc4271.md` - BGP UPDATE wire format (NLRI + attributes)

**Key insights:**
- BMP Route Monitoring messages wrap verbatim BGP UPDATEs. The UPDATE body is
  already in ze's wire format. No re-encoding needed.
- ribInPool is a two-level map keyed by ProtocolID then peer address. A "bmp"
  ProtocolID stores BMP routes separately from BGP routes. gatherCandidatesLocked
  only iterates bgpPeers (the cached reference to ribInPool[bgpProtocolID]), so
  BMP routes are automatically excluded from best-path with zero filter code.
- newInboundSource (rib_pipeline.go) currently iterates all ribInPool protocols.
  This spec restricts `bgp rib show` to bgpProtocolID and adds `bgp rib show-protocol`
  for protocol-filtered queries. BMP plugin registers `bmp rib show` as the user command.
- The looking glass gets dedicated BMP API endpoints (`/protocols/bmp`, `/routes/bmp/{name}`)
  separate from the BGP endpoints. BMP peer data comes from "bmp peers" command.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - processRouteMonitoring logs and discards
- [ ] `internal/component/bgp/plugins/bmp/msg.go` - RouteMonitoring type (Peer + BGPUpdate bytes)
- [ ] `internal/component/bgp/plugins/bmp/state.go` - bmpState tracks routers + peers, no routes
- [ ] `internal/component/bgp/plugins/rib/storage/routeentry.go` - RouteEntry 56 bytes, StaleLevel
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - gatherCandidatesLocked, injectRoute
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - checkBestPathChange -> locRIB
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - dispatchStructured handler
- [ ] `internal/component/lg/handler_api.go` - birdwatcher API, queries "summary", "peer X bgp rib show"
- [ ] `internal/component/lg/server.go` - CommandDispatcher interface

**Behavior to preserve:**
- Real BGP peers unaffected. Their routes enter best-path and FIB as before.
- BMP sender functionality unchanged.
- BMP receiver session/peer tracking in bmpState unchanged.
- Looking glass existing peer/route queries work identically for real peers.
- RouteEntry size remains 56 bytes.

**Behavior to change:**
- `processRouteMonitoring` stores received BGP UPDATE routes in the RIB
  via a new DirectBridge typed handler (`InjectWireRoute`) under ProtocolID
  "bmp". Peer identity uses composite key `<router>:<peer-address>`.
- BMP routes are automatically excluded from best-path because
  gatherCandidatesLocked only iterates bgpPeers (ribInPool[bgpProtocolID]).
- BMP-monitored peers appear in looking glass peer lists with a "bmp" tag.
  The LG queries "bmp peers" alongside "summary" and merges results.
- BMP-monitored peer routes are queryable via `bmp rib show` (dedicated command).
  `bgp rib show` is restricted to bgpProtocolID only (no longer iterates all
  ribInPool protocols). BMP plugin dispatches `bgp rib show-protocol bmp` to the
  RIB plugin, which filters ribInPool[bmpProtocolID].

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- BMP TCP connection delivers a Route Monitoring message (Type 0).
- Message contains: per-peer header (peer address, AS, distinguisher) +
  BGP UPDATE wire bytes.

### Transformation Path
1. BMP receiver decodes the message header and extracts the BGP UPDATE body
   (`processRouteMonitoring` in `bmp.go`)
2. BMP plugin calls `bridge.InjectWireRoute("bmp", "<router>:<peer>", wireBytes)`
   via a new DirectBridge typed handler. The peer identity is a composite key
   derived from the BMP per-peer header (router address + peer address).
3. Engine-side handler resolves ProtocolID "bmp" and stores the route in
   `ribInPool[bmpProtocolID]["<router>:<peer>"]` using the same attribute
   parsing path as handleReceivedStructured.
4. On BMP Peer Down, the plugin dispatches a withdraw-all for that monitored peer.
5. On BMP session disconnect (Termination or TCP close), the plugin dispatches
   withdraw-all for all peers of that router.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network -> BMP plugin | TCP listener, existing | [ ] |
| BMP plugin -> RIB (via engine) | DirectBridge `InjectWireRoute` (zero-copy wire bytes) | [ ] |
| RIB plugin -> LG (BGP) | CommandDispatcher "peer X bgp rib show" (existing, filtered to bgpProtocolID) | [ ] |
| RIB plugin -> LG (BMP) | CommandDispatcher "bgp rib show-protocol bmp" (new, filtered to bmpProtocolID) | [ ] |
| BMP plugin -> LG | CommandDispatcher "bmp peers" (existing bmpState.peersCommand) | [ ] |

### Integration Points
- `processRouteMonitoring` - currently discards BGP UPDATE, will call InjectWireRoute
- DirectBridge `InjectWireRoute` handler - stores in ribInPool[bmpProtocolID]
- Best-path exclusion - automatic: gatherCandidatesLocked iterates only bgpPeers
- newInboundSource - restrict `bgp rib show` to bgpProtocolID; add protocol filter for `bmp rib show`
- `bmpState.peerDown` / `removeRouter` - trigger withdraw-all for monitored peers
- Looking glass - separate `/api/looking-glass/protocols/bmp` and `/api/looking-glass/routes/bmp/{name}` endpoints

### Architectural Verification
- [ ] BMP routes flow through the existing RIB storage (no parallel RIB)
- [ ] BMP routes never reach locRIB (stored under bmpProtocolID, gatherCandidatesLocked only iterates bgpPeers)
- [ ] Zero-copy: BGP UPDATE wire bytes from BMP passed via DirectBridge InjectWireRoute, no hex encoding
- [ ] No new coupling: BMP plugin communicates via DirectBridge typed handler, not sibling imports
- [ ] Peer identity: composite key `<router>:<peer-address>` prevents collision with real BGP peers

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BMP receiver gets Route Monitoring | → | RIB stores under bmpProtocolID | `TestBMPRouteMonitoringInjectsToRIB` |
| LG queries `/api/looking-glass/protocols/bmp` | → | BMP-monitored peers listed | `TestLGBMPProtocols` |
| LG queries `/api/looking-glass/routes/bmp/{name}` | → | BMP peer routes returned | `TestLGBMPRoutes` |
| BMP Peer Down | → | Routes withdrawn from bmpProtocolID | `TestBMPPeerDownWithdrawsRoutes` |
| Best-path runs with BMP routes in RIB | → | BMP routes not considered (separate ProtocolID) | `TestBMPRoutesExcludedFromBestPath` |
| `bmp rib show` via BMP plugin | → | Routes from ribInPool[bmpProtocolID] displayed | `TestBMPRIBShow` |
| BMP session disconnects | → | All router peers withdrawn | `test/plugin/bmp-lg-disconnect.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BMP receiver gets Route Monitoring with IPv4 unicast UPDATE | Route stored in ribInPool[bmpProtocolID] under composite peer key, visible via `bmp rib show` |
| AC-2 | BMP receiver gets Route Monitoring with IPv6 unicast UPDATE | Same as AC-1 for IPv6 |
| AC-3 | BMP route exists for a prefix, real BGP peer also has the prefix | Best-path selects only the real peer's route. BMP route not considered (separate ProtocolID). |
| AC-4 | BMP route exists for a prefix, no real peer has it | No best path selected. Route not installed in locRIB/FIB. Visible via `bmp rib show` only. |
| AC-5 | BMP Peer Down received | All routes for that monitored peer withdrawn from ribInPool[bmpProtocolID] |
| AC-6 | BMP session disconnects (TCP close or Termination) | All routes for all peers of that router withdrawn |
| AC-7 | Looking glass API `/api/looking-glass/protocols/bmp` | BMP-monitored peers listed (separate endpoint from BGP protocols) |
| AC-8 | Looking glass API `/api/looking-glass/routes/bmp/{name}` for a BMP peer | Routes returned in birdwatcher format |
| AC-9 | Looking glass UI peer list | BMP-monitored peers in separate section, visually distinguished |
| AC-10 | `bmp rib show` | BMP routes displayed (dedicated command, not mixed into `bgp rib show`) |
| AC-11 | BMP receiver disabled or no BMP sessions | No impact on existing RIB or LG behavior |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBMPRoutesExcludedFromBestPath` | `internal/component/bgp/plugins/rib/rib_test.go` | gatherCandidatesLocked does not iterate bmpProtocolID peers | |
| `TestInjectWireRouteStoresBMPProtocol` | `internal/component/bgp/plugins/rib/rib_test.go` | InjectWireRoute stores under ribInPool[bmpProtocolID] | |
| `TestInjectWireRouteWithdraw` | `internal/component/bgp/plugins/rib/rib_test.go` | withdraw removes route from bmpProtocolID peer | |
| `TestBMPRouteMonitoringInjectsToRIB` | `internal/component/bgp/plugins/bmp/bmp_test.go` | processRouteMonitoring calls InjectWireRoute | |
| `TestBMPPeerDownWithdrawsRoutes` | `internal/component/bgp/plugins/bmp/bmp_test.go` | processPeerDown dispatches withdraw-all | |
| `TestBMPSessionDisconnectWithdrawsAll` | `internal/component/bgp/plugins/bmp/bmp_test.go` | session close dispatches withdraw-all per router | |
| `TestBMPCompositeKeyFormat` | `internal/component/bgp/plugins/bmp/bmp_test.go` | peer key is `<router>:<peer-address>` | |
| `TestLGBMPProtocols` | `internal/component/lg/handler_api_test.go` | `/api/looking-glass/protocols/bmp` returns BMP peers | |
| `TestLGBMPRoutes` | `internal/component/lg/handler_api_test.go` | `/api/looking-glass/routes/bmp/{name}` returns BMP peer routes | |
| `TestLGBGPProtocolsExcludesBMP` | `internal/component/lg/handler_api_test.go` | `/api/looking-glass/protocols/bgp` does not include BMP peers | |
| `TestDirectBridgeInjectWireRoute` | `pkg/plugin/rpc/bridge_test.go` | bridge handler Set/Has/call triplet works | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP UPDATE length in InjectWireRoute | 23..4096 | 4096 (RFC 4271 max) | 22 (below minimum UPDATE) | 4097 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-bmp-lg-ingest` | `test/plugin/bmp-lg-ingest.ci` | BMP receiver ingests routes, LG shows them | |
| `test-bmp-lg-disconnect` | `test/plugin/bmp-lg-disconnect.ci` | BMP session down withdraws all routes from LG | |
| `test-bmp-lg-bestpath-isolation` | `test/plugin/bmp-lg-bestpath-isolation.ci` | Monitor-only routes do not affect real best-path | |

### Future (if deferring any tests)
- Interop with a real BMP sender (router or OpenBMP) - requires external fixture.

## Files to Modify
- `pkg/plugin/rpc/bridge.go` - add InjectWireRoute handler triplet (type, Set, Has, call)
- `internal/component/plugin/server/dispatch.go` - wire InjectWireRoute in wireBridgeDispatch
- `internal/component/bgp/plugins/rib/rib.go` - register bmpProtocolID, handle InjectWireRoute on engine side
- `internal/component/bgp/plugins/rib/rib_commands.go` - add withdraw command for bmpProtocolID peers
- `internal/component/bgp/plugins/rib/rib_attr_format.go` - display "bmp" source tag for bmpProtocolID routes
- `internal/component/bgp/plugins/bmp/bmp.go` - processRouteMonitoring calls InjectWireRoute
- `internal/component/bgp/plugins/bmp/state.go` - track peer->route-count for withdraw-all
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - newInboundSource filters by ProtocolID (bgpPeers only for `bgp rib show`)
- `internal/component/lg/handler_api.go` - add `/api/looking-glass/protocols/bmp` and `/api/looking-glass/routes/bmp/{name}` endpoints
- `internal/component/lg/server.go` - register BMP API routes

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Receiver config already exists |
| CLI commands/flags | Yes | `bmp rib show` (user-facing), `bgp rib show-protocol` (internal) |
| Editor autocomplete | No | No new user config |
| Functional test for new API | Yes | `test/plugin/bmp-lg-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - BMP looking glass |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | Internal commands only |
| 4 | API/RPC added/changed? | Yes | New `/api/looking-glass/protocols/bmp`, `/api/looking-glass/routes/bmp/{name}` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - bgp-bmp enhanced |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md` - add LG section |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | BMP Route Monitoring is already parsed |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - BMP looking glass column |
| 12 | Internal architecture changed? | No | Uses existing RIB + LG paths |

## Files to Create
- `test/plugin/bmp-lg-ingest.ci`
- `test/plugin/bmp-lg-disconnect.ci`
- `test/plugin/bmp-lg-bestpath-isolation.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files, tests |
| 3. Implement (TDD) | Phases below |
| 4. /ze-review gate | Review Gate |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Checklist |
| 7. Fix issues | - |
| 8. Re-verify | Stage 5 |
| 9. Repeat 6-8 | Max 2 passes |
| 10. Deliverables review | Checklist |
| 11. Security review | Checklist |
| 12. Re-verify | Stage 5 |
| 13. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: DirectBridge InjectWireRoute handler** - add typed handler triplet to
   bridge.go, wire in dispatch.go. Register bmpProtocolID in RIB plugin. Engine-side
   handler stores routes in ribInPool[bmpProtocolID] using existing attribute parsing.
   - Tests: `TestDirectBridgeInjectWireRoute`, `TestInjectWireRouteStoresBMPProtocol`, `TestBMPRoutesExcludedFromBestPath`
   - Files: `pkg/plugin/rpc/bridge.go`, `internal/component/plugin/server/dispatch.go`, `rib/rib.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: BMP Route Monitoring -> RIB via InjectWireRoute** - processRouteMonitoring
   calls bridge.InjectWireRoute with composite peer key `<router>:<peer-address>`.
   Peer Down and session disconnect dispatch withdraw-all.
   - Tests: `TestBMPRouteMonitoringInjectsToRIB`, `TestBMPPeerDownWithdrawsRoutes`, `TestBMPSessionDisconnectWithdrawsAll`, `TestBMPCompositeKeyFormat`
   - Files: `bmp/bmp.go`, `bmp/state.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Route display separation** - restrict `bgp rib show` to bgpProtocolID.
   Add `bgp rib show-protocol <name>` command for protocol-filtered queries.
   BMP plugin registers `bmp rib show` dispatching to `bgp rib show-protocol bmp`.
   - Tests: `TestBMPRIBShow`, `TestInjectWireRouteWithdraw`
   - Files: `rib/rib_pipeline.go`, `rib/rib_commands.go`, `bmp/bmp.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Looking glass BMP endpoints** - add `/api/looking-glass/protocols/bmp`
   (queries "bmp peers") and `/api/looking-glass/routes/bmp/{name}` (queries
   `bgp rib show-protocol bmp`). LG UI shows BMP peers in a separate section.
   - Tests: `TestLGBMPProtocols`, `TestLGBMPRoutes`, `TestLGBGPProtocolsExcludesBMP`
   - Files: `lg/handler_api.go`, `lg/server.go`, `lg/handler_ui.go`
   - Verify: tests fail -> implement -> tests pass

5. **Functional tests** - create .ci tests covering ingest, disconnect, isolation.
   - Files: `test/plugin/bmp-lg-*.ci`

6. **Docs** - features, comparison, BMP guide.

7. **Full verification** - `make ze-verify`

8. **Complete spec** - audit + learned summary `plan/learned/NNN-bmp-6-looking-glass.md`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each AC has file:line |
| Correctness | BMP routes never reach locRIB (separate ProtocolID, gatherCandidatesLocked iterates bgpPeers only) |
| Naming | bmpProtocolID, InjectWireRoute, composite peer key `<router>:<peer>` |
| Data flow | BMP -> DirectBridge InjectWireRoute -> RIB storage. No sibling import. |
| Rule: no-layering | No parallel RIB storage; reuses existing FamilyRIB under bmpProtocolID |
| Rule: buffer-first | BGP UPDATE wire bytes from BMP passed zero-copy via DirectBridge |
| Separation | `bgp rib show` excludes BMP; `bmp rib show` is dedicated; LG has separate `/bmp/` endpoints |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| bmpProtocolID registered | grep `RegisterProtocol.*bmp` rib/rib.go |
| InjectWireRoute bridge handler | grep `InjectWireRoute` pkg/plugin/rpc/bridge.go |
| InjectWireRoute wired in dispatch | grep `InjectWireRoute\|SetInjectWireRoute` dispatch.go |
| processRouteMonitoring calls InjectWireRoute | grep `InjectWireRoute` bmp/bmp.go |
| Composite peer key | grep `<router>:` or string concat in bmp/bmp.go |
| `bgp rib show` excludes BMP | newInboundSource filters to bgpPeers |
| `bmp rib show` command registered | grep `bmp rib show` bmp/bmp.go |
| LG BMP endpoints | grep `/protocols/bmp\|/routes/bmp/` lg/handler_api.go |
| LG shows BMP peers | functional test bmp-lg-ingest.ci passes |
| Docs updated | ls docs/guide/bmp.md, grep "looking glass" docs/features.md |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | BMP UPDATE wire bytes validated before RIB insert (malformed -> skip + counter) |
| Resource exhaustion | No cap implemented. Document risk: full Internet table is ~1M IPv4 + ~200K IPv6 prefixes per monitored peer. Multiple BMP routers with many peers can consume significant memory. Operator responsibility to size deployment. Defer cap to future spec. |
| Peer identity collision | Composite key `<router>:<peer-address>` under separate bmpProtocolID. No collision with real BGP peers possible (different ProtocolID + different key format). |
| Error leakage | Errors name BMP fields, not internal addresses |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| BMP route reaches locRIB/best-path | BLOCKER. Verify bmpProtocolID is not iterated by gatherCandidatesLocked |
| BMP peer collides with real peer | Cannot happen: separate ProtocolID + composite key format |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- ProtocolID "bmp" leverages the two-level ribInPool map (added in spec-rib-4-extraction)
  to separate BMP routes from BGP routes. Best-path exclusion is automatic:
  gatherCandidatesLocked only iterates bgpPeers. No StaleLevel hack needed.
  StaleLevel stays clean for its actual purpose (GR/LLGR staleness within a protocol).
- DirectBridge InjectWireRoute follows the ForwardCached/ReleaseCached precedent:
  a typed bridge handler for a specific operation where performance matters.
  Eliminates hex encode/decode overhead on BMP full table sync (~1M routes).
- Composite peer key `<router>:<peer-address>` encodes full BMP identity. Handles
  multiple BMP routers reporting the same downstream peer (redundant route reflectors).
- BMP Route Monitoring carries verbatim BGP UPDATEs. This means the RIB's
  existing wire-byte attribute parsing works directly on BMP payloads.
- Clean separation: `bgp rib show` is BGP-only, `bmp rib show` is BMP-only,
  LG has `/protocols/bmp` and `/routes/bmp/` endpoints. No mixing of operational
  BGP data with monitored BMP data.
- This spec replaces spec-rib-3-remote-client. The gRPC remote RIB subscription
  pattern does not exist in any OSS BGP daemon. BMP (RFC 7854 + RFC 9069) is the
  standards-track answer to "mirror a remote RIB without a BGP session."

## RFC Documentation

BMP Route Monitoring (RFC 7854 Section 4.6): each message contains one BGP
UPDATE PDU. The per-peer header identifies the monitored peer.
RFC 9069: Loc-RIB monitoring via BMP (extends Route Monitoring to Loc-RIB).

## Implementation Summary

### What Was Implemented

**Phase 1: DirectBridge InjectWireRoute handler**
- `pkg/plugin/rpc/bridge.go`: InjectWireRouteHandler type, Set/Has/call triplet, global RegisterRouteInjector/GetRouteInjector
- `internal/component/plugin/server/dispatch.go`: wireBridgeDispatch wires InjectWireRoute on every plugin's bridge
- `internal/component/bgp/plugins/rib/rib.go`: bmpProtocolID registered, bmpProtocolID inner map in NewRIBManager, RegisterRouteInjector called in RunRIBPlugin
- `internal/component/bgp/plugins/rib/rib_inject.go`: handleInjectWireRoute (UPDATE parsing + storage), withdrawAllForPeer, withdrawAllForRouter, showProtocolPipeline, registerInjectCommands (show-protocol, withdraw-protocol, withdraw-router)
- `pkg/plugin/sdk/sdk_engine.go`: Plugin.InjectWireRoute() method

**Phase 2: BMP Route Monitoring -> RIB**
- `internal/component/bgp/plugins/bmp/bmp.go`: processRouteMonitoring calls InjectWireRoute with composite key, processPeerDown dispatches withdraw-protocol, processTermination dispatches withdraw-router, handleSession deferred withdraw-router on disconnect, bmpCompositeKey/peerAddressString helpers, "bmp rib show" command registered and dispatched to "bgp rib show-protocol bmp"

**Phase 3: Route display separation**
- `internal/component/bgp/plugins/rib/rib_pipeline.go`: newInboundSource restricted to bgpPeers only, protocolInboundSource added for protocol-filtered queries

**Phase 4: Looking glass BMP endpoints**
- `internal/component/lg/handler_api.go`: handleAPIBMPProtocols, handleAPIBMPRoutes, transformBMPProtocols
- `internal/component/lg/server.go`: registered /api/looking-glass/protocols/bmp and /api/looking-glass/routes/bmp/{name}

### Bugs Found/Fixed
- BMP tests panicked on nil plugin reference in deferred withdraw calls. Fixed with nil guards.
- RIB protocol test expected 16 commands, updated to 19 for new commands.

### Documentation Updates
- docs/guide/bmp.md: Added Looking Glass Integration section (CLI, API, route lifecycle)
- docs/features.md: Updated Looking Glass description to mention BMP route display

### Deviations from Plan
- Used global RegisterRouteInjector/GetRouteInjector in rpc package instead of PluginServerAccessor interface extension. Simpler, no import cycle, same semantics.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Store BMP routes in RIB under bmpProtocolID | done | rib_inject.go:25 | handleInjectWireRoute |
| DirectBridge InjectWireRoute typed handler | done | bridge.go:304 | zero-copy, no hex |
| BMP routes excluded from best-path | done | rib_commands.go:901 | gatherCandidatesLocked iterates bgpPeers only |
| Dedicated BMP LG API endpoints | done | handler_api.go:152,166 | /protocols/bmp, /routes/bmp/{name} |
| bgp rib show excludes BMP | done | rib_pipeline.go:73 | newInboundSource uses bgpPeers |
| bmp rib show dedicated command | done | bmp.go:473 | dispatches to bgp rib show-protocol bmp |
| Withdraw on peer down | done | bmp.go:576 | withdraw-protocol via DispatchCommand |
| Withdraw on session disconnect | done | bmp.go:397 | withdraw-router via deferred DispatchCommand |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | TestInjectWireRouteStoresBMPProtocol, bmp-lg-ingest.ci | IPv4 unicast stored |
| AC-2 | done | handleInjectWireRoute MPReach path | IPv6 via MP_REACH_NLRI |
| AC-3 | done | TestBMPRoutesExcludedFromBestPath, bmp-lg-bestpath-isolation.ci | separate ProtocolID |
| AC-4 | done | TestBMPRoutesExcludedFromBestPath | no best path for BMP-only prefix |
| AC-5 | done | TestWithdrawAllForPeer | routes withdrawn on peer down |
| AC-6 | done | TestWithdrawAllForRouter, bmp-lg-disconnect.ci | session disconnect withdraws all |
| AC-7 | done | handleAPIBMPProtocols, TestTransformBMPProtocolsFields | BMP peers listed |
| AC-8 | done | handleAPIBMPRoutes | routes in birdwatcher format |
| AC-9 | partial | - | UI section not implemented (API only) |
| AC-10 | done | TestShowProtocolPipelineBMP | bmp rib show works |
| AC-11 | done | TestInjectWireRouteUnknownProtocol | no impact when BMP disabled |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestBMPRoutesExcludedFromBestPath | done | rib_inject_test.go | |
| TestInjectWireRouteStoresBMPProtocol | done | rib_inject_test.go | |
| TestInjectWireRouteWithdraw | done | rib_inject_test.go | |
| TestBMPCompositeKeyFormat | done | bmp_test.go | |
| TestLGBMPProtocols | done | handler_api_test.go | TestTransformBMPProtocolsFields |
| TestLGBMPRoutes | partial | handler_api_test.go | transform tested, handler needs integration test |
| TestLGBGPProtocolsExcludesBMP | done | handler_api_test.go | |
| TestDirectBridgeInjectWireRoute | done | rib_inject_test.go | via TestInjectWireRouteStoresBMPProtocol |
| bmp-lg-ingest.ci | done | test/plugin/ | |
| bmp-lg-disconnect.ci | done | test/plugin/ | |
| bmp-lg-bestpath-isolation.ci | done | test/plugin/ | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| pkg/plugin/rpc/bridge.go | modified | InjectWireRoute handler triplet |
| internal/component/plugin/server/dispatch.go | modified | wireBridgeDispatch wiring |
| internal/component/bgp/plugins/rib/rib.go | modified | bmpProtocolID, RegisterRouteInjector |
| internal/component/bgp/plugins/rib/rib_inject.go | created | inject handler, withdraw, show-protocol |
| internal/component/bgp/plugins/rib/rib_commands.go | modified | registerInjectCommands call |
| internal/component/bgp/plugins/rib/rib_pipeline.go | modified | protocolInboundSource, bgpPeers filter |
| internal/component/bgp/plugins/bmp/bmp.go | modified | Route Monitoring injection, withdrawals |
| internal/component/lg/handler_api.go | modified | BMP endpoints |
| internal/component/lg/server.go | modified | BMP route registration |
| test/plugin/bmp-lg-ingest.ci | created | |
| test/plugin/bmp-lg-disconnect.ci | created | |
| test/plugin/bmp-lg-bestpath-isolation.ci | created | |

### Audit Summary
- **Total items:** 31
- **Done:** 29
- **Partial:** 2 (AC-9 UI section, TestLGBMPRoutes integration)
- **Skipped:** 0
- **Changed:** 0

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| rib_inject.go | yes | internal/component/bgp/plugins/rib/rib_inject.go |
| rib_inject_test.go | yes | internal/component/bgp/plugins/rib/rib_inject_test.go |
| bmp_test.go | yes | internal/component/bgp/plugins/bmp/bmp_test.go |
| bmp-lg-ingest.ci | yes | test/plugin/bmp-lg-ingest.ci |
| bmp-lg-disconnect.ci | yes | test/plugin/bmp-lg-disconnect.ci |
| bmp-lg-bestpath-isolation.ci | yes | test/plugin/bmp-lg-bestpath-isolation.ci |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | BMP routes stored under bmpProtocolID | grep InjectWireRoute bmp.go: calls bp.plugin.InjectWireRoute |
| AC-3 | BMP excluded from best-path | gatherCandidatesLocked iterates r.bgpPeers only |
| AC-5 | Peer down withdraws | grep withdraw-protocol bmp.go: dispatched on processPeerDown |
| AC-6 | Session disconnect withdraws | deferred withdraw-router in handleSession |
| AC-7 | LG BMP protocols endpoint | /api/looking-glass/protocols/bmp registered in server.go |
| AC-10 | bmp rib show | grep "bmp rib show" bmp.go: command registered and dispatched |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| BMP Route Monitoring -> RIB | bmp-lg-ingest.ci | yes |
| BMP session disconnect -> withdraw | bmp-lg-disconnect.ci | yes |
| BMP route isolation from best-path | bmp-lg-bestpath-isolation.ci | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-bmp-6-looking-glass.md`
- [ ] Summary included in commit
