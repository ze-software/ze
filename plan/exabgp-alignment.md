# ZeBGP/ExaBGP Alignment Review Plan

**Purpose:** Systematically review each difference and decide: align with ExaBGP, keep ZeBGP approach, or skip.

**Last Review:** 2025-12-22 (Critical review completed - many items already implemented)

---

## Review Status Legend

- **ALIGN** - Change ZeBGP to match ExaBGP behavior
- **KEEP** - Keep ZeBGP's current approach (document why)
- **SKIP** - Not relevant / defer to later
- **DONE** - Already implemented in ZeBGP

---

## Phase 1: Critical Compatibility Issues

### 1.1 RFC 8203/9003 Shutdown Communication
**Current:** ZeBGP implements `ShutdownMessage()` with RFC 9003 parsing
**ExaBGP:** Parses length-prefixed UTF-8 shutdown message
**Status:** ✅ **DONE** - Full implementation in `notification.go:210-249` with UTF-8 validation

### 1.2 Per-Message-Type Length Validation
**Current:** ZeBGP validates OPEN>=29, UPDATE>=23, KEEPALIVE==19, NOTIFICATION>=21, ROUTE_REFRESH>=23
**ExaBGP:** Same validation
**Status:** ✅ **DONE** - `ValidateLength()` in `header.go:111-163`

### 1.3 Extended Message Size Integration
**Current:** Constants defined, `ValidateLengthWithMax()` exists but NOT wired in receive path
**ExaBGP:** Sets msg_size immediately when capability negotiated
**Impact:** May fail to send/receive large UPDATE messages
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP
**Work:** Wire `ValidateLengthWithMax(extendedMessage)` in `session.go` receive path

### 1.4 KEEPALIVE Payload Validation
**Current:** ZeBGP rejects KEEPALIVE with payload via NOTIFICATION error
**ExaBGP:** Same behavior
**Status:** ✅ **DONE** - `keepalive.go:42-55` returns `NotifyHeaderBadLength`

---

## Phase 2: Capability Differences

### 2.1 RFC 9072 Extended Optional Parameters
**Current:** Not implemented
**ExaBGP:** Supports marker 0xFF 0xFF + 2-byte length for large capability sets
**Impact:** Cannot handle peers with many capabilities (>255 bytes total)
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 2.2 Enhanced Route Refresh (RFC 7313)
**Current:** Not implemented (only basic RFC 2918)
**ExaBGP:** Full support with Begin-of-RR/End-of-RR markers
**Impact:** Cannot do ORF-based route refresh
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 2.3 Multi-Session BGP (Code 68)
**Current:** Not implemented
**ExaBGP:** Full support including Cisco variant (0x83)
**Decision:** [ ] ALIGN / [ ] KEEP / [x] SKIP

### 2.4 Dynamic Capability (Code 67)
**Current:** Not implemented
**ExaBGP:** Code defined (not fully implemented either)
**Decision:** [ ] ALIGN / [ ] KEEP / [x] SKIP *(defer)*

### 2.5 Capability Conflict Detection
**Current:** Silent intersection (no error reporting)
**ExaBGP:** Active detection with mismatch list and RFC error codes
**Impact:** No visibility into negotiation mismatches
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 3: Timer Defaults

### 3.1 Default Hold Time
**Current:** ZeBGP uses 90 seconds
**ExaBGP:** Uses 180 seconds
**RFC 4271 Section 10:** "suggested default value for the HoldTime is 90 seconds"
**Decision:** [ ] ALIGN (180s) / [x] KEEP (90s) / [ ] SKIP
**Rationale:** RFC explicitly suggests 90s

### 3.2 Hold Time Validation
**Current:** ZeBGP accepts 0-65535
**ExaBGP:** Validates 0 or >= 3 seconds per RFC 4271
**Impact:** May accept invalid hold times (1-2 seconds)
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 4: Path Attributes

### 4.1 ORIGIN Validation
**Current:** ZeBGP rejects invalid values (>2)
**ExaBGP:** Accepts any value
**RFC 4271 Section 6.3:** "MUST be set to Invalid Origin Attribute" for undefined values
**Decision:** [ ] ALIGN (permissive) / [x] KEEP (strict) / [ ] SKIP
**Rationale:** RFC 4271 REQUIRES rejection - this is mandatory, not optional

### 4.2 AS_PATH Segment Auto-Split
**Current:** ZeBGP auto-splits via `packSegmentWithSplit()` at 255 ASNs
**ExaBGP:** Same behavior
**Status:** ✅ **DONE** - `aspath.go:139-178`

### 4.3 Extended Communities IPv6 (RFC 5701)
**Current:** Not implemented (only 8-byte)
**ExaBGP:** Full 20-byte IPv6 extended community support
**Impact:** Cannot handle IPv6 extended communities
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 4.4 Large Community Deduplication
**Current:** ZeBGP deduplicates on send (`Pack()`) and receive (`ParseLargeCommunities()`)
**ExaBGP:** Same behavior
**RFC 8092 Section 5:** "MUST silently remove redundant values"
**Status:** ✅ **DONE** - `community.go:228-301`

### 4.5 Community Sorting
**Current:** No sorting
**ExaBGP:** Auto-sorts communities
**RFC 1997:** Uses "set" terminology (unordered)
**Decision:** [ ] ALIGN / [x] KEEP / [ ] SKIP
**Rationale:** RFC allows any order, sorting is unnecessary computation

### 4.6 Attribute Caching
**Current:** Pool (`internal/pool/`) and Store (`internal/store/`) for deduplication
**ExaBGP:** Extensive caching (ORIGIN, communities, etc.)
**Decision:** [ ] ALIGN / [x] KEEP / [ ] SKIP
**Rationale:** ZeBGP has equivalent functionality via Store/Pool architecture

### 4.7 Attribute Ordering on Send
**Current:** ZeBGP orders via `PackAttributesOrdered()` and manual ordering in commit path
**ExaBGP:** Orders by type code per RFC 4271 Appendix F.3
**Status:** ✅ **DONE** - `origin.go:100-137`, `update.go:58`, `commit.go:189-249`

---

## Phase 5: MP-NLRI Handling

### 5.1 Family Validation Against Negotiated
**Current:** ZeBGP validates via `validateUpdateFamilies()` with strict/lenient modes
**ExaBGP:** Strict validation against negotiated families
**Status:** ✅ **DONE** - `session.go:440-526`

### 5.2 Extended Next-Hop Support
**Current:** Not implemented
**ExaBGP:** Via negotiated.nexthop flag
**Impact:** Cannot handle IPv6 next-hops for IPv4 prefixes
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 5.3 MP-NLRI Chunking
**Current:** No chunking
**ExaBGP:** Respects attribute size limits, splits across UPDATEs
**Impact:** May fail with large NLRI sets
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 5.4 Route Distinguisher in Next-Hop
**Current:** Not handled
**ExaBGP:** Full VPN RD support in MP_REACH next-hop
**Impact:** VPN next-hop parsing may fail
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 6: NLRI Types

### 6.1 EVPN Type 1 (Ethernet Auto-Discovery)
**Current:** Generic wrapper (EVPNGeneric)
**ExaBGP:** Full parsing
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 6.2 EVPN Type 4 (Ethernet Segment)
**Current:** Generic wrapper (EVPNGeneric)
**ExaBGP:** Full parsing
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 6.3 FlowSpec VPN Variant
**Current:** Not implemented
**ExaBGP:** Full support
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 6.4 VPLS NLRI
**Current:** Not implemented
**ExaBGP:** Full support
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 6.5 RTC (Route Target Constraint)
**Current:** Not implemented
**ExaBGP:** Full support
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 7: FSM & Session

### 7.1 FSM Architecture
**Current:** Event-driven with explicit Event() calls
**ExaBGP:** Procedural with direct state changes
**Decision:** [ ] ALIGN / [x] KEEP / [ ] SKIP
**Rationale:** Event-driven is Go idiomatic, more testable

### 7.2 Timer Implementation
**Current:** Go time.Timer objects with callbacks
**ExaBGP:** Polling-based with timestamps
**Decision:** [ ] ALIGN / [x] KEEP / [ ] SKIP
**Rationale:** Go idiomatic, leverages runtime scheduler

### 7.3 Reconnect Backoff
**Current:** Exponential 5s-60s in Peer loop
**ExaBGP:** Delay class with increase/reset
**Decision:** [ ] ALIGN / [x] KEEP / [ ] SKIP
**Rationale:** Equivalent behavior, Go idiomatic implementation

---

## Phase 8: Error Handling

### 8.1 Error Subcode Coverage
**Current:** 12 subcodes defined
**ExaBGP:** 48+ subcodes with descriptions
**Impact:** Less detailed error messages
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 8.2 RFC 7606 Error Recovery
**Current:** Strict fail (RFC 4271 approach)
**ExaBGP:** Some RFC 7606 recovery tactics
**RFC 7606:** Defines treat-as-withdraw, attribute discard, AFI/SAFI disable
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP
**Note:** RFC 7606 supersedes RFC 4271 §6 for error handling. Previous "KEEP" was incorrect.

---

## Phase 9: Configuration

### 9.1 Hold Time RFC Validation
**Current:** Accepts 1-2 seconds
**ExaBGP:** Rejects 1-2 seconds per RFC 4271
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP *(duplicate of 3.2)*

### 9.2 Local Address 'auto' Keyword
**Current:** Requires explicit IP
**ExaBGP:** Supports 'auto' for dynamic binding
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 9.3 Extended-Message Capability Config
**Current:** Not in config schema
**ExaBGP:** Explicit leaf with default=true
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

### 9.4 Per-Family Add-Path Config
**Current:** Global only
**ExaBGP:** Can configure per address family
**Decision:** [x] ALIGN / [ ] KEEP / [ ] SKIP

---

## Summary Tracking

| Phase | Total Items | ALIGN | KEEP | SKIP | DONE |
|-------|-------------|-------|------|------|------|
| 1. Critical | 4 | 1 | 0 | 0 | 3 |
| 2. Capabilities | 5 | 3 | 0 | 2 | 0 |
| 3. Timers | 2 | 1 | 1 | 0 | 0 |
| 4. Attributes | 7 | 1 | 3 | 0 | 3 |
| 5. MP-NLRI | 4 | 3 | 0 | 0 | 1 |
| 6. NLRI Types | 5 | 5 | 0 | 0 | 0 |
| 7. FSM | 3 | 0 | 3 | 0 | 0 |
| 8. Errors | 2 | 2 | 0 | 0 | 0 |
| 9. Config | 4 | 4 | 0 | 0 | 0 |
| **Total** | **36** | **20** | **7** | **2** | **7** |

---

## Priority Implementation Order

### High Priority (RFC Compliance)

| Item | Description | Work |
|------|-------------|------|
| 1.3 | Extended Message Integration | Wire `ValidateLengthWithMax()` in session recv |
| 8.2 | RFC 7606 Error Recovery | Implement treat-as-withdraw tactics |
| 3.2 | Hold Time Validation | Reject 1-2 second values |

### Medium Priority (Functionality)

| Item | Description |
|------|-------------|
| 2.1 | RFC 9072 Extended Optional Parameters |
| 2.2 | Enhanced Route Refresh (RFC 7313) |
| 5.2 | Extended Next-Hop Support |
| 5.3 | MP-NLRI Chunking |

### Lower Priority (Features)

| Item | Description |
|------|-------------|
| 4.3 | IPv6 Extended Communities |
| 6.1-6.5 | EVPN/VPLS/RTC NLRI types |
| 8.1 | Additional error subcodes |
| 9.2-9.4 | Config enhancements |

---

## Notes

- Items marked **DONE** were verified by code review on 2025-12-22
- RFC 7606 supersedes RFC 4271 §6 - ZeBGP should implement recovery tactics
- ExaBGP claims verified accurate against source code
