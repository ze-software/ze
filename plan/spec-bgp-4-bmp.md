# Spec: BMP Receiver + Sender (RFC 7854)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc7854.md` - BMP base spec (create if missing)
4. `internal/component/bgp/plugins/` - where new BGP plugins live
5. `internal/component/bgp/message/` - shared BGP message decoder
6. `internal/component/bgp/reactor/reactor_notify.go` - event dispatch for sender
7. `internal/component/bgp/reactor/session.go` - OPEN message access for sender
8. `internal/component/bgp/plugins/rpki/rtr_session.go` - outbound TCP client pattern
9. `docs/features.md` and `docs/comparison.md` - "No BMP" entries to update

## Task

BGP Monitoring Protocol (RFC 7854) is the de-facto standard for ingesting and
exporting live peer state and Adj-RIB views for observability and analysis.
Operators expect a modern BGP daemon to both accept BMP feeds from their fleet
and send its own BGP state to external collectors.

Ze does not currently implement BMP. This spec covers **both directions**:

**Receiver:** ze accepts TCP connections from remote routers that speak BMP v3,
parses the wrapped BGP messages with the existing message decoder, and
materializes the monitored peer state into a queryable view.

**Sender:** ze connects to one or more external BMP collectors and streams its
own peer state changes, route updates, and statistics as BMP messages.

Implementation is split into four phases: shared wire format, receiver, sender,
documentation and verification.

### Out of Scope

- Adj-RIB-Out monitoring (RFC 8671) - follow-up spec
- Local RIB / Loc-RIB monitoring (RFC 9069) - follow-up spec
- Route Mirroring (Type 6) encoding on the sender side - follow-up
- Time-series storage of monitored routes (external, plugin-driven)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - plugin model
  -> Constraint: plugins register via init() in register.go; core discovers through registries
- [ ] `.claude/patterns/plugin.md` - how to register a new plugin
  -> Constraint: RunEngine entry point receives net.Conn; uses SDK 5-stage protocol
- [ ] `.claude/patterns/config-option.md` - YANG container vs list + listener extension
- [ ] `.claude/rules/config-design.md` - listener structure
  -> Constraint: every listener uses `zt:listener` + `ze:listener` extension
- [ ] `.claude/rules/buffer-first.md` - wire encoding into pooled bounded buffers
  -> Constraint: all wire encoding via WriteTo(buf, off) int; no append, no make in helpers
- [ ] `internal/component/bgp/plugins/rpki/rtr_session.go` - outbound TCP client pattern
  -> Constraint: long-lived goroutine per session, reconnect with backoff, shutdown via channel

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7854.md` - BMP v3 base spec (create before implementing)
  -> Constraint: version must be 3; common header 6 bytes; per-peer header 42 bytes
  -> Constraint: Initiation and Termination have no per-peer header
  -> Constraint: IANA default port 11019
  -> Constraint: unidirectional data flow (router -> collector)
  -> Constraint: Route Monitoring wraps standard BGP UPDATE messages
  -> Constraint: IPv4 addresses stored as IPv4-mapped IPv6 (::ffff:x.x.x.x) in 16-byte field
  -> Constraint: reconnect intervals: 30s min, 720s max suggested
- [ ] `rfc/short/rfc8671.md` - Adj-RIB-Out addition (follow-up spec)
- [ ] `rfc/short/rfc9069.md` - Local RIB monitoring (follow-up spec)

**Key insights:**
- BMP wraps BGP messages in a framing header. Inner BGP messages can be decoded
  by the existing wire decoder once the outer frame is stripped.
- BMP uses TCP; every session is unidirectional (router -> collector).
- The wire format is identical in both directions -- same headers, message types,
  TLV encoding. A shared wire package serves both receiver and sender.
- GoBGP implements sender; bio-routing implements receiver. Both studied.
- Per-peer header flags determine interpretation: V=IPv6, L=post-policy,
  A=2-byte-AS, O=Adj-RIB-Out.
- Statistics Report uses 14 standard counter types (RFC 7854 section 4.8).
- Peer Down has 5 reason codes mapping to different FSM/NOTIFICATION scenarios.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/` - existing sub-plugin shape
  -> Constraint: register.go + schema/ subdir + RunXxxPlugin() entry point
- [ ] `internal/component/bgp/message/parse.go` - message-level decoder
  -> Constraint: decodes UPDATE, OPEN, NOTIFICATION, KEEPALIVE from raw bytes
- [ ] `internal/component/bgp/wire/` - WireUpdate lazy access
- [ ] `internal/yang/modules/ze-bgp-conf.yang` - where new plugin config lives
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - event dispatch
  -> Constraint: PeerLifecycleObserver interface: OnPeerEstablished, OnPeerClosed
  -> Constraint: EventDispatcher delivers structured events to subscribed plugins
- [ ] `internal/component/bgp/reactor/session.go` - OPEN messages
  -> Constraint: Session.localOpen and Session.peerOpen hold BGP OPEN messages
- [ ] `internal/component/bgp/reactor/peer_stats.go` - per-peer counters
  -> Constraint: UpdatesReceived, UpdatesSent, KeepalivesReceived, etc.
- [ ] `internal/component/bgp/plugins/rpki/rtr_session.go` - outbound TCP pattern
  -> Constraint: Dial with timeout, retry loop, shutdown via stopCh channel
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - config-driven session lifecycle
  -> Constraint: startSessions() from OnConfigure, stop old on reload

**Behavior to preserve:**
- Existing BGP peer sessions, reactor, RIB operation unchanged
- The existing wire parser remains the canonical way to decode BGP messages
- Plugin event bus behavior unchanged

**Behavior to change:**
- Add a new listener (TCP, configurable ip + port) that accepts BMP sessions (receiver)
- Add outbound TCP clients that connect to BMP collectors (sender)
- Add a new plugin that owns both listener and clients, parses/generates BMP framing,
  reuses the BGP decoder for inner BGP messages
- Receiver publishes monitored peer state on the plugin bus
- Sender subscribes to reactor peer/update events and streams them as BMP
- Add queryable views: CLI + web + RPC for monitored peers (receiver) and collector
  status (sender)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Memory Pool Design

BMP uses pooled buffers for both read and write paths, following ze's buffer-first
discipline. Simpler than BGP's dual peer pool model because BMP messages are consumed
once -- no shared read-only forwarding, no copy-on-modify.

**Receiver read pool:** sync.Pool of bounded buffers (one per session). TCP read loop
acquires a buffer, reads the BMP message into it, parses in-place. The inner BGP UPDATE
bytes are a slice of the same buffer -- handed to the existing decoder without copying.
Buffer released after processing completes.

**Sender write pool:** sync.Pool of bounded buffers (one per collector). BMP message
serialization writes into the pooled buffer via WriteTo(buf, off) int, then
conn.Write(buf[:n]), then release. One buffer handles the encode-write-release cycle.

**Buffer sizing:** RFC 7854 length field is uint32 but practical BMP messages are bounded
by BGP max message size (4096) plus BMP framing (6 common + 42 per-peer = 48). Pool
buffer size: 4144 bytes covers the common case. Oversized messages (large UPDATE in
initial RIB dump) use a one-off allocation returned to the pool only if within the
pool's max size.

**Lifecycle:**

| Path | Allocate | Use | Release |
|------|----------|-----|---------|
| Receiver read | pool.Get() at session read loop top | TCP read + in-place parse + inner BGP decode | pool.Put() after message fully processed |
| Sender write | pool.Get() at event handler | WriteTo serialization + conn.Write | pool.Put() after write completes or fails |

### Receiver Data Flow

#### Entry Point
- TCP connection on configured BMP listener (default port 11019)
- First bytes: BMP Common Header (version==3, message type, length)

#### Transformation Path
1. TCP accept - spawn per-session goroutine
2. Acquire buffer from receiver read pool
3. Read BMP Common Header (6 bytes) into buffer - validate version, extract length and type
4. Read remaining message bytes into same buffer
5. Parse Per-Peer Header (42 bytes) for types 0-3 -- offset-based, no copy
6. Dispatch on message type:
   - Route Monitoring (0): inner BGP bytes are buf[48:length] -- hand slice to existing
     `internal/component/bgp/message` decoder
   - Statistics Report (1): decode TLV counters from buffer, update monitored peer stats
   - Peer Down (2): decode reason from buffer, mark monitored peer down, drop RIB snapshot
   - Peer Up (3): decode local addr/ports + OPEN messages from buffer, add monitored peer
   - Initiation (4): decode TLVs (sysName, sysDescr) from buffer, identify router
   - Termination (5): decode reason TLVs from buffer, close session cleanly
   - Route Mirroring (6): decode TLVs from buffer, log/forward raw BGP PDUs
7. Release buffer to receiver read pool
8. Publish events to plugin bus (namespace `bmp`)
9. Update monitored peer map and Adj-RIB-In snapshots

### Sender Data Flow

#### Entry Point
- Config enables BMP sender with one or more collector endpoints (ip + port)
- Plugin subscribes to reactor events via OnStructuredEvent

#### Transformation Path
1. Connect to collector via TCP (outbound, with exponential backoff)
2. Acquire buffer from sender write pool
3. Serialize Initiation message into buffer via WriteTo(buf, off), conn.Write, release
4. For each already-established peer: acquire buffer, serialize Peer Up (with localOpen + peerOpen), write, release
5. Ongoing event loop:
   - Peer reaches Established: acquire buffer, serialize Peer Up with OPEN messages, write, release
   - Peer leaves Established: acquire buffer, serialize Peer Down with reason code, write, release
   - UPDATE received: acquire buffer, wrap in BMP Route Monitoring frame, write, release
   - Timer fires: acquire buffer, collect per-peer stats, serialize Statistics Report, write, release
6. On shutdown: acquire buffer, serialize Termination message, write, release, close connection
7. On connection loss: reconnect with exponential backoff (30s-720s)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| TCP listener -> plugin (receiver) | new listener under `bmp` plugin | [ ] |
| Inner BGP bytes -> decoder (receiver) | reuse `internal/component/bgp/message` | [ ] |
| Plugin -> bus (receiver) | events in `bmp/*` namespace | [ ] |
| Plugin -> CLI/web (both) | new RPCs via plugin registration | [ ] |
| Reactor events -> plugin (sender) | OnStructuredEvent subscription | [ ] |
| Plugin -> TCP (sender) | BMP message serialization -> conn.Write | [ ] |
| Peer FSM -> OPEN access (sender) | via structured event peer metadata | [ ] |

### Integration Points

#### Receiver
- Plugin registration: `internal/component/bgp/plugins/bmp/register.go`
- YANG: new container `ze-bgp-conf:bmp` with receiver listener + sender collector list
- CLI: `ze bmp sessions`, `ze bmp peers`, `ze bmp rib <peer>`
- Web UI: read-only monitoring pane

#### Sender
- Event subscription: `bgp` namespace, `state` + `update` event types
- Peer OPEN access: via structured event metadata (localOpen, peerOpen)
- Per-peer statistics: via `peer.Stats()` counters
- CLI: `ze bmp collectors` (connection status)

### Architectural Verification
- [ ] No bypass: BMP inner BGP parsing goes through existing decoder (receiver)
- [ ] No coupling: BMP receiver does not write into the main RIB
- [ ] No duplication: BMP reuses the wire decoder (receiver) and existing event system (sender)
- [ ] Zero-copy preserved where applicable (inner bytes are a slice of pooled read buffer)
- [ ] Sender uses buffer-first encoding (WriteTo pattern, pooled write buffers)
- [ ] Pool buffers acquired at loop top, released after processing/write completes
- [ ] No make([]byte) in message encode/decode helpers -- all use provided buffer + offset

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| BMP config with receiver listener | -> | plugin registers and binds TCP listener | `TestBMPListenerStartsFromConfig` |
| Remote router connects to receiver | -> | session goroutine parses headers | `TestBMPSessionAccepts` |
| Route Monitoring message arrives | -> | inner BGP decoded via shared decoder | `TestBMPRouteMonitoringParsed` |
| BMP config with collector endpoint | -> | plugin connects outbound TCP | `TestBMPSenderConnects` |
| Peer reaches Established | -> | sender sends Peer Up to collector | `TestBMPSenderPeerUp` |
| BGP UPDATE received | -> | sender sends Route Monitoring to collector | `TestBMPSenderRouteMonitoring` |
| CLI `ze bmp sessions` | -> | plugin RPC returns session list | `test/plugin/bmp-list-sessions.ci` |
| CLI `ze bmp peers` | -> | plugin RPC returns monitored peer state | `test/plugin/bmp-list-peers.ci` |
| CLI `ze bmp collectors` | -> | plugin RPC returns collector status | `test/plugin/bmp-list-collectors.ci` |

## Acceptance Criteria

### Phase 1: Shared Wire Format

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Raw bytes with valid BMP Common Header | Decoder extracts version=3, length, type correctly |
| AC-2 | Raw bytes with valid Per-Peer Header | Decoder extracts peer type, flags, address, AS, BGP ID, timestamps |
| AC-3 | Per-Peer Header with V flag set | Peer address interpreted as IPv6 |
| AC-4 | Per-Peer Header with V flag clear | Peer address interpreted as IPv4-mapped-IPv6 |
| AC-5 | BMP version != 3 | Decoder returns error, does not panic |
| AC-6 | Message length shorter than minimum for type | Decoder returns error |
| AC-7 | Initiation TLVs (sysName, sysDescr, string) | TLV decoder extracts type + value correctly |
| AC-8 | Statistics Report with counter TLVs | Decoder extracts stat type + 64-bit gauge values |
| AC-9 | Serialized Common Header + Per-Peer Header | Round-trip: encode then decode produces identical fields |

### Phase 2: Receiver

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-10 | Config enables `bmp` receiver with `ip` + `port` | Plugin binds a TCP listener on that address |
| AC-11 | Port collision with another listener | YANG validation rejects at commit time (listener extension) |
| AC-12 | Remote router opens a TCP session | Plugin accepts, reads BMP Common Header, validates version==3 |
| AC-13 | Initiation message received | Router identification fields (sysName, sysDescr) captured and queryable |
| AC-14 | Peer Up Notification received | Monitored peer appears in `ze bmp peers` with OPEN details |
| AC-15 | Route Monitoring (BGP UPDATE wrapped) | Inner UPDATE decoded via existing BGP decoder; prefixes visible via `ze bmp rib <peer>` |
| AC-16 | Peer Down Notification received | Monitored peer marked down, RIB snapshot dropped |
| AC-17 | Statistics Report received | Per-peer counters updated and queryable |
| AC-18 | Termination message received | Session closed cleanly |
| AC-19 | Malformed BMP Common Header | Session closed; error logged; other sessions unaffected |
| AC-20 | Malformed inner BGP message | Error surfaced; monitored peer marked errored; session stays up |
| AC-21 | Config reload disabling BMP receiver | Listener and all sessions stopped, plugin emits shutdown event |
| AC-22 | Fuzz corpus from existing BGP fuzzer replayed as BMP inner messages | No panic, no deadlock |

### Phase 3: Sender

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-23 | Config enables BMP sender with collector `ip` + `port` | Plugin connects outbound TCP to collector |
| AC-24 | Collector unreachable | Plugin retries with exponential backoff (30s min, 720s max) |
| AC-25 | Connection to collector established | Initiation message sent with ze sysName and sysDescr |
| AC-26 | BGP peer reaches Established state | Peer Up sent to collector with sent and received OPEN messages |
| AC-27 | BGP peer leaves Established state | Peer Down sent with correct reason code (local notification, remote notification, de-configured, etc.) |
| AC-28 | BGP UPDATE received from peer (pre-policy) | Route Monitoring message sent wrapping the UPDATE |
| AC-29 | Statistics timer fires | Statistics Report sent with per-peer counters (routes received, rejected, etc.) |
| AC-30 | Config has multiple collectors | Plugin maintains independent connection to each |
| AC-31 | Collector connection drops | Plugin reconnects with backoff; re-sends Initiation + Peer Up for all established peers |
| AC-32 | Config reload removing a collector | Connection to that collector closed with Termination message |
| AC-33 | Config reload adding a collector | New connection established without disturbing existing collectors |
| AC-34 | ze shutdown | Termination message sent to all collectors before TCP close |
| AC-35 | Per-peer ribout dedup | Identical route not re-sent; only diffs transmitted |

### Phase 4: Documentation and Verification

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-36 | `rfc/short/rfc7854.md` | RFC summary exists and covers all 7 message types |
| AC-37 | `docs/guide/bmp.md` | User guide covers both receiver and sender configuration |
| AC-38 | `docs/features.md` | BMP listed as supported feature |
| AC-39 | `docs/comparison.md` | BMP row updated from "No" to "BMP receiver + sender (RFC 7854)" |
| AC-40 | `make ze-verify` | All tests pass |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBMPCommonHeaderDecode` | `internal/component/bgp/plugins/bmp/header_test.go` | Common Header parser (6 bytes) | |
| `TestBMPCommonHeaderEncode` | `internal/component/bgp/plugins/bmp/header_test.go` | Common Header serializer | |
| `TestBMPCommonHeaderRoundTrip` | `internal/component/bgp/plugins/bmp/header_test.go` | Encode then decode identity | |
| `TestBMPPeerHeaderDecode` | `internal/component/bgp/plugins/bmp/header_test.go` | Per-Peer Header parser (42 bytes) | |
| `TestBMPPeerHeaderEncode` | `internal/component/bgp/plugins/bmp/header_test.go` | Per-Peer Header serializer | |
| `TestBMPPeerHeaderFlags` | `internal/component/bgp/plugins/bmp/header_test.go` | V, L, A, O flag interpretation | |
| `TestBMPPeerHeaderIPv4Mapped` | `internal/component/bgp/plugins/bmp/header_test.go` | IPv4 stored as ::ffff:x.x.x.x | |
| `TestBMPTLVDecode` | `internal/component/bgp/plugins/bmp/tlv_test.go` | TLV type + length + value extraction | |
| `TestBMPTLVEncode` | `internal/component/bgp/plugins/bmp/tlv_test.go` | TLV serialization | |
| `TestBMPInitiationDecode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Initiation message with sysName, sysDescr TLVs | |
| `TestBMPInitiationEncode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Initiation message serialization | |
| `TestBMPTerminationDecode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Termination message with reason TLVs | |
| `TestBMPPeerUpDecode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Peer Up: local addr, ports, sent/received OPEN | |
| `TestBMPPeerUpEncode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Peer Up serialization for sender | |
| `TestBMPPeerDownDecode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Peer Down: all 5 reason codes | |
| `TestBMPPeerDownEncode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Peer Down serialization for sender | |
| `TestBMPRouteMonitoringDecode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Route Monitoring wraps existing BGP decoder | |
| `TestBMPRouteMonitoringEncode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Route Monitoring wraps BGP UPDATE bytes for sender | |
| `TestBMPStatisticsReportDecode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Statistics Report with counter TLVs | |
| `TestBMPStatisticsReportEncode` | `internal/component/bgp/plugins/bmp/msg_test.go` | Statistics Report serialization for sender | |
| `TestBMPRouteMonitoringInnerBGP` | `internal/component/bgp/plugins/bmp/monitor_test.go` | Inner UPDATE decoded via shared decoder | |
| `TestBMPListenerStartsFromConfig` | `internal/component/bgp/plugins/bmp/listener_test.go` | Config triggers listener | |
| `TestBMPSessionAccepts` | `internal/component/bgp/plugins/bmp/session_test.go` | Accept + initial handshake | |
| `TestBMPMalformedHeaderDrops` | `internal/component/bgp/plugins/bmp/session_test.go` | Bad framing closes session without panic | |
| `TestBMPSenderConnects` | `internal/component/bgp/plugins/bmp/sender_test.go` | Outbound TCP connection to collector | |
| `TestBMPSenderReconnect` | `internal/component/bgp/plugins/bmp/sender_test.go` | Exponential backoff on connection failure | |
| `TestBMPSenderInitiation` | `internal/component/bgp/plugins/bmp/sender_test.go` | Initiation sent on connect | |
| `TestBMPSenderPeerUp` | `internal/component/bgp/plugins/bmp/sender_test.go` | Peer Up sent when peer reaches Established | |
| `TestBMPSenderPeerDown` | `internal/component/bgp/plugins/bmp/sender_test.go` | Peer Down sent with correct reason mapping | |
| `TestBMPSenderRouteMonitoring` | `internal/component/bgp/plugins/bmp/sender_test.go` | Route Monitoring wraps BGP UPDATE for collector | |
| `TestBMPSenderStatistics` | `internal/component/bgp/plugins/bmp/sender_test.go` | Periodic statistics report | |
| `TestBMPSenderTermination` | `internal/component/bgp/plugins/bmp/sender_test.go` | Termination sent on shutdown | |
| `TestBMPSenderRiboutDedup` | `internal/component/bgp/plugins/bmp/sender_test.go` | Identical path not re-sent | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BMP version | 3 only | 3 | 2 | 4 |
| Common Header length | 6..max-uint32 | max-uint32 | 5 | N/A (uint32) |
| Message type | 0..6 | 6 | N/A (uint8 0) | 7 |
| Per-Peer Header peer type | 0..3 | 3 | N/A (uint8 0) | 4 |
| TLV type | 0..max-uint16 | max-uint16 | N/A | N/A |
| TLV length | 0..max-uint16 | max-uint16 | N/A | N/A |
| TCP listen port | 1..65535 | 65535 | 0 | 65536 |
| Statistics timeout | 0..65535 | 65535 (0=disabled) | N/A | N/A |
| Peer Down reason | 1..5 | 5 | 0 | 6 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-bmp-sessions` | `test/plugin/bmp-sessions.ci` | Operator configures BMP receiver, synthetic BMP client connects | |
| `test-bmp-peers` | `test/plugin/bmp-peers.ci` | Monitored peer Up/Down visible in `ze bmp peers` | |
| `test-bmp-rib` | `test/plugin/bmp-rib.ci` | Monitored routes visible in `ze bmp rib` | |
| `test-bmp-collectors` | `test/plugin/bmp-collectors.ci` | Sender connects to test collector, streams Initiation + Peer Up | |
| `test-bmp-sender-updates` | `test/plugin/bmp-sender-updates.ci` | Sender streams Route Monitoring when peer receives UPDATE | |

### Future (follow-up specs)
- Adj-RIB-Out monitoring (RFC 8671)
- Local RIB / Loc-RIB monitoring (RFC 9069)
- Route Mirroring encoding on sender side

## Files to Modify
- `internal/yang/modules/ze-bgp-conf.yang` - add `bmp` container with receiver listener + sender collector list
- `internal/component/bgp/plugins/all.go` (or equivalent registrar) - import new plugin
- `docs/features.md` - flip "No BMP" to "BMP receiver + sender (RFC 7854)"
- `docs/comparison.md` - update BMP row

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/yang/modules/ze-bgp-conf.yang` |
| CLI commands/flags | Yes | new RPCs registered by the plugin |
| Editor autocomplete | Yes (auto if YANG) | - |
| Functional test for new RPC/API | Yes | `test/plugin/bmp-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc7854.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - BMP test harness |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |

## Files to Create
- `internal/component/bgp/plugins/bmp/doc.go` - package doc + RFC anchor
- `internal/component/bgp/plugins/bmp/register.go` - plugin registration
- `internal/component/bgp/plugins/bmp/header.go` - Common Header + Per-Peer Header encode/decode
- `internal/component/bgp/plugins/bmp/tlv.go` - TLV encode/decode (shared by all message types)
- `internal/component/bgp/plugins/bmp/msg.go` - all 7 message types: encode/decode dispatch
- `internal/component/bgp/plugins/bmp/listener.go` - TCP accept loop (receiver)
- `internal/component/bgp/plugins/bmp/session.go` - per-connection receive loop (receiver)
- `internal/component/bgp/plugins/bmp/monitor.go` - Route Monitoring inner BGP decode (receiver)
- `internal/component/bgp/plugins/bmp/state.go` - monitored peer map + Adj-RIB-In snapshots (receiver)
- `internal/component/bgp/plugins/bmp/pool.go` - sync.Pool wrappers for read and write buffers
- `internal/component/bgp/plugins/bmp/sender.go` - outbound TCP client + event loop (sender)
- `internal/component/bgp/plugins/bmp/ribout.go` - per-NLRI dedup for sender
- `internal/component/bgp/plugins/bmp/stats.go` - statistics collection + serialization (sender)
- `internal/component/bgp/plugins/bmp/header_test.go`
- `internal/component/bgp/plugins/bmp/tlv_test.go`
- `internal/component/bgp/plugins/bmp/msg_test.go`
- `internal/component/bgp/plugins/bmp/monitor_test.go`
- `internal/component/bgp/plugins/bmp/listener_test.go`
- `internal/component/bgp/plugins/bmp/session_test.go`
- `internal/component/bgp/plugins/bmp/sender_test.go`
- `internal/component/bgp/plugins/bmp/schema/register.go` - YANG module registration
- `internal/component/bgp/plugins/bmp/schema/embed.go` - go:embed for YANG
- `internal/component/bgp/plugins/bmp/schema/ze-bmp-conf.yang` - BMP configuration schema
- `rfc/short/rfc7854.md` - RFC short summary
- `docs/guide/bmp.md` - user guide
- `test/plugin/bmp-sessions.ci`
- `test/plugin/bmp-peers.ci`
- `test/plugin/bmp-rib.ci`
- `test/plugin/bmp-collectors.ci`
- `test/plugin/bmp-sender-updates.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files, tests |
| 3. Implement (TDD) | Phases 1-4 below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Checklist |
| 6. Fix issues | - |
| 9. Deliverables review | Checklist |
| 10. Security review | Checklist |
| 12. Present summary | Executive Summary |

### Phase 1 -- Shared Wire Format

Header and message encode/decode. No network, no plugin registration. Pure data transformation.

1. **Common Header** encode/decode (6 bytes: version, length, type)
   - Tests: `TestBMPCommonHeaderDecode`, `TestBMPCommonHeaderEncode`, `TestBMPCommonHeaderRoundTrip`
   - Files: `header.go`, `header_test.go`

2. **Per-Peer Header** encode/decode (42 bytes: peer type, flags, distinguisher, address, AS, BGP ID, timestamps)
   - Tests: `TestBMPPeerHeaderDecode`, `TestBMPPeerHeaderEncode`, `TestBMPPeerHeaderFlags`, `TestBMPPeerHeaderIPv4Mapped`
   - Files: `header.go`, `header_test.go`

3. **TLV** encode/decode (used by Initiation, Termination, Statistics, Route Mirroring)
   - Tests: `TestBMPTLVDecode`, `TestBMPTLVEncode`
   - Files: `tlv.go`, `tlv_test.go`

4. **Message types** encode/decode for all 7 types
   - Route Monitoring (0): Common Header + Per-Peer Header + inner BGP UPDATE bytes
   - Statistics Report (1): Common Header + Per-Peer Header + stat count + stat TLVs
   - Peer Down (2): Common Header + Per-Peer Header + reason byte + optional NOTIFICATION/FSM-code
   - Peer Up (3): Common Header + Per-Peer Header + local addr + ports + sent OPEN + received OPEN + optional TLVs
   - Initiation (4): Common Header + information TLVs (no per-peer header)
   - Termination (5): Common Header + information TLVs (no per-peer header)
   - Route Mirroring (6): Common Header + Per-Peer Header + mirroring TLVs
   - Tests: `TestBMPInitiation{Decode,Encode}`, `TestBMPTermination{Decode}`, `TestBMPPeerUp{Decode,Encode}`, `TestBMPPeerDown{Decode,Encode}`, `TestBMPRouteMonitoring{Decode,Encode}`, `TestBMPStatisticsReport{Decode,Encode}`
   - Files: `msg.go`, `msg_test.go`

5. **Boundary tests** for all numeric fields (version, length, type, peer type, reason codes)

### Phase 2 -- Receiver

TCP listener, session management, inner BGP decode, monitored peer state, query RPCs.

6. **YANG config** - `bmp` container with receiver section: listener (ip + port, `zt:listener` + `ze:listener`), per-session options, peer filter
   - Files: `schema/ze-bmp-conf.yang`, `schema/register.go`, `schema/embed.go`

7. **Plugin registration** - register.go with RunBMPPlugin entry point, ConfigRoots, YANG
   - Tests: `TestBMPListenerStartsFromConfig`
   - Files: `register.go`, `doc.go`

8. **TCP listener** - accept loop, per-session goroutine, read deadlines, session cap
   - Tests: `TestBMPSessionAccepts`, `TestBMPMalformedHeaderDrops`
   - Files: `listener.go`, `session.go`, `listener_test.go`, `session_test.go`

9. **Route Monitoring decode** - strip BMP framing, hand inner bytes to existing BGP message decoder
   - Tests: `TestBMPRouteMonitoringInnerBGP`
   - Files: `monitor.go`, `monitor_test.go`

10. **Monitored peer state** - peer map keyed by (router, peer address, peer distinguisher), Adj-RIB-In snapshots
    - Files: `state.go`

11. **Query RPCs** - `bmp sessions`, `bmp peers`, `bmp rib <peer>`
    - Files: plugin RPC registration in `register.go`

12. **Functional tests** - synthetic BMP client sends Initiation + Peer Up + Route Monitoring + Peer Down
    - Tests: `test/plugin/bmp-sessions.ci`, `test/plugin/bmp-peers.ci`, `test/plugin/bmp-rib.ci`

### Phase 3 -- Sender

Outbound TCP, event subscription, BMP message generation, statistics, dedup.

13. **YANG config** - sender section under `bmp`: collector list (name + ip + port), statistics-timeout, route-monitoring-policy (pre-policy, all)
    - Files: `schema/ze-bmp-conf.yang` (extend Phase 2 schema)

14. **Outbound TCP client** - connect to collector, exponential backoff reconnection (30s-720s), shutdown via channel
    - Tests: `TestBMPSenderConnects`, `TestBMPSenderReconnect`
    - Files: `sender.go`, `sender_test.go`

15. **Initiation on connect** - send Initiation message with ze sysName and version
    - Tests: `TestBMPSenderInitiation`

16. **Event subscription** - subscribe to `(bgp, state)` and `(bgp, update)` via OnStructuredEvent
    - Peer reaches Established: build Peer Up from OPEN messages
    - Peer leaves Established: build Peer Down with reason code mapping
    - Tests: `TestBMPSenderPeerUp`, `TestBMPSenderPeerDown`

17. **Route Monitoring generation** - wrap received BGP UPDATE in BMP Route Monitoring frame
    - Tests: `TestBMPSenderRouteMonitoring`

18. **Ribout dedup** - per-NLRI tracking, skip identical paths, track withdrawals
    - Tests: `TestBMPSenderRiboutDedup`
    - Files: `ribout.go`

19. **Statistics Report** - periodic timer, collect per-peer counters, serialize
    - Tests: `TestBMPSenderStatistics`
    - Files: `stats.go`

20. **Termination on shutdown** - send Termination to all collectors before close
    - Tests: `TestBMPSenderTermination`

21. **Config reload** - add/remove collectors without disturbing others
    - Related ACs: AC-32, AC-33

22. **Reconnection state** - on reconnect, re-send Initiation + Peer Up for all established peers + Adj-RIB-In dump

23. **Functional tests** - test collector receives Initiation, Peer Up, Route Monitoring
    - Tests: `test/plugin/bmp-collectors.ci`, `test/plugin/bmp-sender-updates.ci`

### Phase 4 -- Documentation and Verification

24. **RFC summary** - `rfc/short/rfc7854.md` covering all 7 message types, wire format, session lifecycle
25. **User guide** - `docs/guide/bmp.md` covering receiver and sender configuration
26. **Feature docs** - update `docs/features.md`, `docs/comparison.md`
27. **Architecture docs** - update `docs/architecture/core-design.md` with BMP plugin
28. **Full verification** - `make ze-verify`
29. **Complete spec** - audit + learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation file:line |
| Correctness | Route Monitoring round-trips against canonical BGP fixtures |
| Naming | Plugin name `bgp-bmp`, YANG container `bmp`, RPCs `bmp.*` |
| Data flow | Inner BGP bytes go through shared decoder (receiver); no private copy |
| Data flow | Sender events flow through OnStructuredEvent, not custom observer |
| Rule: no-layering | No parallel BGP parser |
| Rule: buffer-first | Session read loop uses a pooled buffer (receiver); sender encodes into pooled buffer |
| Rule: self-documenting | Every BMP handler cites RFC 7854 Section |
| Rule: sibling-audit | All structured event subscribers checked when adding BMP subscription |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Plugin registered | grep `Register` in plugin all-file |
| Listener extension in YANG | grep `ze:listener` in `ze-bmp-conf.yang` |
| Receiver session goroutine handles Peer Up/Down | unit test pass output |
| Route Monitoring uses shared decoder | grep call into `internal/component/bgp/message` |
| Sender connects to collectors | unit test with mock TCP listener |
| Sender streams Peer Up/Down/Route Monitoring | unit test pass output |
| Sender statistics timer fires | unit test pass output |
| Functional tests pass | `test/plugin/bmp-*.ci` pass output |
| Documentation pages exist | `ls docs/guide/bmp.md rfc/short/rfc7854.md` |
| Comparison doc updated | grep `BMP` in `docs/comparison.md` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation (receiver) | Reject version != 3; reject length beyond max; reject truncated frames |
| Resource exhaustion (receiver) | Per-listener session cap; per-session buffer cap; read deadline |
| Backpressure (receiver) | Slow consumer on bus must not block session accept loop |
| Resource exhaustion (sender) | Bounded write buffer; write deadline on collector connection |
| Backpressure (sender) | Event queue depth; drop policy when collector is slow |
| Authentication | TLS option for both receiver and sender connections |
| Error leakage | Errors name the frame field, not memory addresses |
| DoS (receiver) | Malformed frames close the single session, never the listener |
| Reconnection flood (sender) | Backoff prevents tight reconnect loops |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Inner BGP decode fails (receiver) | Mark monitored peer errored, keep BMP session open |
| Listener bind fails (receiver) | Plugin reports config error; config commit rejected |
| Collector connection fails (sender) | Retry with exponential backoff; do not block other collectors |
| Event delivery fails (sender) | Log and continue; do not crash plugin |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Research Notes

### Reference Implementations Studied

**GoBGP** (sender, `~/Code/github.com/osrg/gobgp/`):
- Monolithic `pkg/packet/bmp/bmp.go` (1202 lines) with all message types
- `pkg/server/bmp.go` (470 lines) for client lifecycle
- Watch system: subscribes to WatchPeer, WatchUpdate, WatchPostUpdate, WatchBestPath, watchMessage
- Ribout dedup: per-NLRI map prevents resending identical paths
- Reconnection: exponential backoff 1s to 30s
- Statistics: periodic timer sends stats via ListPeer API
- RFC 9069 minimal: Loc-RIB via peer type 3 + synthetic Peer Up/Down
- Pattern: make([]byte, N) allocations throughout (not buffer-first)

**bio-routing** (receiver, `~/Code/github.com/bio-routing/bio-rd/`):
- Clean two-layer split: `protocols/bmp/packet/` (wire) + `protocols/bgp/server/bmp_*.go` (session)
- One file per message type in packet layer
- BMPReceiver: TCP listener, manages Router instances per remote endpoint
- Router: per-router state with serve loop, message dispatch, neighbor manager
- Neighbor: wraps a BGP FSM created in Established state from captured OPEN messages
- recvBMPMsg: reads 6-byte header, then remaining bytes; 4096 default buffer
- Policy filtering: IgnorePrePolicy, IgnorePostPolicy, IgnorePeerASNs config options
- Metrics: Prometheus counters per message type

### Wire Format Reference

**Common Header (6 bytes):**

| Offset | Size | Field | Notes |
|--------|------|-------|-------|
| 0 | 1 | Version | Must be 3 |
| 1 | 4 | Length | Total message length including this header |
| 5 | 1 | Type | 0=Route Monitoring, 1=Stats, 2=Peer Down, 3=Peer Up, 4=Initiation, 5=Termination, 6=Route Mirroring |

**Per-Peer Header (42 bytes, present on types 0-3 and 6):**

| Offset | Size | Field | Notes |
|--------|------|-------|-------|
| 0 | 1 | Peer Type | 0=Global, 1=L3VPN, 2=Local, 3=Loc-RIB |
| 1 | 1 | Peer Flags | bit 7=V(IPv6), bit 6=L(post-policy), bit 5=A(2-byte-AS), bit 4=O(Adj-RIB-Out) |
| 2 | 8 | Peer Distinguisher | RD for VRF peers |
| 10 | 16 | Peer Address | Always 16 bytes; IPv4 as ::ffff:x.x.x.x |
| 26 | 4 | Peer AS | Always 4-byte |
| 30 | 4 | Peer BGP ID | Router ID |
| 34 | 4 | Timestamp (sec) | Epoch seconds |
| 38 | 4 | Timestamp (usec) | Microseconds |

**TLV Format (4-byte overhead):**

| Offset | Size | Field |
|--------|------|-------|
| 0 | 2 | Type |
| 2 | 2 | Length |
| 4 | variable | Value |

**Peer Down Reason Codes:**

| Code | Meaning | Data Following |
|------|---------|----------------|
| 1 | Local system closed, NOTIFICATION sent | BGP NOTIFICATION PDU |
| 2 | Local system closed, no NOTIFICATION | 2-byte FSM event code |
| 3 | Remote system closed, NOTIFICATION received | BGP NOTIFICATION PDU |
| 4 | Remote system closed, no data | None |
| 5 | Peer de-configured | None |

**Statistics Types (RFC 7854 Section 4.8):**

| Type | Name | Size |
|------|------|------|
| 0 | Prefixes rejected by inbound policy | 8 bytes (uint64) |
| 1 | Known duplicate prefix advertisements | 8 bytes |
| 2 | Known duplicate withdraws | 8 bytes |
| 3 | Updates invalidated by CLUSTER_LIST loop | 8 bytes |
| 4 | Updates invalidated by AS_PATH loop | 8 bytes |
| 5 | Updates invalidated by ORIGINATOR_ID | 8 bytes |
| 6 | Updates invalidated by AS_CONFED loop | 8 bytes |
| 7 | Routes in Adj-RIBs-In | 8 bytes |
| 8 | Routes in Loc-RIB | 8 bytes |
| 9 | Routes in per-AFI/SAFI Adj-RIB-In | 11 bytes (2 AFI + 1 SAFI + 8 value) |
| 10 | Routes in per-AFI/SAFI Loc-RIB | 11 bytes |
| 11 | Updates subjected to treat-as-withdraw | 8 bytes |
| 12 | Prefixes subjected to treat-as-withdraw | 8 bytes |
| 13 | Duplicate update messages received | 8 bytes |

**IANA port:** 11019

**Initiation TLV Types:**

| Type | Name |
|------|------|
| 0 | String (free-form UTF-8) |
| 1 | sysDescr (MIB-II ASCII) |
| 2 | sysName (MIB-II ASCII) |

**Termination TLV Types:**

| Type | Name |
|------|------|
| 0 | String (free-form UTF-8) |
| 1 | Reason (uint16: 0=admin-down, 1=unspecified, 2=out-of-resources, 3=redundant, 4=perm-admin-down) |

## Design Insights

### Sender Integration

The sender needs data from the reactor that is currently available via two mechanisms:

1. **PeerLifecycleObserver** (synchronous, reactor-side) - OnPeerEstablished / OnPeerClosed.
   Direct access to Peer struct including Session.localOpen / Session.peerOpen.

2. **OnStructuredEvent** (asynchronous, plugin-side) - structured events with peer metadata
   and raw wire UPDATE bytes. Preferred for BMP because it follows the plugin pattern.

The sender should use OnStructuredEvent (mechanism 2) to stay within the plugin model. If
the structured event does not currently carry the OPEN messages needed for Peer Up, the
event schema may need extension -- but this should be verified during implementation by
reading the actual StructuredEvent fields.

### Receiver vs Sender Asymmetry

The receiver is architecturally simple: standalone listener, parse external data, own
state. Low coupling to the rest of ze.

The sender is architecturally coupled: it must observe the reactor's peer lifecycle and
route updates, serialize them as BMP, and manage outbound connections. It follows the
RPKI plugin pattern (outbound TCP, config-driven sessions, reconnect with backoff).

Both directions share the wire format code (header.go, tlv.go, msg.go).

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## RFC Documentation

RFC 7854 Section references required on: Common Header decoder (Section 4.1), Per-Peer
Header decoder (Section 4.2), Initiation (Section 4.3), Termination (Section 4.5),
Peer Up (Section 4.10), Peer Down (Section 4.9), Route Monitoring (Section 4.6),
Statistics Report (Section 4.8), Route Mirroring (Section 4.7).

## Implementation Summary

### What Was Implemented
- (fill during /implement)

### Bugs Found/Fixed
- (fill during /implement)

### Documentation Updates
- (fill during /implement)

### Deviations from Plan
- (fill during /implement)

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
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] AC-1..AC-40 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Plugin integrated end-to-end (both receiver and sender)
- [ ] Architecture docs updated
- [ ] RFC 7854 short summary added

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

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-bgp-4-bmp.md`
- [ ] Summary included in commit
