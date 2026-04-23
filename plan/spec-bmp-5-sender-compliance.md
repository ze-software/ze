# Spec: bmp-5-sender-compliance

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/4 |
| Updated | 2026-04-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc7854.md` - BMP wire format, message types, Peer Up OPEN requirements
4. `internal/component/bgp/plugins/bmp/bmp.go` - plugin lifecycle, event handling, sender dispatch
5. `internal/component/bgp/plugins/bmp/msg.go` - BMP message encoding/decoding, BuildSyntheticOpen
6. `internal/component/bgp/plugins/bmp/sender.go` - sender session, writeRouteMonitoring, writePeerUp
7. `internal/component/bgp/server/events.go` - event delivery: getStructuredEvent vs getStructuredStateEvent
8. `internal/component/bgp/message/open.go` - Open struct, WriteTo method
9. `internal/component/bgp/plugins/rib/events/events.go` - BestChangeBatch, BestChangeEntry

## Task

Close four RFC compliance gaps in the BMP sender, documented as follow-up in `plan/learned/574-bgp-4-bmp.md`:

1. **Real OPEN PDUs in Peer Up** (RFC 7854 S4.10) -- replace synthetic 29-byte OPENs with actual negotiated OPEN messages by subscribing to "open" events and caching per peer
2. **Route Mirroring sender** (RFC 7854 S4.7) -- stream verbatim copies of all BGP messages to collectors by subscribing to all message type events and wrapping in Route Mirroring TLVs
3. **Per-NLRI ribout dedup** -- avoid re-sending identical Route Monitoring updates by parsing NLRIs from WireUpdate and tracking per-prefix state per collector
4. **Loc-RIB Route Monitoring** (RFC 9069) -- stream best-path changes as Route Monitoring with PeerType=3 (deferrable if encoder design stalls)

Phases 1-3 are BMP-plugin-contained. Phase 4 crosses the RIB plugin / event system boundary.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - EventDispatcher, DirectBridge, StructuredEvent delivery
  -> Decision: DirectBridge plugins receive *rpc.StructuredEvent via OnStructuredEvent; RawMessage carries wire bytes for all message types (UPDATE, OPEN, NOTIFICATION, KEEPALIVE, ROUTE-REFRESH)
  -> Constraint: state events (SessionStateUp/Down) use getStructuredStateEvent which sets RawMessage=nil; message events use getStructuredEvent which sets RawMessage=*bgptypes.RawMessage

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7854.md` - BMP wire format, Peer Up requires sent+received OPEN PDUs, Route Mirroring carries verbatim BGP messages as TLV type 0
  -> Constraint: Peer Up MUST include complete sent and received BGP OPEN messages (RFC 7854 S4.10); Route Mirroring Per-Peer Header identifies the peer context (S4.7)
- [ ] `rfc/short/rfc9069.md` - DOES NOT EXIST, must create; Loc-RIB monitoring, PeerType=3, F flag
  -> Constraint: RFC 9069 defines Peer Type 3 (Loc-RIB) with no peer address/AS in the per-peer header; F flag (bit 7 of Flags) marks filtered (policy-rejected) routes
- [ ] `rfc/short/rfc8671.md` - DOES NOT EXIST, must create; Adj-RIB-Out support, O flag
  -> Constraint: already implemented -- O flag and L flag set for sent-direction events; Peer Up for Adj-RIB-Out uses the same OPENs as Adj-RIB-In

**Key insights:**
- OPEN events (EventKindOpen) already carry RawMessage with wire bytes via the standard message delivery path in events.go. BMP just doesn't subscribe to them. Subscribing to "open direction received" and "open direction sent" gives the sender raw OPEN bodies before the state event fires.
- State events deliver NO raw message data (RawMessage=nil). The OPEN bytes must come from a separate subscription, cached per peer, and correlated with the subsequent SessionStateUp.
- WriteRouteMirroring encoder exists in msg.go:425 but is unused by the sender. Route Mirroring TLV type 0 wraps a complete BGP message (marker + length + type + body).
- RIB best-path changes are published as BestChangeBatch on EventBus topic (bgp-rib, best-change). Payload is structured (prefix, next-hop, metric, action) -- no wire bytes. Loc-RIB Route Monitoring requires either reconstructing wire UPDATEs or correlating with raw UPDATE events.
- Open.WriteTo(buf, off, nil) in message/open.go can reconstruct complete OPEN wire bytes from the parsed struct. Fallback if event-based OPEN caching has a race.
- The sender's scratch buffer (maxBMPMsgSize = 65535 per senderSession) is already allocated and reused for all message types. Route Mirroring fits within this budget.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - plugin lifecycle; subscribes to "state", "update direction received", "update direction sent"; handleStructuredEvent dispatches state->handleSenderState, update->handleSenderUpdate; handleSenderState builds synthetic OPENs via BuildSyntheticOpen for Peer Up
  -> Constraint: event handler iterates senders serially (one event at a time), so per-sender scratch is safe without locking
- [ ] `internal/component/bgp/plugins/bmp/msg.go` - BuildSyntheticOpen creates 29-byte OPEN (marker+len+type+version+AS(2byte)+hold(90)+routerID(0)+optlen(0)); WriteRouteMirroring exists but unused; all BMP message encoders use WriteTo(buf, off) pattern
  -> Constraint: BMP encoders follow buffer-first WriteTo(buf, off) int pattern; no allocations on the hot path
- [ ] `internal/component/bgp/plugins/bmp/sender.go` - senderSession manages TCP to collector; writeRouteMonitoring synthesizes 19-byte BGP header from body bytes + msgType; scratchFor(need) returns bounded scratch slice
  -> Constraint: scratch buffer is maxBMPMsgSize (65535); messages exceeding this are dropped with error log
- [ ] `internal/component/bgp/plugins/bmp/header.go` - PeerHeader struct with PeerType, Flags (V/L/A/O); PeerTypeLocRIB=3 constant already defined
- [ ] `internal/component/bgp/plugins/bmp/state.go` - bmpState tracks monitored routers and peers; peerKey=(router, distinguisher, address)
- [ ] `internal/component/bgp/server/events.go` - getStructuredEvent populates RawMessage for all message types; getStructuredStateEvent populates State+Reason only (RawMessage=nil); messageTypeToEventKind maps OPEN->EventKindOpen, NOTIFICATION->EventKindNotification, etc.; subscription filtering via GetMatching supports all event kinds
  -> Constraint: StructuredEvent is pooled (GetStructuredEvent/PutStructuredEvent); consumers must not hold references past the event handler return
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - BestChangeBatch has Protocol, Family, Changes[]BestChangeEntry; BestChangeEntry has Action, Prefix, NextHop, Priority, Metric, ProtocolType; published via typed handle BestChange on EventBus
  -> Constraint: BestChangeBatch carries parsed data only; no RawMessage, no wire bytes, no attribute wire encoding
- [ ] `internal/component/bgp/message/open.go` - Open struct with Version, MyAS, HoldTime, BGPIdentifier, ASN4, OptionalParams; WriteTo reconstructs full wire OPEN (header + body); Len() returns total length
  -> Constraint: WriteTo requires buffer >= Len() bytes; extended format adds 4 bytes for OptionalParams > 255

**Behavior to preserve:**
- Existing sender Route Monitoring for Adj-RIB-In (pre-policy) and Adj-RIB-Out (post-policy, RFC 8671)
- Existing Peer Down handling (reason mapping, NOTIFICATION data)
- Existing receiver-side decoding of all 7 BMP message types
- Serial event handling per senderSession (no concurrent writes)
- Scratch buffer allocation pattern (one per senderSession, maxBMPMsgSize)
- Statistics Report periodic timer
- Reconnection with exponential backoff (30s-720s)
- Config structure (YANG list, string values)

**Behavior to change:**
- Peer Up: replace BuildSyntheticOpen with cached real OPEN bytes from event subscription
- New: Route Mirroring sender (currently receiver-only)
- New: per-NLRI dedup tracking in Route Monitoring
- New: Loc-RIB Route Monitoring from RIB best-path events (deferrable)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point -- Phase 1 (Real OPENs)
- Reactor sends/receives OPEN messages -> onMessageReceived/onMessageSent (server/events.go) creates RawMessage with Type=OPEN, RawBytes=body (no header) -> getStructuredEvent populates StructuredEvent with RawMessage -> BMP plugin receives via OnStructuredEvent
- Separately: reactor calls OnPeerEstablished -> apiStateObserver -> OnPeerStateChange -> getStructuredStateEvent (NO RawMessage) -> BMP plugin receives state event

### Entry Point -- Phase 2 (Route Mirroring)
- Same as Phase 1 but for ALL message types (OPEN, UPDATE, NOTIFICATION, KEEPALIVE, ROUTE-REFRESH)
- Each message arrives as StructuredEvent with RawMessage.RawBytes (body only)
- BMP sender wraps: 16-byte marker + 2-byte length + 1-byte type + body -> Route Mirroring TLV type 0 -> BMP Route Mirroring message

### Entry Point -- Phase 3 (Ribout Dedup)
- Same UPDATE path as current Route Monitoring
- Before writeRouteMonitoring: extract NLRIs from RawMessage.WireUpdate -> check per-prefix tracking map -> skip if unchanged

### Entry Point -- Phase 4 (Loc-RIB)
- RIB plugin publishes BestChangeBatch on EventBus topic (bgp-rib, best-change)
- BMP plugin subscribes to EventBus -> receives structured batch (prefix, next-hop, metric, action)
- Must reconstruct BGP UPDATE wire bytes from structured data -> Route Monitoring with PeerType=3

### Transformation Path
1. Wire bytes read by reactor session_read.go -> processMessage -> onMessageReceived callback
2. RawMessage created in reactor_notify.go (body bytes, no header)
3. EventDispatcher delivers to subscribed plugins via DirectBridge StructuredEvent
4. BMP plugin caches/forwards/wraps depending on message type and phase
5. BMP sender encodes into scratch buffer -> TCP write to collector

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor -> EventDispatcher | onMessageReceived/onMessageSent with RawMessage | [ ] |
| EventDispatcher -> BMP Plugin | DirectBridge OnStructuredEvent with StructuredEvent | [ ] |
| BMP Plugin -> Collector | TCP write of BMP-framed messages | [ ] |
| RIB Plugin -> EventBus -> BMP Plugin | BestChangeBatch typed handle (Phase 4 only) | [ ] |

### Integration Points
- `server/events.go:onMessageReceived` - already delivers OPEN/NOTIFICATION/KEEPALIVE as StructuredEvent
- `server/events.go:onPeerStateChange` - delivers state with no raw data (needs correlation, not modification)
- `bmp/bmp.go:SetStartupSubscriptions` - add new event subscriptions
- `bmp/bmp.go:handleStructuredEvent` - add new EventKind cases
- `rib/events/events.go:BestChange` - typed EventBus handle for Loc-RIB subscription (Phase 4)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| reactor OPEN message -> EventDispatcher -> BMP plugin | -> | openCache.store() per peer | TestBMPOpenCaching |
| state event SessionStateUp -> handleSenderState | -> | writePeerUp with cached OPENs | TestBMPPeerUpRealOpen |
| reactor NOTIFICATION -> EventDispatcher -> BMP plugin | -> | writeRouteMirroring with TLV | TestBMPRouteMirroringSend |
| reactor UPDATE -> handleSenderUpdate | -> | NLRI dedup check -> writeRouteMonitoring | TestBMPRiboutDedup |
| EventBus best-change -> BMP plugin | -> | Loc-RIB writeRouteMonitoring PeerType=3 | TestBMPLocRIBRouteMonitoring |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer establishes BGP session (OPEN exchange) | BMP Peer Up message contains the actual sent and received OPEN PDUs (with capabilities), not synthetic 29-byte OPENs |
| AC-2 | Peer OPEN with extended params (>255 bytes) | Peer Up OPEN PDU correctly includes RFC 9072 extended format |
| AC-3 | Peer establishes before sender connects to collector | Peer Up still uses real OPENs (cached from event, not lost) |
| AC-4 | Route Mirroring enabled, peer sends UPDATE | Route Mirroring message sent to collector with TLV type 0 containing verbatim BGP UPDATE (marker + length + type + body) |
| AC-5 | Route Mirroring enabled, peer sends NOTIFICATION | Route Mirroring message sent with TLV type 0 containing verbatim BGP NOTIFICATION |
| AC-6 | Route Mirroring enabled, KEEPALIVE received | Route Mirroring message sent with TLV type 0 containing 19-byte KEEPALIVE (marker + length + type) |
| AC-7 | Same UPDATE forwarded to same collector twice | Second Route Monitoring suppressed (NLRI dedup fires) |
| AC-8 | Different UPDATE for same prefix sent to collector | Route Monitoring sent (dedup recognizes attribute change) |
| AC-9 | Peer down then up again | Dedup state for that peer cleared on peer-down |
| AC-10 | RIB best-path add for 10.0.0.0/24 | Loc-RIB Route Monitoring sent with PeerType=3, prefix 10.0.0.0/24, correct next-hop (Phase 4, deferrable) |
| AC-11 | RIB best-path withdraw for 10.0.0.0/24 | Loc-RIB Route Monitoring sent as BGP UPDATE with withdrawn route (Phase 4, deferrable) |
| AC-12 | Route Mirroring disabled in config | No Route Mirroring messages sent; existing Route Monitoring unaffected |
| AC-13 | BuildSyntheticOpen removed | No references to BuildSyntheticOpen in production code after Phase 1 |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBMPOpenCaching` | `internal/component/bgp/plugins/bmp/bmp_test.go` | OPEN events cached per peer address, retrieved on Peer Up | |
| `TestBMPPeerUpRealOpen` | `internal/component/bgp/plugins/bmp/sender_test.go` | writePeerUp encodes real OPEN bytes (capabilities present) | |
| `TestBMPPeerUpFallbackReconstructed` | `internal/component/bgp/plugins/bmp/bmp_test.go` | If OPEN cache miss, reconstruct via Open.WriteTo (edge case) | |
| `TestBMPRouteMirroringEncode` | `internal/component/bgp/plugins/bmp/msg_test.go` | WriteRouteMirroring produces valid BMP message with TLV type 0 | |
| `TestBMPRouteMirroringSend` | `internal/component/bgp/plugins/bmp/bmp_test.go` | OPEN/NOTIFICATION/KEEPALIVE events trigger Route Mirroring writes | |
| `TestBMPRiboutDedupSameUpdate` | `internal/component/bgp/plugins/bmp/bmp_test.go` | Duplicate UPDATE for same NLRI suppressed | |
| `TestBMPRiboutDedupDifferentAttrs` | `internal/component/bgp/plugins/bmp/bmp_test.go` | Same NLRI with different attributes not suppressed | |
| `TestBMPRiboutDedupPeerDown` | `internal/component/bgp/plugins/bmp/bmp_test.go` | Peer down clears dedup state for that peer | |
| `TestBMPLocRIBRouteMonitoring` | `internal/component/bgp/plugins/bmp/bmp_test.go` | BestChangeBatch -> Route Monitoring with PeerType=3 (Phase 4) | |
| `TestBMPRouteMirroringConfig` | `internal/component/bgp/plugins/bmp/bmp_test.go` | Config toggle enables/disables Route Mirroring per collector | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BMP message size | 6 - 65535 | 65535 | N/A (minimum is common header) | 65536 (exceeds scratch) |
| OPEN PDU size | 29 - 4096 | 4096 (RFC 4271 max) | 28 (below minimum OPEN) | 65535 (RFC 9072 extended) |
| Route Mirroring TLV type | 0-1 | 1 (messages lost) | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bmp-sender-peer-up-open` | `test/plugin/bmp-sender-peer-up-open.ci` | Peer establishes; collector receives Peer Up with real OPEN capabilities | |
| `bmp-sender-route-mirroring` | `test/plugin/bmp-sender-route-mirroring.ci` | Peer sends UPDATE; collector receives both Route Monitoring and Route Mirroring | |

### Future (if deferring any tests)
- Phase 4 Loc-RIB functional test: requires EventBus subscription from BMP plugin + UPDATE wire reconstruction -- deferred until encoder design resolved

## Files to Modify
- `internal/component/bgp/plugins/bmp/bmp.go` - add subscriptions for "open"/"notification"/"keepalive"/"refresh"; add OPEN cache; add EventKind cases in handleStructuredEvent; add Route Mirroring dispatch; add ribout dedup tracking
- `internal/component/bgp/plugins/bmp/sender.go` - add writeRouteMirroring method; add Loc-RIB writeRouteMonitoring variant with PeerType=3
- `internal/component/bgp/plugins/bmp/msg.go` - remove BuildSyntheticOpen; add Route Mirroring TLV constants (type 0 = BGP Message, type 1 = Messages Lost)
- `internal/component/bgp/plugins/bmp/bmp_test.go` - new tests for OPEN caching, Route Mirroring, dedup
- `internal/component/bgp/plugins/bmp/sender_test.go` - new tests for writePeerUp with real OPENs, writeRouteMirroring
- `internal/component/bgp/plugins/bmp/msg_test.go` - new tests for Route Mirroring encode/decode roundtrip
- `internal/component/bgp/plugins/bmp/schema/ze-bmp-conf.yang` - add route-mirroring leaf under sender config

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [x] | `internal/component/bgp/plugins/bmp/schema/ze-bmp-conf.yang` |
| CLI commands/flags | [ ] | N/A - existing "bmp collectors" shows status |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new feature | [x] | `test/plugin/bmp-sender-peer-up-open.ci`, `test/plugin/bmp-sender-route-mirroring.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - Route Mirroring sender, real OPEN PDUs |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - route-mirroring option |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` - BMP plugin capabilities updated |
| 6 | Has a user guide page? | [x] | `docs/guide/bmp.md` - Route Mirroring, dedup, Loc-RIB sections |
| 7 | Wire format changed? | [ ] | N/A (standard BMP wire) |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc9069.md` (create), `rfc/short/rfc8671.md` (create) |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - Route Mirroring, Loc-RIB columns |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `rfc/short/rfc9069.md` - RFC 9069 summary (Loc-RIB support for BMP)
- `rfc/short/rfc8671.md` - RFC 8671 summary (Adj-RIB-Out support, already implemented but undocumented)
- `test/plugin/bmp-sender-peer-up-open.ci` - functional test for real OPEN PDUs in Peer Up
- `test/plugin/bmp-sender-route-mirroring.ci` - functional test for Route Mirroring

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. /ze-review gate | Review Gate section -- run `/ze-review`; fix every BLOCKER/ISSUE; re-run until only NOTEs remain (BEFORE full verification) |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Real OPEN PDUs** -- Replace synthetic OPENs with cached real OPEN bytes
   - Add "open direction received" and "open direction sent" to subscriptions
   - Add per-peer OPEN cache (map[string]openPair with sentOpen/recvOpen bytes)
   - On EventKindOpen: extract RawMessage.RawBytes, synthesize BGP header, cache per peer
   - On SessionStateUp: look up cached OPENs; use in writePeerUp
   - On SessionStateDown: clear cached OPENs for that peer
   - Remove BuildSyntheticOpen
   - Tests: TestBMPOpenCaching, TestBMPPeerUpRealOpen, TestBMPPeerUpFallbackReconstructed
   - Files: bmp.go, msg.go, bmp_test.go, sender_test.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Route Mirroring sender** -- Stream verbatim BGP messages as Route Mirroring
   - Add "notification", "keepalive", "refresh" to subscriptions (OPEN already added in Phase 1)
   - Add route-mirroring config leaf in YANG schema
   - Add Route Mirroring TLV type constants (type 0 = BGP Message, type 1 = Messages Lost)
   - Add writeRouteMirroring to senderSession (wrap complete BGP PDU in TLV type 0)
   - Add handleSenderMirror: for all EventKinds (open, update, notification, keepalive, refresh), synthesize BGP header from body + type, wrap in Route Mirroring
   - Config: per-collector or global route-mirroring enabled/disabled
   - Tests: TestBMPRouteMirroringEncode, TestBMPRouteMirroringSend, TestBMPRouteMirroringConfig
   - Files: bmp.go, sender.go, msg.go, schema/ze-bmp-conf.yang, bmp_test.go, msg_test.go
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Per-NLRI ribout dedup** -- Track per-prefix state to avoid redundant Route Monitoring
   - Add per-collector per-peer prefix tracking: map[peerAddr]map[nlriKey]uint64 (attribute hash)
   - In handleSenderUpdate: before writeRouteMonitoring, extract NLRIs from WireUpdate
   - Hash attribute wire bytes (RawMessage.RawBytes excluding NLRI sections) per UPDATE
   - For each NLRI prefix: compare current attribute hash with tracked; skip Route Monitoring if unchanged
   - On peer-down: clear tracked state for that peer
   - Bound map growth: cap per-peer prefix count
   - Tests: TestBMPRiboutDedupSameUpdate, TestBMPRiboutDedupDifferentAttrs, TestBMPRiboutDedupPeerDown
   - Files: bmp.go, bmp_test.go
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Loc-RIB Route Monitoring** (deferrable) -- Stream best-path changes as Route Monitoring
   - Subscribe to EventBus (bgp-rib, best-change) via BestChange.Subscribe
   - On BestChangeBatch: for each BestChangeEntry, build BGP UPDATE wire bytes
   - UPDATE construction: withdrawn routes for withdraw action; NLRI + attributes for add/update
   - Attributes needed: ORIGIN, AS_PATH (empty for Loc-RIB), NEXT_HOP from BestChangeEntry
   - Write Route Monitoring with PeerHeader PeerType=3 (Loc-RIB), no peer address
   - Tests: TestBMPLocRIBRouteMonitoring
   - Files: bmp.go, sender.go, bmp_test.go
   - Verify: tests fail -> implement -> tests pass

5. **Functional tests** -> Create after feature works. Cover user-visible behavior.
6. **RFC refs** -> Add `// RFC 7854 Section X.Y` comments (protocol work only)
7. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)
8. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-bmp-5-sender-compliance.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | OPEN cache populated before state event fires; Route Mirroring TLV type is correct (0 for BGP message); dedup correctly identifies unchanged vs changed prefixes |
| Naming | Config leaf uses kebab-case (route-mirroring); YANG uses kebab-case |
| Data flow | OPEN events arrive before state events (reactor processes OPEN -> Established -> state notification); no race between OPEN cache write and state event read |
| Rule: no-layering | BuildSyntheticOpen fully removed after Phase 1 |
| Rule: buffer-first | All new BMP message encoding uses WriteTo(buf, off) int pattern; no []byte returns |
| Rule: no-alloc-hot-path | Route Mirroring and dedup must not allocate per event (use scratch buffer, pre-allocated maps) |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Real OPEN PDUs in Peer Up | `grep -r "BuildSyntheticOpen" internal/` returns only test references |
| Route Mirroring sender | `grep -r "writeRouteMirroring\|MsgRouteMirroring" internal/component/bgp/plugins/bmp/sender.go` |
| Per-NLRI ribout dedup | `grep -r "dedup\|ribout\|nlriTrack" internal/component/bgp/plugins/bmp/` |
| YANG config update | `grep "route-mirroring" internal/component/bgp/plugins/bmp/schema/ze-bmp-conf.yang` |
| RFC 9069 summary | `ls rfc/short/rfc9069.md` |
| RFC 8671 summary | `ls rfc/short/rfc8671.md` |
| Functional tests | `ls test/plugin/bmp-sender-peer-up-open.ci test/plugin/bmp-sender-route-mirroring.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | OPEN cache: bound cache size per peer count (prevent memory exhaustion from many peers); clear on peer-down |
| Resource exhaustion | Ribout dedup map: bound per-peer prefix count; Route Mirroring: don't amplify traffic (one mirror per message, not per collector if disabled) |
| Buffer overflow | All WriteTo calls must check scratch buffer bounds via scratchFor; Route Mirroring TLV length must not exceed maxBMPMsgSize |
| Untrusted input | RawMessage.RawBytes from reactor are trusted (internal code); collector TCP is outbound-only (no inbound data processed) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

## Design Alternatives

### Phase 1: Real OPEN PDUs

| Approach | How | Trade-offs |
|---|---|---|
| **A: Event-based OPEN caching (chosen)** | Subscribe to "open direction received/sent"; cache RawBytes per peer; correlate with state event | +simple, +uses existing infra, +actual wire bytes. -implicit ordering dependency |
| B: Reconstruct from parsed Open struct | Modify getStructuredStateEvent to carry *message.Open; use Open.WriteTo() | +single event. -modifies shared infra, -reconstructed bytes may differ from wire |

### Phase 2: Route Mirroring

| Approach | How | Trade-offs |
|---|---|---|
| **A: Subscribe all message types (chosen)** | Add "notification"/"keepalive"/"refresh" subs; handleSenderMirror wraps in TLV type 0 | +plugin-contained, +encoder exists. -more events delivered to BMP |
| B: Reactor raw PDU stream | New notification channel in reactor for raw messages | +single sub. -new reactor infra, -breaks plugin isolation |

### Phase 3: Ribout Dedup

| Approach | How | Trade-offs |
|---|---|---|
| A: MessageID tracking per NLRI | Track messageID per prefix; suppress same ID | +simple. -messageID never repeats so never actually dedup |
| **B: Attribute hash per NLRI (chosen)** | Hash attribute wire bytes per prefix; suppress when hash unchanged | +detects actual unchanged routes. -hash cost, -collision risk |

### Phase 4: Loc-RIB Route Monitoring (deferrable)

| Approach | How | Trade-offs |
|---|---|---|
| **A: Lightweight UPDATE builder (chosen)** | Build minimal BGP UPDATE from BestChangeEntry (ORIGIN=IGP, AS_PATH=empty, NEXT_HOP) | +self-contained, +semantically correct for Loc-RIB. -parallel encoding |
| B: Correlate raw UPDATEs with best-path | Cache recent UPDATEs indexed by prefix; look up on best-change | +wire fidelity. -complex, -multi-prefix UPDATEs, -memory |

### Failure Modes

| Mode | Impact | Mitigation |
|---|---|---|
| OPEN cache miss on SessionStateUp | Peer Up without OPENs | Log warning; skip Peer Up (collector sees Route Monitoring without prior Peer Up) |
| Stale OPEN cache entry (peer fails negotiation) | Memory leak | Clear on any peer-down; bound cache size |
| Route Mirroring exceeds scratch buffer | Message dropped | scratchFor returns error (existing pattern) |
| Dedup map unbounded growth | Memory exhaustion | Cap per-peer prefix count |
| Loc-RIB UPDATE builder produces invalid wire | Collector rejects | Round-trip test: build -> decode -> verify |

### Triple Challenge

| Challenge | Answer |
|---|---|
| Simplicity | Phase 1: 2 maps + ~50 lines. Phase 2: new handler + existing encoder. Phase 3: hash map. Phase 4: minimal builder. No simpler approach for any phase. |
| Uniformity | All phases: StructuredEvent -> encode scratch -> writeMsg. Phase 4 adds EventBus sub (follows RIB subscriber pattern). No new patterns. |
| Performance | Phase 1: map lookup on cold path. Phase 2: scratch encode per message. Phase 3: WireUpdate extraction (zero-copy) + hash. Phase 4: lightweight encode. No per-event allocations. |

## Design Insights

## RFC Documentation

Add `// RFC 7854 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: Peer Up OPEN requirement (S4.10), Route Mirroring TLV format (S4.7), Loc-RIB PeerType=3 (RFC 9069).

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

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

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-bmp-5-sender-compliance.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
