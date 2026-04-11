# Spec: BMP Receiver (RFC 7854)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc7854.md` - BMP base spec (create if missing)
4. `internal/component/bgp/plugins/` - where new BGP plugins live
5. `internal/component/bgp/message/` - shared BGP message decoder
6. `docs/features.md` and `docs/comparison.md` - "No BMP" entries to update

## Task

BGP Monitoring Protocol (RFC 7854) is the de-facto standard for ingesting live
peer state and Adj-RIB-In / Adj-RIB-Out views from a router for observability
and analysis. Operators reasonably expect a modern BGP daemon or route server
to accept BMP feeds from their fleet.

Ze does not currently implement BMP. This spec covers a **BMP receiver**
(not a sender): ze accepts TCP connections from remote routers that speak BMP
v3, parses the wrapped BGP messages with the existing message decoder, and
materializes the monitored peer state into a queryable view.

Out of scope for this spec:
- BMP sender (a later spec could add it; requires separate design).
- Storing the full history of all monitored routes (this is a receiver, not a
  time-series database). Integration with an external store is optional and
  plugin-driven.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - plugin model
- [ ] `.claude/patterns/plugin.md` - how to register a new plugin
- [ ] `.claude/patterns/config-option.md` - YANG container vs list + listener extension
- [ ] `.claude/rules/config-design.md` - listener structure
  → Constraint: every listener uses `zt:listener` + `ze:listener` extension

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7854.md` - BMP v3 base spec
- [ ] `rfc/short/rfc8671.md` - Adj-RIB-Out addition (can be a follow-up)
- [ ] `rfc/short/rfc9069.md` - Local RIB monitoring (follow-up)

**Key insights:**
- BMP wraps BGP messages in a framing header. The inner BGP messages can be
  decoded by the existing wire decoder once the outer frame is stripped.
- BMP uses TCP; every session is unidirectional (router -> receiver).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/` - existing sub-plugin shape
- [ ] `internal/component/bgp/message/parse.go` - message-level decoder
- [ ] `internal/component/bgp/wire/` - WireUpdate lazy access
- [ ] `internal/yang/modules/ze-bgp-conf.yang` - where new plugin config lives

**Behavior to preserve:**
- Existing BGP peer sessions, reactor, RIB operation unchanged.
- The existing wire parser remains the canonical way to decode BGP messages.

**Behavior to change:**
- Add a new listener (TCP, configurable ip + port) that accepts BMP sessions.
- Add a new plugin that owns the listener, parses BMP framing, reuses the BGP
  decoder for inner BGP messages, and publishes monitored peer state on the
  plugin bus.
- Add a queryable view: CLI + web + RPC to inspect monitored peers and their
  Adj-RIB-In snapshots.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- TCP connection on configured BMP listener (default port TBD, see RFC).
- First bytes: BMP Common Header (version byte == 3, message type byte, length).

### Transformation Path
1. TCP accept - spawn per-session goroutine
2. Read BMP Common Header - dispatch on message type
3. For Route Monitoring: strip framing, hand inner BGP bytes to existing
   `internal/component/bgp/message` decoder
4. For Peer Up / Peer Down / Initiation / Termination: decode framing fields,
   update monitored peer map
5. Publish events to plugin bus (new namespace `bmp`)
6. Optional: persist snapshots in an adj-rib-in-style store (plugin-owned,
   not the main ze RIB)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| TCP listener -> plugin | new listener under `bmp` plugin | [ ] |
| Inner BGP bytes -> decoder | reuse `internal/component/bgp/message` | [ ] |
| Plugin -> bus | events in `bmp/*` namespace | [ ] |
| Plugin -> CLI/web | new RPCs via plugin registration | [ ] |

### Integration Points
- Plugin registration: `internal/component/bgp/plugins/bmp/register.go`
- YANG: new container `ze-bgp-conf:bmp` with listener + per-session options
- CLI: `ze bmp sessions`, `ze bmp peers`, `ze bmp rib <peer>`
- Web UI: read-only monitoring pane

### Architectural Verification
- [ ] No bypass: BMP inner BGP parsing goes through existing decoder
- [ ] No coupling: BMP plugin does not write into the main RIB
- [ ] No duplication: BMP reuses the wire decoder
- [ ] Zero-copy preserved where applicable (inner bytes remain pooled)

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BMP config container loaded | → | plugin registers listener | `TestBMPListenerStartsFromConfig` |
| Remote router connects | → | session goroutine parses headers | `TestBMPSessionAccepts` |
| Route Monitoring message arrives | → | inner BGP decoded via shared decoder | `TestBMPRouteMonitoringParsed` |
| CLI `ze bmp sessions` | → | plugin RPC returns session list | `test/plugin/bmp-list-sessions.ci` |
| CLI `ze bmp peers` | → | plugin RPC returns monitored peer state | `test/plugin/bmp-list-peers.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config enables `bmp` with an `ip` + `port` | Plugin binds a TCP listener on that address |
| AC-2 | Port collision with another listener | YANG validation rejects at commit time (listener extension) |
| AC-3 | Remote router opens a TCP session | Plugin accepts, reads BMP Common Header, validates version==3 |
| AC-4 | Initiation message | Peer identification fields captured and queryable |
| AC-5 | Peer Up Notification | Monitored peer appears in `ze bmp peers` |
| AC-6 | Route Monitoring (BGP UPDATE wrapped) | Inner UPDATE is decoded via the existing BGP decoder; prefixes are visible via `ze bmp rib <peer>` |
| AC-7 | Peer Down Notification | Monitored peer is marked down and its RIB snapshot is dropped |
| AC-8 | Termination | Session is closed cleanly |
| AC-9 | Malformed BMP Common Header | Session is closed; error is logged; other sessions unaffected |
| AC-10 | Malformed inner BGP message | Error path from existing decoder is surfaced; monitored peer marked errored; session stays up |
| AC-11 | Reload of config with BMP listener disabled | Listener and all sessions are stopped, plugin emits a shutdown event |
| AC-12 | Fuzz corpus from the existing BGP fuzzer replayed as BMP inner messages | No panic, no deadlock |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBMPHeaderDecode` | `internal/component/bgp/plugins/bmp/header_test.go` | Common Header parser | |
| `TestBMPPeerHeaderDecode` | `internal/component/bgp/plugins/bmp/peer_test.go` | Per-Peer Header parser | |
| `TestBMPRouteMonitoring` | `internal/component/bgp/plugins/bmp/monitor_test.go` | Route Monitoring wraps existing BGP decoder | |
| `TestBMPListenerStartsFromConfig` | `internal/component/bgp/plugins/bmp/listener_test.go` | Config triggers listener | |
| `TestBMPSessionAccepts` | `internal/component/bgp/plugins/bmp/session_test.go` | Accept + initial handshake | |
| `TestBMPMalformedHeaderDrops` | `internal/component/bgp/plugins/bmp/session_test.go` | Bad framing closes session without panic | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BMP version | 3 only | 3 | 2 | 4 |
| Message length | RFC-defined min..max | max | min-1 | max+1 |
| TCP listen port | 1..65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-bmp-sessions` | `test/plugin/bmp-sessions.ci` | Operator configures BMP listener and a synthetic BMP client connects | |
| `test-bmp-peers` | `test/plugin/bmp-peers.ci` | Monitored peer Up/Down visible in `ze bmp peers` | |
| `test-bmp-rib` | `test/plugin/bmp-rib.ci` | Monitored routes visible in `ze bmp rib` | |

### Future (if deferring any tests)
- Adj-RIB-Out monitoring (RFC 8671) - follow-up spec.
- Local RIB monitoring (RFC 9069) - follow-up spec.

## Files to Modify
- `internal/yang/modules/ze-bgp-conf.yang` - add `bmp` container with listener
- `internal/component/bgp/plugins/all.go` (or equivalent registrar) - import new plugin
- `docs/features.md` - flip "No BMP" to "BMP receiver (RFC 7854)"
- `docs/comparison.md` - update BMP row
- `docs/architecture/core-design.md` - document new plugin

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
- `internal/component/bgp/plugins/bmp/listener.go` - TCP accept loop
- `internal/component/bgp/plugins/bmp/session.go` - per-connection loop
- `internal/component/bgp/plugins/bmp/header.go` - Common + Per-Peer headers
- `internal/component/bgp/plugins/bmp/monitor.go` - Route Monitoring (inner BGP)
- `internal/component/bgp/plugins/bmp/state.go` - monitored peer map + snapshots
- `internal/component/bgp/plugins/bmp/*_test.go` - unit tests
- `rfc/short/rfc7854.md` - RFC short summary
- `docs/guide/bmp.md` - user guide
- `test/plugin/bmp-sessions.ci`
- `test/plugin/bmp-peers.ci`
- `test/plugin/bmp-rib.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files, tests |
| 3. Implement (TDD) | Phases |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Checklist |
| 6. Fix issues | - |
| 9. Deliverables review | Checklist |
| 10. Security review | Checklist |
| 12. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: Framing** - header decoders + unit tests. No network yet.
2. **Phase: Inner BGP reuse** - Route Monitoring path hands inner bytes to the
   existing BGP decoder. Tests assert prefix/attribute equivalence.
3. **Phase: YANG config** - `bmp` container with listener, per-session options,
   peer filter. Listener extension prevents port collisions.
4. **Phase: Plugin wiring** - registration, listener accept loop, session
   lifecycle, bus events. Unit tests for accept + shutdown.
5. **Phase: Query RPCs** - `bmp sessions`, `bmp peers`, `bmp rib <peer>`.
6. **Phase: Functional tests** - synthetic BMP client in `test/` fixtures.
7. **Phase: Docs + RFC short** - write user guide, RFC anchor, update features
   and comparison.
8. **Full verification** - `make ze-verify`.
9. **Complete spec** - audit + learned summary.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation file:line |
| Correctness | Route Monitoring round-trips against canonical BGP fixtures |
| Naming | Plugin name `bmp`, YANG container `bmp`, RPCs `bmp.*` |
| Data flow | Inner BGP bytes go through shared decoder; no private copy |
| Rule: no-layering | No parallel BGP parser |
| Rule: buffer-first | Session read loop uses a pooled buffer, not `append` |
| Rule: self-documenting | Every BMP handler cites RFC 7854 Section |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Plugin registered | grep `Register` in plugin all-file |
| Listener extension in YANG | grep `ze:listener` in `ze-bgp-conf.yang` |
| Session goroutine handles Peer Up/Down | unit test pass output |
| Route Monitoring uses shared decoder | grep call into `internal/component/bgp/message` |
| Functional tests pass | `test/plugin/bmp-*.ci` pass output |
| Documentation pages exist | `ls docs/guide/bmp.md` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Reject version != 3; reject length beyond max; reject truncated frames |
| Resource exhaustion | Per-listener session cap; per-session buffer cap; read deadline |
| Backpressure | Slow consumer on bus must not block session accept loop |
| Authentication | TLS / MD5 / allowlist - which does ze use? Decide during design |
| Error leakage | Errors name the frame field, not memory addresses |
| DoS | Malformed frames close the single session, never the listener |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Inner BGP decode fails | Mark monitored peer errored, keep BMP session open |
| Listener bind fails | Plugin reports config error; config commit rejected |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights

## RFC Documentation

RFC 7854 Section references required on: Common Header decoder, Per-Peer
Header decoder, Initiation / Termination TLVs, Peer Up / Peer Down encoding,
Route Monitoring framing.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Plugin integrated end-to-end
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
- [ ] Learned summary written to `plan/learned/NNN-bmp-receiver.md`
- [ ] Summary included in commit
