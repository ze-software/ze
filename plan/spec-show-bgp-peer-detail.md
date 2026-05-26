# Spec: show-bgp-peer-detail

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-05-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/commands.md` - command handler pattern
4. `internal/component/plugin/types_bgp.go` - PeerInfo struct (30 fields currently)
5. `internal/component/bgp/reactor/peer_stats.go` - atomic counters (6 currently)
6. `internal/component/bgp/reactor/reactor_api.go` - Peers() builder (lines 89-137)
7. `internal/component/bgp/plugins/cmd/peer/peer.go` - HandleBgpPeerDetail (lines 134-208)

## Task

Enrich `show bgp peer <x>` output to match production NOS implementations (JunOS, IOS-XR, EOS, FRR). Ze currently outputs ~20 fields; industry standard is 50-60. The gaps are: full message counters (opens, notifications, route-refresh, total), negotiated timers, last notification details, last read/write timestamps, connection stability metrics, inline capabilities, policy names, peer type, transport details, and graceful restart state.

All changes are additive. No new commands, no new RPCs. Existing output fields and the separate `capabilities`/`statistics` subcommands remain unchanged.

Prefix counters (received/accepted/active/sent) are out of scope: they require a RIB plugin bridge that is separate work.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - command handler pattern
  -> Constraint: handlers return `plugin.Response` with `map[string]any` Data
  -> Constraint: kebab-case keys in all JSON output
- [ ] `docs/architecture/core-design.md` - reactor event loop
  -> Constraint: counters must be lock-free atomics on hot path

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - hold time negotiation (Section 4.2), keepalive derivation (Section 10)
  -> Constraint: negotiated hold = min(local, remote), must be 0 or >= 3s
- [ ] `rfc/short/rfc4724.md` - graceful restart mechanism
  -> Constraint: GR restart time and forwarding state per family
- [ ] `rfc/short/rfc8203.md` - shutdown communication
  -> Constraint: last notification should include shutdown message when subcodes 2/4

**Key insights:**
- Session struct already has `onNotifSent`/`onNotifRecv` callbacks: reuse this pattern for opens, refresh, timestamps
- `capability.Negotiated` already stores `HoldTime` (uint16) and `GracefulRestart` (*GracefulRestart): not exposed through `NegotiatedCapabilities`
- `sessionHealth` tracks flaps in a sliding window (`flapTimes`, 5-min, max 4 entries): need a lifetime counter for the API
- PeerInfo already has `ImportFilters`/`ExportFilters` fields: HandleBgpPeerDetail does not output them
- `PeerSettings` has `MD5Key`, `BFD`, `Port` fields available for exposure
- `remoteRouterID` is `atomic.Uint32` on Peer: set from peer OPEN, currently only used for route reflection

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/types_bgp.go` - PeerInfo struct with ~30 fields, PeerStats with 6 counters
  -> Constraint: PeerInfo is the API contract between reactor and command handlers
- [ ] `internal/component/bgp/reactor/peer_stats.go` - 6 atomic counters (updates recv/sent, keepalives recv/sent, EOR recv/sent), plus establishedAt; Incr methods all push to Prometheus `rmetrics`
  -> Constraint: every new Incr method must also push to Prometheus
- [ ] `internal/component/bgp/reactor/peer.go` - Peer struct (lines 144-283): FSM state as atomic.Int32, negotiated caps as atomic.Pointer, `notificationExchanged` as atomic.Bool, `remoteRouterID` as atomic.Uint32
  -> Constraint: all peer state accessed from multiple goroutines must use atomics
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `Peers()` (lines 89-137) builds PeerInfo under `r.mu.RLock`, reads settings + Stats() + negotiated + EstablishedAt
  -> Constraint: new fields populate here, under existing lock
- [ ] `internal/component/bgp/reactor/session.go` - Session struct (line 184) with `onNotifSent`/`onNotifRecv` function callbacks, set by Peer in `runOnce()`
  -> Constraint: session has no direct Peer reference; use callback pattern
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - computes `negotiatedHold` (lines 47-61), stores in `s.negotiated.HoldTime`
  -> Constraint: negotiated hold/keepalive computed here, currently only stored on session.negotiated
- [ ] `internal/component/bgp/reactor/session_read.go` - message dispatch switch at line 227: UPDATE, OPEN, KEEPALIVE, NOTIFICATION, ROUTEREFRESH
  -> Constraint: lastRead timestamp should be set near existing `recentRead.Store(true)` at line 92
- [ ] `internal/component/bgp/reactor/session_handlers.go` - `handleOpen()` at line 38 processes received OPEN
  -> Constraint: open received counter should be called here
- [ ] `internal/component/bgp/reactor/negotiated.go` - `NegotiatedCapabilities` stores families, ExtendedMessage, RouteRefresh, EnhancedRouteRefresh, ASN4
  -> Constraint: does not expose HoldTime or GracefulRestart; needs extension
- [ ] `internal/component/bgp/reactor/session_health.go` - `flapTimes` ring buffer (line 37), bounded at threshold+1 entries, 5-min window
  -> Constraint: sliding window not suitable for lifetime API count
- [ ] `internal/component/bgp/reactor/reactor_metrics.go` - Prometheus counters exist for: notifSent/Recv (with code/subcode), sessionFlaps, stateTransitions, sessionsEstablished, peerMsgRecv/Sent (with type label)
  -> Constraint: Prometheus already tracks data we lack in atomic counters
- [ ] `internal/component/bgp/capability/negotiated.go` - `Negotiated` struct stores HoldTime (uint16), GracefulRestart (*GracefulRestart), Mismatches, composite sub-components (Identity, Encoding, Session)
  -> Constraint: GracefulRestart is a pointer to capability.GracefulRestart struct
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - HandleBgpPeerDetail (lines 134-208) builds flat `map[string]any` with ~20 fields, optional fields gated by zero-value checks
  -> Constraint: existing flat fields must stay for backward compatibility
- [ ] `internal/component/bgp/reactor/peersettings.go` - PeerSettings struct (line 253): has Port (uint16), MD5Key (string), BFD (*BFDSettings), ImportFilters/ExportFilters ([]string)
  -> Constraint: these are config-time values, always available

**Behavior to preserve:**
- Existing JSON field names and types in HandleBgpPeerDetail output
- Existing `show bgp peer <x> capabilities` subcommand behavior
- Existing `show bgp peer <x> statistics` subcommand behavior
- PeerStats.ClearStats() resets per-session counters on teardown
- Prometheus counter integration (every Incr pushes to rmetrics)
- Counter reset semantics: updates/keepalives/EOR are per-session; new lifetime counters (connections, flaps) survive ClearStats

**Behavior to change:**
- HandleBgpPeerDetail output expanded with ~30 additional fields
- Message counters restructured into nested `messages.received`/`messages.sent` (old flat fields kept)
- Capabilities shown inline in detail output (in addition to separate subcommand)
- Policy names shown in detail output

## Data Flow (MANDATORY)

### Entry Point
- CLI command `show bgp peer <x>` dispatches to RPC `ze-show:bgp-peer`
- Handler: `HandleBgpPeerDetail` in `internal/component/bgp/plugins/cmd/peer/peer.go`

### Transformation Path
1. Handler calls `filterPeersBySelector(ctx)` which calls `ctx.Reactor().Peers()`
2. `Peers()` (in `reactor_api.go`) iterates `r.peers` under RLock, reads settings + Stats() + negotiated from each Peer
3. Returns `[]plugin.PeerInfo` value-copy snapshots
4. Handler iterates PeerInfo slice, builds `map[string]any` per peer
5. Returns `plugin.Response` with nested data map

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor -> Plugin server | PeerInfo struct (value copy via Peers()) | [ ] |
| Atomic counters -> PeerInfo | Load in Peers() under RLock | [ ] |
| Session -> Peer | Callbacks set in runOnce() | [ ] |

### Integration Points
- `Peer.Stats()` returns `PeerStats` snapshot: extend with new counter fields
- `Peer.negotiated.Load()` returns `*NegotiatedCapabilities`: extend with HoldTime, GR
- `Peer.EstablishedAt()` returns establishment time: already used for uptime
- `sessionHealth` flap tracking: add lifetime counter + accessor method

### Architectural Verification
- [ ] No bypassed layers (data flows Peer -> PeerInfo -> handler)
- [ ] No unintended coupling (reactor internals stay in reactor, PeerInfo is the API)
- [ ] No duplicated functionality (extends existing counter and snapshot patterns)
- [ ] Zero-copy preserved (atomics are value reads, no allocation on read path)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp peer <x>` CLI | -> | HandleBgpPeerDetail returns new fields | `TestHandleBgpPeerDetailNewFields` |
| Peer receives OPEN | -> | IncrOpensReceived increments counter | `TestPeerStatsOpens` |
| Peer sends NOTIFICATION | -> | Last notification code/subcode stored | `TestPeerStatsLastNotification` |
| Session negotiates hold time | -> | Negotiated hold exposed in PeerInfo | `TestNegotiatedTimersInPeerInfo` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Established iBGP session | Output includes `peer-type` with value `internal` |
| AC-2 | Established eBGP session | Output includes `peer-type` with value `external` |
| AC-3 | Any established session | Output includes `messages` map with `received` and `sent` sub-maps, each containing `opens`, `updates`, `notifications`, `keepalives`, `route-refresh`, `total` as integers |
| AC-4 | Any established session | Output includes `negotiated-hold-time` and `negotiated-keepalive-time` as integer seconds |
| AC-5 | Any established session | Output includes `capabilities` map with `negotiation-complete`, `families`, `asn4`, `extended-message`, `route-refresh`, `enhanced-route-refresh` |
| AC-6 | Peer has import/export filters | Output includes `import-policy` and `export-policy` string arrays |
| AC-7 | Any session with recent activity | Output includes `last-read` and `last-write` as ISO 8601 timestamps |
| AC-8 | Any session | Output includes `connections-established` and `connections-dropped` as lifetime integer counts |
| AC-9 | NOTIFICATION received/sent before session drop | Output includes `last-notification` map with `code`, `subcode`, `direction` (received/sent), `time` |
| AC-10 | Established session | Output includes `local-port` and `remote-port` as integers |
| AC-11 | MD5 configured | Output includes `md5` with value true |
| AC-12 | BFD configured | Output includes `bfd` with value true |
| AC-13 | Session that has flapped | Output includes `flap-count` as integer |
| AC-14 | GR capability negotiated | `capabilities` map includes `graceful-restart` sub-map with `restart-time` |
| AC-15 | Any session | Existing flat fields `updates-received`, `updates-sent`, `keepalives-received`, `keepalives-sent`, `eor-received`, `eor-sent` still present at top level |
| AC-16 | `show bgp peer <x> capabilities` | Subcommand output unchanged |
| AC-17 | `show bgp peer <x> statistics` | Subcommand output unchanged |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerStatsOpens` | `reactor/peer_stats_test.go` | Open message counters increment and appear in Stats() | |
| `TestPeerStatsNotifications` | same | Notification counters increment, last notification details stored | |
| `TestPeerStatsRefresh` | same | Route-refresh counters increment | |
| `TestPeerStatsConnectionCounts` | same | Connections established/dropped lifetime counters | |
| `TestPeerStatsLastReadWrite` | same | Last read/write timestamps stored and retrieved | |
| `TestPeerStatsClearPreservesLifetime` | same | ClearStats resets per-session counters but preserves lifetime counters (connections, flaps, last-notification) | |
| `TestNegotiatedCapabilitiesHoldTime` | `reactor/negotiated_test.go` | HoldTime and GR populated from capability.Negotiated | |
| `TestSessionHealthFlapLifetime` | `reactor/session_health_test.go` | Lifetime flap counter increments on every Established to non-Established transition | |
| `TestPeersPopulatesNewFields` | `reactor/reactor_api_test.go` | Peers() returns PeerInfo with all new fields populated | |
| `TestPeerTypeDerived` | same | PeerType is "internal" when LocalAS == PeerAS, "external" otherwise | |
| `TestHandleBgpPeerDetailNewFields` | `plugins/cmd/peer/peer_test.go` | HandleBgpPeerDetail output contains messages, capabilities, policy blocks | |
| `TestHandleBgpPeerDetailBackwardCompat` | same | Existing flat fields still present unchanged | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Notification code | 0-255 | 255 | N/A (uint8) | N/A (uint8) |
| Notification subcode | 0-255 | 255 | N/A (uint8) | N/A (uint8) |
| TCP port | 0-65535 | 65535 | N/A (uint16) | N/A (uint16) |
| Negotiated hold time | 0, 3-65535 | 65535 | N/A (uint16) | N/A (uint16) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-show-peer-detail` | `test/bgp/show-peer-detail.ci` | Establish peer, run `show bgp peer <x>`, verify messages/capabilities/policy/timestamps present | |

### Interop Tests
Not applicable: output enrichment only, no protocol behavior change.

### Future
- Prefix counters (received/accepted/active/sent): requires RIB plugin bridge, separate spec
- Advertised vs received capabilities (both sides separately): medium effort, not essential for v1

## Files to Modify

- `internal/component/bgp/reactor/peer_stats.go` - add 12 atomic counters, 8 Incr/Touch methods, extend Stats() and ClearStats()
- `internal/component/bgp/reactor/peer.go` - add 4 atomic fields (negotiated hold/keepalive, local/remote port)
- `internal/component/bgp/reactor/peer_run.go` - wire 6 new callbacks in runOnce()
- `internal/component/bgp/reactor/session.go` - add 6 callback function fields
- `internal/component/bgp/reactor/session_handlers.go` - call onOpenRecv in handleOpen()
- `internal/component/bgp/reactor/session_negotiate.go` - call onOpenSent in sendOpen(), invoke negotiated timer callback
- `internal/component/bgp/reactor/session_read.go` - call onRefreshRecv at ROUTEREFRESH case, call onRead near recentRead
- `internal/component/bgp/reactor/session_write.go` - call onWrite in common write path
- `internal/component/bgp/reactor/session_health.go` - add flapLifetime atomic.Uint32, increment in flap path, add FlapCount() accessor
- `internal/component/bgp/reactor/reactor_api.go` - populate ~25 new PeerInfo fields in Peers() loop
- `internal/component/bgp/reactor/reactor_api_forward.go` - call refresh sent counter in sendRouteRefresh()
- `internal/component/bgp/reactor/negotiated.go` - add HoldTime (uint16) and GracefulRestart fields to NegotiatedCapabilities
- `internal/component/plugin/types_bgp.go` - add ~25 new fields to PeerInfo, extend PeerStats with 8 new counter fields
- `internal/component/bgp/plugins/cmd/peer/peer.go` - extend HandleBgpPeerDetail to emit all new fields

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| CLI grammar | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/bgp/show-peer-detail.ci` |
| Doctor check | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/features/cli-commands.md` - document enriched show bgp peer output |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - update peer-detail response fields |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `test/bgp/show-peer-detail.ci` - functional test for enriched peer detail output

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | `make ze-verify` |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** - Stub new PeerInfo fields, stub new counter methods, write failing wiring tests
   - Tests: `TestPeerStatsOpens`, `TestHandleBgpPeerDetailNewFields`
   - Files: `types_bgp.go` (field stubs), `peer_stats.go` (method stubs), `peer.go` (test)
   - Verify: tests compile but fail (counters return zero, output missing keys)

2. **Phase: Atomic counters** - Implement all new atomic counters, Incr/Touch methods, extend Stats() and ClearStats()
   - Tests: `TestPeerStatsOpens`, `TestPeerStatsNotifications`, `TestPeerStatsRefresh`, `TestPeerStatsConnectionCounts`, `TestPeerStatsLastReadWrite`, `TestPeerStatsClearPreservesLifetime`
   - Files: `peer_stats.go`
   - Verify: all counter unit tests pass

3. **Phase: Session callbacks** - Add callback fields to Session, wire in runOnce(), call from handlers and write path
   - Tests: integration through session message processing
   - Files: `session.go` (new callback fields), `session_handlers.go` (open recv), `session_negotiate.go` (open sent + negotiated timer callback), `session_read.go` (refresh recv, lastRead), `session_write.go` (lastWrite), `peer_run.go` (wire callbacks), `reactor_api_forward.go` (refresh sent)
   - Verify: counters increment when messages flow through session

4. **Phase: Negotiated and health extensions** - Expose HoldTime/GR through NegotiatedCapabilities, add lifetime flap counter to sessionHealth
   - Tests: `TestNegotiatedCapabilitiesHoldTime`, `TestSessionHealthFlapLifetime`
   - Files: `negotiated.go`, `session_health.go`
   - Verify: negotiated hold time and flap count accessible

5. **Phase: PeerInfo population** - Extend Peers() to populate all new fields, add peer atomics for negotiated timers and TCP ports
   - Tests: `TestPeersPopulatesNewFields`, `TestPeerTypeDerived`
   - Files: `reactor_api.go`, `peer.go` (new atomics)
   - Verify: PeerInfo snapshots contain all new data

6. **Phase: Detail output** - Extend HandleBgpPeerDetail to emit all new fields with proper structure
   - Tests: `TestHandleBgpPeerDetailNewFields`, `TestHandleBgpPeerDetailBackwardCompat`
   - Files: `plugins/cmd/peer/peer.go`
   - Verify: JSON output contains all expected kebab-case keys with correct nesting

7. **Functional tests** - Create end-to-end test
8. **Full verification** - `make ze-verify`
9. **Complete spec** - Audit, learned summary, closure

### Output Structure

New fields added to the HandleBgpPeerDetail response (in addition to all existing fields):

| Field path | Type | Source | Condition |
|------------|------|--------|-----------|
| `peer-type` | string | derived: LocalAS == PeerAS ? "internal" : "external" | always |
| `local-port` | integer | TCP conn local addr port | session established |
| `remote-port` | integer | TCP conn remote addr port | session established |
| `md5` | boolean | settings.MD5Key != "" | when true |
| `bfd` | boolean | settings.BFD != nil | when true |
| `negotiated-hold-time` | integer (seconds) | NegotiatedCapabilities.HoldTime | session established |
| `negotiated-keepalive-time` | integer (seconds) | derived: holdTime/3 or clamped | session established |
| `messages.received.opens` | integer | PeerStats.OpensReceived | always |
| `messages.received.updates` | integer | PeerStats.UpdatesReceived | always |
| `messages.received.notifications` | integer | PeerStats.NotificationsReceived | always |
| `messages.received.keepalives` | integer | PeerStats.KeepalivesReceived | always |
| `messages.received.route-refresh` | integer | PeerStats.RefreshReceived | always |
| `messages.received.total` | integer | sum of all received | always |
| `messages.sent.opens` | integer | PeerStats.OpensSent | always |
| `messages.sent.updates` | integer | PeerStats.UpdatesSent | always |
| `messages.sent.notifications` | integer | PeerStats.NotificationsSent | always |
| `messages.sent.keepalives` | integer | PeerStats.KeepalivesSent | always |
| `messages.sent.route-refresh` | integer | PeerStats.RefreshSent | always |
| `messages.sent.total` | integer | sum of all sent | always |
| `last-notification.code` | integer | PeerStats.LastNotifCode | when notification occurred |
| `last-notification.subcode` | integer | PeerStats.LastNotifSubcode | same |
| `last-notification.direction` | string | "received" or "sent" | same |
| `last-notification.time` | string (ISO 8601) | PeerStats.LastNotifTime | same |
| `last-read` | string (ISO 8601) | PeerInfo.LastReadTime | when non-zero |
| `last-write` | string (ISO 8601) | PeerInfo.LastWriteTime | when non-zero |
| `connections-established` | integer | PeerStats.ConnectionsEstablished | always |
| `connections-dropped` | integer | PeerStats.ConnectionsDropped | always |
| `flap-count` | integer | sessionHealth.FlapCount() | always |
| `import-policy` | string array | PeerInfo.ImportFilters | when non-empty |
| `export-policy` | string array | PeerInfo.ExportFilters | when non-empty |
| `capabilities.negotiation-complete` | boolean | NegotiatedCapabilities != nil | always |
| `capabilities.families` | string array | NegotiatedCapabilities.Families() | when negotiated |
| `capabilities.asn4` | boolean | NegotiatedCapabilities.ASN4 | when negotiated |
| `capabilities.extended-message` | boolean | NegotiatedCapabilities.ExtendedMessage | when negotiated |
| `capabilities.route-refresh` | boolean | NegotiatedCapabilities.RouteRefresh | when negotiated |
| `capabilities.enhanced-route-refresh` | boolean | NegotiatedCapabilities.EnhancedRouteRefresh | when negotiated |
| `capabilities.add-path` | map string->string | family -> send/receive/both | when negotiated and non-nil |
| `capabilities.graceful-restart.restart-time` | integer (seconds) | GracefulRestart.Time | when GR negotiated |

### Counter Lifetime Semantics

| Counter group | Reset on session teardown | Rationale |
|---------------|--------------------------|-----------|
| Updates recv/sent, Keepalives recv/sent, EOR recv/sent, Opens recv/sent, Refresh recv/sent | Yes (ClearStats) | Per-session metrics, operators expect current session counts |
| Notifications recv/sent | Yes | Per-session counts (same as other message types) |
| Last notification code/subcode/direction/time | No | Survives across sessions: operators need "why did it last drop?" |
| Connections established/dropped | No | Lifetime stability indicator |
| Flap count | No | Lifetime stability indicator |
| Last read/write timestamps | Yes (implicit: session writes new values) | Current session activity |

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-17 has implementation with file:line |
| Correctness | Counter increments happen exactly once per event (no double-counting from callbacks + direct calls) |
| Naming | All JSON keys are kebab-case |
| Data flow | All new data flows through PeerInfo (no direct reactor access from handlers) |
| Thread safety | All new counters use atomics; all reads in Peers() under existing RLock |
| Backward compat | Flat fields still present; capabilities and statistics subcommands unchanged |
| Prometheus parity | Every new Incr method also pushes to rmetrics when non-nil |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| New atomic counters in peer_stats.go | grep `atomic.Uint32` peer_stats.go, count >= 12 new |
| PeerInfo struct extended | documentSymbol on types_bgp.go shows new fields |
| HandleBgpPeerDetail outputs messages block | grep `"messages"` in peer.go |
| HandleBgpPeerDetail outputs capabilities block | grep `"capabilities"` in peer.go |
| HandleBgpPeerDetail outputs policy names | grep `"import-policy"` in peer.go |
| Functional test exists | ls test/bgp/show-peer-detail.ci |
| All existing tests pass | make ze-verify |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | No new user input; all fields derived from internal session state |
| Counter overflow | uint32 wraps at 4B; acceptable for message counts over any realistic session lifetime |
| Timestamp exposure | Last read/write times do not leak sensitive scheduling information |
| Notification data | Last notification code/subcode are standard protocol values, not arbitrary user data |
| Policy names | Filter chain names come from validated config, not raw user input |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong, revisit design; if AC correct, fix implementation |
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

## Design Insights

## RFC Documentation

Add `// RFC 4271 Section 4.2` above negotiated hold time storage.
Add `// RFC 4724` above graceful restart field exposure.
Add `// RFC 8203` above last notification shutdown message handling.

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| show bgp peer output at NOS parity | functional test | test/bgp/show-peer-detail.ci |
| Backward compatibility preserved | unit test | TestHandleBgpPeerDetailBackwardCompat |
| All message types counted | unit test | TestPeerStatsOpens, TestPeerStatsNotifications, TestPeerStatsRefresh |

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

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests: N/A (output enrichment, not protocol change)
- [ ] Goal Validation table filled

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/792-show-bgp-peer-detail.md`
- [ ] Summary included in commit
