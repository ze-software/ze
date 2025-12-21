# ExaBGP Alignment Implementation Plan

**Source:** `plan/exabgp-alignment.md` (26 ALIGN items) + RFC violations
**Status:** RFC annotation complete, violations merged

---

## Phase 1: Critical Compatibility (5 items)

### 1.1 RFC 8203/9003 Shutdown Communication
- **Task:** Parse NOTIFICATION data field for Cease/Admin Shutdown
- **RFC:** 8203, 9003
- **Files:** `pkg/bgp/message/notification.go`
- **Test:** Decode shutdown message from peer
- [ ] Pending

### 1.2 Per-Message-Type Length Validation
- **Task:** Validate minimum lengths: OPEN≥29, UPDATE≥23, KEEPALIVE==19, RR==23
- **RFC:** 4271 Section 4
- **Files:** `pkg/bgp/message/header.go`, per-message files
- **Test:** Reject undersized messages with correct error
- **Violation:** `header.go:92` - Also missing upper bound (4096)
- [ ] Pending

### 1.3 Extended Message Size Integration
- **Task:** Apply negotiated max message size after capability exchange
- **RFC:** 8654
- **Files:** `pkg/bgp/fsm/`, `pkg/bgp/capability/extendedmessage.go`
- **Test:** Send/receive >4096 byte UPDATEs when negotiated
- **Violation:** `header.go:92` - Must validate against negotiated max (4096 or 65535)
- [ ] Pending

### 1.4 KEEPALIVE Payload Validation
- **Task:** Reject KEEPALIVE with non-empty payload
- **RFC:** 4271 Section 4.4
- **Files:** `pkg/bgp/message/keepalive.go`
- **Test:** Send NOTIFICATION on KEEPALIVE with data
- [ ] Pending

### 1.5 AS4_PATH Validation ⚠️ NEW (from annotation)
- **Task:** Add missing validation to ParseAS4Path
- **RFC:** 6793 Section 6
- **Files:** `pkg/bgp/attribute/as4.go`
- **Violations:**
  - Validate min length (6 bytes)
  - Check length is multiple of 2
  - Reject zero-length segments
  - Validate segment type (1-4)
  - Discard AS_CONFED_* from OLD speakers (MUST)
- **Test:** Reject malformed AS4_PATH with correct error
- [ ] Pending

---

## Phase 2: Capabilities (3 items, 2 skipped)

### 2.1 RFC 9072 Extended Optional Parameters
- **Task:** Support 0xFF marker + 2-byte length for large capability sets
- **RFC:** 9072
- **Files:** `pkg/bgp/message/open.go`
- **Test:** Handle >255 bytes of capabilities
- [ ] Pending

### 2.2 Enhanced Route Refresh (RFC 7313)
- **Task:** Implement BoRR/EoRR markers
- **RFC:** 7313
- **Files:** `pkg/bgp/message/routerefresh.go`, `pkg/bgp/capability/`
- **Test:** Send/receive enhanced route refresh
- [ ] Pending

### 2.5 Capability Conflict Detection
- **Task:** Active detection with mismatch reporting
- **RFC:** 5492
- **Files:** `pkg/bgp/fsm/`, `pkg/bgp/capability/`
- **Test:** Log/report capability mismatches
- [ ] Pending

---

## Phase 3: Timers (1 item, 1 kept)

### 3.2 Hold Time Validation
- **Task:** Reject hold times 1-2 seconds (must be 0 or ≥3)
- **RFC:** 4271 Section 4.2
- **Files:** `pkg/bgp/message/open.go`, `pkg/config/`
- **Test:** NOTIFICATION on invalid hold time
- [ ] Pending

---

## Phase 4: Path Attributes (6 items)

### 4.2 AS_PATH Segment Auto-Split
- **Task:** Auto-split segments at 255 ASNs
- **RFC:** 4271 Section 5.1.2
- **Files:** `pkg/bgp/attribute/aspath.go`
- **Test:** Encode AS_PATH with >255 ASNs
- **Violation:** `aspath.go:197` - Prepend() doesn't handle overflow (SHOULD)
- [ ] Pending

### 4.3 Extended Communities IPv6 (RFC 5701)
- **Task:** Support 20-byte IPv6 extended communities
- **RFC:** 5701
- **Files:** `pkg/bgp/attribute/extcommunity.go`
- **Test:** Parse/encode IPv6 extended community
- [ ] Pending

### 4.4 Large Community Deduplication
- **Task:** Remove duplicate large communities on receive
- **RFC:** 8092
- **Files:** `pkg/bgp/attribute/largecommunity.go`
- **Test:** Deduplicate on unpack
- [ ] Pending

### 4.7 Attribute Ordering on Send
- **Task:** Order attributes by type code per RFC
- **RFC:** 4271 Appendix F.3
- **Files:** `pkg/bgp/message/update.go`
- **Test:** Verify attribute order in encoded UPDATE
- [ ] Pending

### 4.8 AS4_PATH Merge Semantics ⚠️ NEW (from annotation)
- **Task:** Fix countASNs() to use correct path length semantics
- **RFC:** 6793 Section 4.2.3, RFC 4271 Section 9.1.2.2
- **Files:** `pkg/bgp/attribute/as4.go`
- **Violation:** `as4.go:323-329` - Must count AS_SET=1, confederation=0
- **Test:** Verify merge algorithm uses correct length calculation
- [ ] Pending

### 4.9 AS_CONFED Segment Handling ⚠️ NEW (from annotation)
- **Task:** Discard confed segments in AS4_PATH from OLD speakers
- **RFC:** 6793 Section 6
- **Files:** `pkg/bgp/attribute/as4.go`
- **Violation:** `as4.go:115-146` - MUST discard AS_CONFED_* from OLD
- **Test:** Verify confed segments filtered on receive
- [ ] Pending

---

## Phase 5: MP-NLRI Handling (4 items)

### 5.1 Family Validation Against Negotiated
- **Task:** Reject NLRI for non-negotiated families
- **RFC:** 4271 Section 9
- **Files:** `pkg/bgp/fsm/`, `pkg/bgp/message/update.go`
- **Test:** Ignore/error on non-negotiated AFI/SAFI
- [ ] Pending

### 5.2 Extended Next-Hop Support
- **Task:** Handle IPv6 next-hops for IPv4 prefixes
- **RFC:** 5549
- **Files:** `pkg/bgp/attribute/mpreach.go`, `pkg/bgp/capability/`
- **Test:** Parse IPv6 NH for IPv4 NLRI
- [ ] Pending

### 5.3 MP-NLRI Chunking
- **Task:** Split large NLRI across multiple UPDATEs
- **RFC:** 4271, 4760
- **Files:** `pkg/bgp/message/update.go`
- **Test:** Chunk >4096 byte NLRI sets
- [ ] Pending

### 5.4 Route Distinguisher in Next-Hop
- **Task:** Parse RD in MP_REACH next-hop for VPN
- **RFC:** 4364, 4659
- **Files:** `pkg/bgp/attribute/mpreach.go`
- **Test:** Parse VPN next-hop with RD
- [ ] Pending

---

## Phase 6: NLRI Types (7 items)

### 6.1 EVPN Type 1 (Ethernet Auto-Discovery)
- **Task:** Full parsing (replace EVPNGeneric)
- **RFC:** 7432 Section 7.1
- **Files:** `pkg/bgp/nlri/evpn.go`
- **Test:** Parse/encode Type 1 EVPN
- [ ] Pending

### 6.2 EVPN Type 4 (Ethernet Segment)
- **Task:** Full parsing (replace EVPNGeneric)
- **RFC:** 7432 Section 7.4
- **Files:** `pkg/bgp/nlri/evpn.go`
- **Test:** Parse/encode Type 4 EVPN
- [ ] Pending

### 6.3 FlowSpec VPN Variant
- **Task:** Implement FlowSpec VPN (AFI/SAFI 1/133, 2/133)
- **RFC:** 8955
- **Files:** `pkg/bgp/nlri/flowspec.go`
- **Test:** Parse/encode FlowSpec VPN NLRI
- [ ] Pending

### 6.4 VPLS NLRI
- **Task:** Implement VPLS NLRI type
- **RFC:** 4761
- **Files:** `pkg/bgp/nlri/vpls.go` (new)
- **Test:** Parse/encode VPLS NLRI
- [ ] Pending

### 6.5 RTC (Route Target Constraint)
- **Task:** Implement RTC NLRI type
- **RFC:** 4684
- **Files:** `pkg/bgp/nlri/rtc.go` (new)
- **Test:** Parse/encode RTC NLRI
- [ ] Pending

### 6.6 EVPN Type 5 Prefix Encoding ⚠️ NEW (from annotation)
- **Task:** Fix IP prefix to use fixed 4/16 octet fields
- **RFC:** 9136 Section 3
- **Files:** `pkg/bgp/nlri/evpn.go`
- **Violation:** `evpn.go:440` - Uses variable-length, should be fixed size
- **Test:** Verify Type 5 uses fixed 4/16 byte prefix field
- [ ] Pending

### 6.7 BGP-LS Descriptor Encoding ⚠️ NEW (from annotation)
- **Task:** Fix link/prefix descriptor TLV encoding
- **RFC:** 7752 Section 3.2
- **Files:** `pkg/bgp/nlri/bgpls.go`
- **Violations:**
  - `bgpls.go:168` - TLV 258 is Link Local/Remote ID, not container
  - `bgpls.go:193` - TLV 264 is OSPF Route Type, not container
  - `bgpls.go:440,521` - Bytes() wraps incorrectly
- **Test:** Verify descriptor TLVs appear directly in NLRI
- [ ] Pending

---

## Phase 8: Error Handling (1 item, 1 kept)

### 8.1 Error Subcode Coverage
- **Task:** Expand from 12 to 48+ subcodes with descriptions
- **RFC:** 4271, various
- **Files:** `pkg/bgp/message/notification.go`
- **Test:** Correct subcode for each error type
- [ ] Pending

---

## Phase 9: Configuration (4 items)

### 9.1 Hold Time RFC Validation
- **Task:** Config rejects 1-2 second hold times
- **RFC:** 4271
- **Files:** `pkg/config/`
- **Test:** Config validation error
- [ ] Pending (duplicate of 3.2)

### 9.2 Local Address 'auto' Keyword
- **Task:** Support 'auto' for dynamic local address binding
- **Files:** `pkg/config/`, `pkg/bgp/peer.go`
- **Test:** Bind to auto-selected address
- [ ] Pending

### 9.3 Extended-Message Capability Config
- **Task:** Add config option with default=true
- **RFC:** 8654
- **Files:** `pkg/config/`
- **Test:** Enable/disable via config
- [ ] Pending

### 9.4 Per-Family Add-Path Config
- **Task:** Configure add-path per AFI/SAFI
- **RFC:** 7911
- **Files:** `pkg/config/`
- **Test:** Per-family add-path settings
- [ ] Pending

---

## Summary

| Phase | Items | New | Status |
|-------|-------|-----|--------|
| 1. Critical | 5 | +1 | Pending |
| 2. Capabilities | 3 | 0 | Pending |
| 3. Timers | 1 | 0 | Pending |
| 4. Attributes | 6 | +2 | Pending |
| 5. MP-NLRI | 4 | 0 | Pending |
| 6. NLRI Types | 7 | +2 | Pending |
| 8. Errors | 1 | 0 | Pending |
| 9. Config | 4 | 0 | Pending |
| **Total** | **31** | **+5** | **Pending** |

---

## Execution Order

1. ✅ Complete RFC annotation (`plan/rfc-annotation.md`)
2. ✅ Merge violations into this plan
3. Implement Phase 1 (Critical) first - includes AS4_PATH fixes
4. Proceed phase by phase with TDD
5. Run `make test` after each item
