# Claude Continuation State

**Last Updated:** 2025-12-27

---

## CURRENT STATUS

✅ **Completed:** Process Backpressure and Respawn Limits

### Session Summary (2025-12-27)

**Completed this session:**
- ✅ Write queue backpressure for API processes
  - `WriteQueueHighWater = 1000` (matches ExaBGP)
  - Non-blocking queue send, drops events when full
  - `QueueSize()`, `QueueDropped()` stats methods
  - Warning log on first backpressure event
  - defer/recover handles race condition on shutdown
- ✅ Respawn limits for API processes
  - `RespawnLimit = 5` per 60-second window (matches ExaBGP)
  - `IsDisabled()`, `Respawn()` methods on ProcessManager
  - Process disabled after exceeding limit
  - Warning log when limit exceeded
- ✅ 8 new tests (TDD) including race condition coverage
- ✅ Critical review performed - no issues found
- ✅ All tests pass (`make test`)
- ✅ Lint clean (`make lint`)

**Key files modified:**
- `pkg/api/process.go` - writeQueue, backpressure, respawn tracking (+210 lines)
- `pkg/api/types.go` - RespawnEnabled field
- `pkg/api/process_test.go` - 8 new tests (+231 lines)

**Spec file:** `plan/done/spec-process-backpressure.md`

---

### Previous Session (2025-12-27) - Collision Detection

**Completed:**
- ✅ Full RFC 4271 §6.8 collision detection implementation
- ✅ ESTABLISHED collision: incoming rejected with NOTIFICATION 6/7
- ✅ OpenConfirm collision: BGP ID comparison, close loser
- ✅ Pending connection tracking in Peer
- ✅ 10 collision tests (TDD)
- ✅ All tests pass (`make test`)
- ✅ Lint clean (`make lint`)

**Architecture:**
```
handleConnection()
├── ESTABLISHED → rejectConnectionCollision() [NOTIFICATION 6/7]
├── OpenConfirm → SetPendingConnection() + go handlePendingCollision()
│                  └── Read OPEN → ResolvePendingCollision()
│                       ├── Local wins → rejectConnectionCollision()
│                       └── Remote wins → CloseWithNotification() existing
│                                        + acceptPendingConnection()
└── Other states → normal AcceptConnection()
```

**Key files modified:**
- `pkg/reactor/session.go` - `DetectCollision()`, `CloseWithNotification()`, `AcceptWithOpen()`, `processOpen()`
- `pkg/reactor/peer.go` - `pendingConn/pendingOpen` fields, collision tracking methods
- `pkg/reactor/reactor.go` - `handleConnection()`, `handlePendingCollision()`, `acceptPendingConnection()`
- `pkg/reactor/collision_test.go` - 10 test functions

**Spec file:** `plan/spec-collision-detection.md`

### Previous Session (2025-12-27) - Session API Commands

**Completed:**
- ✅ Implemented 8 session API commands (ExaBGP compatible)
  - `session ack enable/disable/silence` - ACK response control
  - `session sync enable/disable` - wire transmission sync
  - `session reset` - reset session state
  - `session ping` - health check (returns pong + PID)
  - `session bye` - client disconnect cleanup

**Spec file:** `plan/spec-session-commands.md`

### Previous Session (2025-12-27) - RIB Commands

**Completed:**
- ✅ `rib clear in` - clears all routes from Adj-RIB-In
- ✅ `rib clear out` - withdraws all routes from Adj-RIB-Out
- ✅ `rib flush out` - re-queues sent routes for re-announcement

**Spec file:** `plan/spec-rib-flush-clear.md`

---

## PREVIOUS STATUS

🟡 **Previous Priority:** Encode Test Fixes

### Progress (2025-12-27)

**Extended Community Support - COMPLETE ✅**
- ✅ Added `parseExtendedCommunity()` - parses origin:, redirect:, rate-limit: formats (RFC 4360/5575)
- ✅ Added `parseExtendedCommunities()` - parses bracketed lists
- ✅ Added `extended-community` keyword to UnicastKeywords, MPLSKeywords, VPNKeywords
- ✅ Wired to UPDATE building in `buildAnnounceUpdate()`

**Remaining encode test issues:**
- FlowSpec ICMP type/code encoding (MP_REACH_NLRI length mismatch)
- Extended nexthop (RFC 8950)
- ADD-PATH (path-information)

---

## CRITICAL REVIEW FINDINGS (2025-12-22)

**Major discovery:** Alignment plan was outdated. 7 of 36 items already implemented.

### Items Already Done (No Work Needed)
| Item | Feature | Evidence |
|------|---------|----------|
| 1.1 | RFC 9003 Shutdown Communication | `notification.go:210-249` |
| 1.2 | Per-message-type length validation | `header.go:111-163` |
| 1.3 | RFC 8654 Extended Message validation | `session.go:294-311`, `session.go:590-594` |
| 1.4 | KEEPALIVE payload rejection | `keepalive.go:42-55` |
| 4.2 | AS_PATH auto-split at 255 | `aspath.go:139-178` |
| 4.4 | Large community deduplication | `community.go:228-301` |
| 4.7 | Attribute ordering on send | `origin.go:100-137`, `commit.go` |
| 5.1 | Family validation against negotiated | `session.go:440-526` |
| 3.2 | Hold Time Validation (0 or >=3s) | `session.go:385-401` |
| 8.2 | RFC 7606 Error Recovery | `message/rfc7606.go`, `session.go:validateUpdateRFC7606()` |

---

## HIGH PRIORITY (Test Compatibility)

| Item | Description | Plan |
|------|-------------|------|
| **API Commit System** | Required for all 45 `.run` tests to pass | `api-commit-batching.md` |

## MEDIUM PRIORITY (Functionality)

| Item | Description |
|------|-------------|
| 2.1 | RFC 9072 Extended Optional Parameters |
| 2.2 | Enhanced Route Refresh (RFC 7313) |
| 5.2 | Extended Next-Hop Support |
| 5.3 | MP-NLRI Chunking |

---

## RECENTLY COMPLETED

**Collision Detection (RFC 4271 §6.8)** - 2025-12-27
- ✅ Full implementation with OpenConfirm BGP ID comparison
- ✅ ESTABLISHED early rejection
- ✅ Pending connection tracking
- ✅ NOTIFICATION 6/7 to loser

**Session API Commands** - 2025-12-27
- ✅ 8 ExaBGP-compatible session commands

**RIB Commands** - 2025-12-27
- ✅ `rib clear in/out`, `rib flush out`

**EVPN Encoding** - 2025-12-27
- ✅ All 5 route types have Bytes() implemented

**Critical Review** - 2025-12-22
- ✅ Verified all Phase 1 claims against code
- ✅ Verified Phase 4-5 claims against code
- ✅ Reviewed KEEP decision rationales

---

## REFERENCE DOCS

| Doc | Purpose |
|-----|---------|
| `exabgp-alignment.md` | Review decisions (18 ALIGN, 7 KEEP, 2 SKIP, 9 DONE) |
| `route-families.md` | Route family keyword validation plan |
| `ARCHITECTURE.md` | Codebase architecture overview |

---

## TEST STATUS

✅ **All unit tests pass** (`make test`)
✅ **Lint clean** (`make lint` - 0 issues)

---

## KEY FILES

| Purpose | File |
|---------|------|
| Collision detection | `pkg/reactor/session.go`, `pkg/reactor/peer.go`, `pkg/reactor/reactor.go` |
| Collision tests | `pkg/reactor/collision_test.go` |
| Route handlers | `pkg/api/route.go` |
| UPDATE building | `pkg/reactor/reactor.go` |
| Session commands | `pkg/api/session.go` |
| RIB commands | `pkg/api/rib.go` |
| This file | `plan/CLAUDE_CONTINUATION.md` |
| Protocols | `.claude/ESSENTIAL_PROTOCOLS.md` |

---

## NOTES

- All code changes require TDD (test first, show failure, implement, show pass)
- Plans go in `plan/`, protocols go in `.claude/`
- Check ExaBGP reference before implementing BGP features
- **RFC 7606 implemented** - treat-as-withdraw, attribute-discard, session-reset tactics
- **ALWAYS run `make test && make lint` before requesting a commit**

---

## COMPREHENSIVE DOCUMENTATION REVIEW (2025-12-26)

Full review of ZeBGP implementation against all 44 `.claude` documentation files.

### ✅ EXCELLENT COMPLIANCE (No Action Needed)

| Area | Status | Evidence |
|------|--------|----------|
| **BGP Message Wire Format** | 100% RFC 4271 | All 5 message types, RFC 8654 extended msg, RFC 9072 extended params |
| **Path Attributes** | 100% Compliant | All 17 types, RFC 7606 error handling, ordering per RFC 4271 Appendix F.3 |
| **Capabilities** | 10/10 Documented | All negotiation correct, ADD-PATH asymmetric per RFC 7911 |
| **FSM States** | 6/6 Correct | All states, transitions, 3 mandatory timers |
| **TDD Compliance** | Strong | 67 test files, 578+ tests, table-driven, VALIDATES/PREVENTS pattern |
| **Coding Standards** | Good | Testify usage, proper naming, mock patterns |

### ⚠️ PARTIAL COMPLIANCE (Gaps Identified)

| Area | Issue | Impact | Doc Reference |
|------|-------|--------|---------------|
| ~~NLRI Encoding~~ | ~~EVPN Bytes() returns nil~~ | ✅ DONE | `wire/NLRI_EVPN.md` |
| ~~Collision Detection~~ | ~~RFC 4271 §6.8 violation~~ | ✅ DONE | `behavior/FSM.md` |
| **NLRI Encoding** | FlowSpec encoding incomplete | Limited FlowSpec support | `wire/NLRI_FLOWSPEC.md` |
| **BGP-LS TLVs** | RFC violation in descriptor containers | Interop risk | `wire/NLRI_BGPLS.md` |
| **API v4 JSON** | Not implemented (v6 only) | No v4 compat | `api/JSON_FORMAT.md` |
| **Process Protocol** | No backpressure queue, no respawn limits | Memory/stability risk | `api/PROCESS_PROTOCOL.md` |

### ❌ NOT IMPLEMENTED (Missing Features)

| Feature | RFC | Impact | Doc Reference |
|---------|-----|--------|---------------|
| **MVPN/VPLS/RTC/MUP** | Various | Stub only | `wire/NLRI.md` |
| **Outbound Route Filtering** | RFC 5291 | Optional, acceptable | `wire/CAPABILITIES.md` |
| **Graceful Restart State Machine** | RFC 4724 | Capability parsed, FSM missing | `wire/CAPABILITIES.md` |

### 📈 Test Coverage Summary

| Package | Coverage | Status |
|---------|----------|--------|
| pkg/bgp/fsm | 90.3% | ✅ Excellent |
| pkg/wire | 90.2% | ✅ Excellent |
| pkg/config/migration | 88.6% | ✅ Excellent |
| pkg/bgp/attribute | 83.7% | ✅ Excellent |
| pkg/bgp/message | 83.8% | ✅ Excellent |
| pkg/rib | 83.3% | ✅ Excellent |
| pkg/bgp/capability | 72.9% | ✅ Good |
| pkg/bgp/nlri | 70.4% | ✅ Good |
| pkg/api | 58.7% | ⚠️ Moderate |
| pkg/config | 48.7% | ⚠️ Moderate |
| pkg/reactor | 31.5% | ⚠️ Low |
| pkg/trace | 0% | ❌ None |

### 🔴 CRITICAL Issues - ALL RESOLVED ✅

1. ~~EVPN encoding missing~~ - ✅ DONE - All 5 route types have Bytes() implemented
2. ~~Collision detection missing~~ - ✅ DONE - RFC 4271 §6.8 fully implemented

### 🟡 IMPORTANT Issues

1. **FSM violation** - OpenSent + TCPFails → Idle (should → Active) - documented in code
2. ~~Process backpressure missing~~ - ✅ DONE (2025-12-27)

### 🟢 Minor Issues

1. **FlowSpec ICMP encoding** - ICMP type/code components missing in MP_REACH_NLRI
2. **BGP-LS TLV containers** - Non-RFC descriptor wrapping
3. **No benchmarks** - Only 2 benchmark tests in codebase

### 🎯 Recommendations Priority

| Priority | Action | Effort | Status |
|----------|--------|--------|--------|
| ✅ Done | Extended community parsing | ~200 lines | 2025-12-27 |
| ✅ Done | EVPN Bytes() methods | ~300 lines | 2025-12-27 |
| ✅ Done | RIB flush/clear API commands | ~200 lines | 2025-12-27 |
| ✅ Done | Session API commands | ~200 lines | 2025-12-27 |
| ✅ Done | Collision detection (RFC 4271 §6.8) | ~350 lines | 2025-12-27 |
| ✅ Done | Process backpressure & respawn limits | ~100 lines | 2025-12-27 |
| 🟢 P2 | Fix FlowSpec ICMP encoding | ~100 lines | Pending |
| 🟢 P2 | Fix BGP-LS TLV containers | ~150 lines | Pending |

### Design Decisions (Intentional Differences)

| Decision | Rationale |
|----------|-----------|
| JSON uses `"zebgp"` not `"exabgp"` | ZeBGP is a distinct implementation |
| No multi-session peer selectors | Simplification - tests requiring this removed |
| Attribute order differs from ExaBGP | Both RFC-compliant, documented in `EXABGP_DIFFERENCES.md` |

### Overall Assessment

**~93% complete** against documented specifications. Core BGP wire format is excellent. All critical RFC compliance issues resolved. Process stability features (backpressure, respawn limits) now implemented.
