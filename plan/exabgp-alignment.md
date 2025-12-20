# ZeBGP/ExaBGP Alignment Review Plan

**Purpose:** Systematically review each difference and decide: align with ExaBGP, keep ZeBGP approach, or skip.

---

## Review Process

For each item, decide:
- **ALIGN** - Change ZeBGP to match ExaBGP behavior
- **KEEP** - Keep ZeBGP's current approach (document why)
- **SKIP** - Not relevant / defer to later

---

## Phase 1: Critical Compatibility Issues

### 1.1 RFC 8203/9003 Shutdown Communication
**Current:** ZeBGP ignores NOTIFICATION data field for Cease/Admin Shutdown
**ExaBGP:** Parses length-prefixed UTF-8 shutdown message
**Impact:** Cannot display graceful shutdown reasons from peers
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 1.2 Per-Message-Type Length Validation
**Current:** ZeBGP only checks >= 19 bytes
**ExaBGP:** OPEN>=29, UPDATE>=23, KEEPALIVE==19, ROUTE_REFRESH==23
**Impact:** May accept malformed messages
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 1.3 Extended Message Size Integration
**Current:** Constants defined (65535) but not applied after negotiation
**ExaBGP:** Sets msg_size immediately when capability negotiated
**Impact:** May fail to send/receive large UPDATE messages
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 1.4 KEEPALIVE Payload Validation
**Current:** ZeBGP silently ignores extra data in KEEPALIVE
**ExaBGP:** Raises error if KEEPALIVE contains any payload
**Impact:** Accepts non-compliant peers
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 2: Capability Differences

### 2.1 RFC 9072 Extended Optional Parameters
**Current:** Not implemented
**ExaBGP:** Supports marker 0xFF 0xFF + 2-byte length for large capability sets
**Impact:** Cannot handle peers with many capabilities (>255 bytes total)
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 2.2 Enhanced Route Refresh (RFC 7313)
**Current:** Not implemented (only basic RFC 2918)
**ExaBGP:** Full support with Begin-of-RR/End-of-RR markers
**Impact:** Cannot do ORF-based route refresh
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 2.3 Multi-Session BGP (Code 68)
**Current:** Not implemented
**ExaBGP:** Full support including Cisco variant (0x83)
**Impact:** Cannot establish multi-session with supporting peers
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 2.4 Dynamic Capability (Code 67)
**Current:** Not implemented
**ExaBGP:** Code defined (not fully implemented either)
**Impact:** Cannot dynamically change capabilities
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 2.5 Capability Conflict Detection
**Current:** Silent intersection (no error reporting)
**ExaBGP:** Active detection with mismatch list and RFC error codes
**Impact:** No visibility into negotiation mismatches
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 3: Timer Defaults

### 3.1 Default Hold Time
**Current:** ZeBGP uses 90 seconds
**ExaBGP:** Uses 180 seconds
**RFC 4271:** Suggests 90 seconds
**Decision:** [ ] ALIGN (180s) / [ ] KEEP (90s) / [ ] SKIP

### 3.2 Hold Time Validation
**Current:** ZeBGP accepts 0-65535
**ExaBGP:** Validates 0 or >= 3 seconds per RFC 4271
**Impact:** May accept invalid hold times (1-2 seconds)
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 4: Path Attributes

### 4.1 ORIGIN Validation
**Current:** ZeBGP rejects invalid values (>2)
**ExaBGP:** Accepts any value
**Decision:** [ ] ALIGN (permissive) / [ ] KEEP (strict) / [ ] SKIP

### 4.2 AS_PATH Segment Auto-Split
**Current:** Caller must handle segment limits
**ExaBGP:** Auto-splits segments at 255 ASNs
**Impact:** May create non-compliant AS_PATH if caller doesn't split
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 4.3 Extended Communities IPv6 (RFC 5701)
**Current:** Not implemented (only 8-byte)
**ExaBGP:** Full 20-byte IPv6 extended community support
**Impact:** Cannot handle IPv6 extended communities
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 4.4 Large Community Deduplication
**Current:** No deduplication
**ExaBGP:** Filters duplicates during unpack
**Impact:** May have duplicate large communities
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 4.5 Community Sorting
**Current:** No sorting
**ExaBGP:** Auto-sorts communities
**Impact:** Order may differ from ExaBGP
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 4.6 Attribute Caching
**Current:** No caching
**ExaBGP:** Extensive caching (ORIGIN, communities, etc.)
**Impact:** Higher memory usage, no deduplication
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 5: MP-NLRI Handling

### 5.1 Family Validation Against Negotiated
**Current:** Not checked
**ExaBGP:** Strict validation against negotiated families
**Impact:** May process NLRI for non-negotiated families
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 5.2 Extended Next-Hop Support
**Current:** Not implemented
**ExaBGP:** Via negotiated.nexthop flag
**Impact:** Cannot handle IPv6 next-hops for IPv4 prefixes
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 5.3 MP-NLRI Chunking
**Current:** No chunking
**ExaBGP:** Respects attribute size limits, splits across UPDATEs
**Impact:** May fail with large NLRI sets
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 5.4 Route Distinguisher in Next-Hop
**Current:** Not handled
**ExaBGP:** Full VPN RD support in MP_REACH next-hop
**Impact:** VPN next-hop parsing may fail
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 6: NLRI Types

### 6.1 EVPN Type 1 (Ethernet Auto-Discovery)
**Current:** Generic wrapper (EVPNGeneric)
**ExaBGP:** Full parsing
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 6.2 EVPN Type 4 (Ethernet Segment)
**Current:** Generic wrapper (EVPNGeneric)
**ExaBGP:** Full parsing
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 6.3 FlowSpec VPN Variant
**Current:** Not implemented
**ExaBGP:** Full support
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 6.4 VPLS NLRI
**Current:** Not implemented
**ExaBGP:** Full support
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 6.5 RTC (Route Target Constraint)
**Current:** Not implemented
**ExaBGP:** Full support
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 7: FSM & Session

### 7.1 FSM Architecture
**Current:** Event-driven with explicit Event() calls
**ExaBGP:** Procedural with direct state changes
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 7.2 Timer Implementation
**Current:** Go time.Timer objects with callbacks
**ExaBGP:** Polling-based with timestamps
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 7.3 Reconnect Backoff
**Current:** Exponential 5s-60s in Peer loop
**ExaBGP:** Delay class with increase/reset
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 8: Error Handling

### 8.1 Error Subcode Coverage
**Current:** 12 subcodes defined
**ExaBGP:** 48+ subcodes with descriptions
**Impact:** Less detailed error messages
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 8.2 Error Recovery in MP-NLRI
**Current:** Strict fail on invalid data
**ExaBGP:** Fallback tactics (e.g., zeros for invalid next-hop)
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

---

## Phase 9: Configuration

### 9.1 Hold Time RFC Validation
**Current:** Accepts 1-2 seconds
**ExaBGP:** Rejects 1-2 seconds per RFC 4271
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 9.2 Local Address 'auto' Keyword
**Current:** Requires explicit IP
**ExaBGP:** Supports 'auto' for dynamic binding
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 9.3 Extended-Message Capability Config
**Current:** Not in config schema
**ExaBGP:** Explicit leaf with default=true
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

### 9.4 Per-Family Add-Path Config
**Current:** Global only
**ExaBGP:** Can configure per address family
**Decision:** [ ] ALIGN / [ ] KEEP / [ ] SKIP

---

## Summary Tracking

| Phase | Total Items | ALIGN | KEEP | SKIP |
|-------|-------------|-------|------|------|
| 1. Critical | 4 | | | |
| 2. Capabilities | 5 | | | |
| 3. Timers | 2 | | | |
| 4. Attributes | 6 | | | |
| 5. MP-NLRI | 4 | | | |
| 6. NLRI Types | 5 | | | |
| 7. FSM | 3 | | | |
| 8. Errors | 2 | | | |
| 9. Config | 4 | | | |
| **Total** | **35** | | | |

---

## Next Steps

After review:
1. Create implementation tasks for ALIGN items
2. Document rationale for KEEP items
3. Archive SKIP items for future consideration
