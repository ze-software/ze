# ExaBGP Alignment Implementation Plan

**Source:** `docs/plan/exabgp-alignment.md` (26 ALIGN items) + RFC violations
**Status:** RFC annotation complete, violations merged

---

## Phase 1: Critical Compatibility (5 items) ✅ COMPLETE

### 1.1 RFC 8203/9003 Shutdown Communication
- **Task:** Parse NOTIFICATION data field for Cease/Admin Shutdown
- **RFC:** 8203, 9003
- **Files:** `pkg/bgp/message/notification.go`
- **Test:** Decode shutdown message from peer
- [x] Complete - Added `ShutdownMessage()` method, `*Notification` implements `error`

### 1.2 Per-Message-Type Length Validation
- **Task:** Validate minimum lengths: OPEN≥29, UPDATE≥23, KEEPALIVE==19, RR==23
- **RFC:** 4271 Section 4
- **Files:** `pkg/bgp/message/header.go`
- **Test:** Reject undersized messages with correct error
- [x] Complete - Added `ValidateLength()` method with per-type minimums

### 1.3 Extended Message Size Integration
- **Task:** Apply negotiated max message size after capability exchange
- **RFC:** 8654
- **Files:** `pkg/bgp/message/header.go`
- **Test:** Send/receive >4096 byte UPDATEs when negotiated
- [x] Complete - Added `ValidateLengthWithMax()` and `MaxMessageLength()` functions

### 1.4 KEEPALIVE Payload Validation
- **Task:** Reject KEEPALIVE with non-empty payload
- **RFC:** 4271 Section 4.4
- **Files:** `pkg/bgp/message/keepalive.go`
- **Test:** Send NOTIFICATION on KEEPALIVE with data
- [x] Complete - Returns `*Notification` error on non-empty payload

### 1.5 AS4_PATH Validation
- **Task:** Add missing validation to ParseAS4Path
- **RFC:** 6793 Section 6
- **Files:** `pkg/bgp/attribute/as4.go`
- **Test:** Reject malformed AS4_PATH with correct error
- [x] Complete - Added: odd length check, zero-count rejection, segment type validation

---

## Phase 2: Capabilities (3 items) ✅ COMPLETE

### 2.1 RFC 9072 Extended Optional Parameters
- **Task:** Support 0xFF marker + 2-byte length for large capability sets
- **RFC:** 9072
- **Files:** `pkg/bgp/message/open.go`
- **Test:** Handle >255 bytes of capabilities
- [x] Complete - Pack/Unpack support extended format when params > 255 bytes

### 2.2 Enhanced Route Refresh (RFC 7313)
- **Task:** Implement BoRR/EoRR markers
- **RFC:** 7313
- **Files:** `pkg/bgp/message/routerefresh.go`, `pkg/bgp/capability/`
- **Test:** Send/receive enhanced route refresh
- [x] Complete - Subtype field + EnhancedRouteRefresh capability (code 70)

### 2.5 Capability Conflict Detection
- **Task:** Active detection with mismatch reporting
- **RFC:** 5492
- **Files:** `pkg/bgp/capability/negotiated.go`
- **Test:** Log/report capability mismatches
- [x] Complete - Mismatch struct + tracking in Negotiate()

---

## Phase 3: Timers (1 item) ✅ COMPLETE

### 3.2 Hold Time Validation
- **Task:** Reject hold times 1-2 seconds (must be 0 or ≥3)
- **RFC:** 4271 Section 4.2
- **Files:** `pkg/bgp/message/open.go`, `pkg/config/`
- **Test:** NOTIFICATION on invalid hold time
- [x] Complete - ValidateHoldTime() returns Notification for hold times 1-2

---

## Phase 4: Path Attributes (6 items)

### 4.2 AS_PATH Segment Auto-Split
- **Task:** Auto-split segments at 255 ASNs
- **RFC:** 4271 Section 5.1.2
- **Files:** `pkg/bgp/attribute/aspath.go`
- **Test:** Encode AS_PATH with >255 ASNs
- **Violation:** `aspath.go:197` - Prepend() doesn't handle overflow (SHOULD)
- [x] Complete - Prepend() handles overflow, PackWithASN4() splits during encoding

### 4.3 Extended Communities IPv6 (RFC 5701)
- **Task:** Support 20-byte IPv6 extended communities
- **RFC:** 5701
- **Files:** `pkg/bgp/attribute/community.go`
- **Test:** Parse/encode IPv6 extended community
- [x] Complete - IPv6ExtendedCommunity[20]byte type, AttrIPv6ExtCommunity code 25

### 4.4 Large Community Deduplication
- **Task:** Remove duplicate large communities on receive
- **RFC:** 8092
- **Files:** `pkg/bgp/attribute/community.go`
- **Test:** Deduplicate on unpack
- [x] Complete - Parse and Pack both deduplicate per RFC 8092 Section 5

### 4.7 Attribute Ordering on Send
- **Task:** Order attributes by type code per RFC
- **RFC:** 4271 Appendix F.3
- **Files:** `pkg/bgp/attribute/origin.go`
- **Test:** Verify attribute order in encoded UPDATE
- [x] Complete - OrderAttributes() and PackAttributesOrdered() utility functions

### 4.8 AS4_PATH Merge Semantics ⚠️ NEW (from annotation)
- **Task:** Fix countASNs() to use correct path length semantics
- **RFC:** 6793 Section 4.2.3, RFC 4271 Section 9.1.2.2
- **Files:** `pkg/bgp/attribute/as4.go`
- **Violation:** `as4.go:323-329` - Must count AS_SET=1, confederation=0
- **Test:** Verify merge algorithm uses correct length calculation
- [x] Complete - countASNs() now counts AS_SET=1, confed=0 per RFC 4271 Section 9.1.2.2

### 4.9 AS_CONFED Segment Handling ⚠️ NEW (from annotation)
- **Task:** Discard confed segments in AS4_PATH from OLD speakers
- **RFC:** 6793 Section 3, 6
- **Files:** `pkg/bgp/attribute/as4.go`
- **Violation:** `as4.go:115-146` - MUST discard AS_CONFED_* from OLD
- **Test:** Verify confed segments filtered on receive
- [x] Complete - Pack()/Len() exclude confed, FilterConfedSegments() helper added

---

## Phase 5: MP-NLRI Handling (4 items)

### 5.1 Family Validation Against Negotiated
- **Task:** Reject NLRI for non-negotiated families
- **RFC:** 4760 Section 6
- **Files:** `pkg/reactor/session.go`, `pkg/config/bgp.go`
- **Test:** Ignore/error on non-negotiated AFI/SAFI
- [x] Complete - validateUpdateFamilies() in session.go, config option for buggy peers

### 5.2 Extended Next-Hop Support
- **Task:** Handle IPv6 next-hops for IPv4 prefixes
- **RFC:** 5549/8950
- **Files:** `pkg/bgp/attribute/mpnlri.go`, `pkg/bgp/capability/capability.go`
- **Test:** Parse IPv6 NH for IPv4 NLRI
- [x] Complete - parseNextHops() uses length-based detection per RFC 5549 Section 3

### 5.3 MP-NLRI Chunking
- **Task:** Split large NLRI across multiple UPDATEs
- **RFC:** 4271 Section 4.3
- **Files:** `pkg/bgp/message/update.go`
- **Test:** ChunkNLRI() splits NLRI respecting prefix boundaries
- [x] Complete - ChunkNLRI() utility function for splitting oversized NLRI

### 5.4 Route Distinguisher in Next-Hop
- **Task:** Parse RD in MP_REACH next-hop for VPN
- **RFC:** 4364 Section 4.3.4, 4659, 5549 Section 6
- **Files:** `pkg/bgp/attribute/mpnlri.go`
- **Test:** Parse VPN-IPv4 (12-byte) and VPN-IPv6 (24-byte) next-hops with RD
- [x] Complete - parseVPNNextHops() handles RD prefix and Extended Next Hop

---

## Phase 6: NLRI Types (7 items)

### 6.1 EVPN Type 1 (Ethernet Auto-Discovery)
- **Task:** Full parsing (replace EVPNGeneric)
- **RFC:** 7432 Section 7.1
- **Files:** `pkg/bgp/nlri/evpn.go`
- **Test:** Parse/encode Type 1 EVPN
- [x] Complete - EVPNType1 struct with ESI, EthernetTag, Labels; parseEVPNType1()

### 6.2 EVPN Type 4 (Ethernet Segment)
- **Task:** Full parsing (replace EVPNGeneric)
- **RFC:** 7432 Section 7.4
- **Files:** `pkg/bgp/nlri/evpn.go`
- **Test:** Parse/encode Type 4 EVPN
- [x] Complete - EVPNType4 struct with ESI, OriginatorIP; parseEVPNType4()

### 6.3 FlowSpec VPN Variant
- **Task:** Implement FlowSpec VPN (AFI/SAFI 1/134, 2/134)
- **RFC:** 8955 Section 8
- **Files:** `pkg/bgp/nlri/flowspec.go`
- **Test:** Parse/encode FlowSpec VPN NLRI
- [x] Complete - FlowSpecVPN struct, ParseFlowSpecVPN(), tests exist

### 6.4 VPLS NLRI
- **Task:** Implement VPLS NLRI type
- **RFC:** 4761
- **Files:** `pkg/bgp/nlri/other.go`
- **Test:** Parse/encode VPLS NLRI
- [x] Complete - VPLS struct, ParseVPLS(), NewVPLSFull() already implemented

### 6.5 RTC (Route Target Constraint)
- **Task:** Implement RTC NLRI type
- **RFC:** 4684
- **Files:** `pkg/bgp/nlri/other.go`
- **Test:** Parse/encode RTC NLRI
- [x] Complete - RTC struct, ParseRTC(), NewRTC() already implemented

### 6.6 EVPN Type 5 Prefix Encoding ⚠️ FIXED
- **Task:** Fix IP prefix to use fixed 4/16 octet fields
- **RFC:** 9136 Section 3.1
- **Files:** `pkg/bgp/nlri/evpn.go`
- **Test:** Verify Type 5 uses fixed 4/16 byte prefix field
- [x] Complete - Fixed parseEVPNType5() to require length 34/58, use fixed-size fields

### 6.7 BGP-LS Descriptor Encoding ⚠️ FIXED
- **Task:** Fix link/prefix descriptor TLV encoding
- **RFC:** 7752 Section 3.2
- **Files:** `pkg/bgp/nlri/bgpls.go`
- **Test:** Verify descriptor TLVs appear directly in NLRI
- [x] Complete - Removed container wrapping; TLVs now appear directly per RFC

---

## Phase 8: Error Handling (1 item, 1 kept) ✅ COMPLETE

### 8.1 Error Subcode Coverage
- **Task:** Expand from 12 to 48+ subcodes with descriptions
- **RFC:** 4271, 6608, 7313, 9234, 9384
- **Files:** `pkg/bgp/message/notification.go`
- **Test:** Correct subcode for each error type
- [x] Complete - Added FSM subcodes (RFC 6608), Route Refresh subcodes (RFC 7313),
  Role Mismatch (RFC 9234), BFD Down (RFC 9384), plus string helpers for all codes

---

## Phase 9: Configuration (4 items) ✅ COMPLETE

### 9.1 Hold Time RFC Validation
- **Task:** Config rejects 1-2 second hold times
- **RFC:** 4271 Section 4.2
- **Files:** `pkg/config/bgp.go`
- **Test:** Config validation error
- [x] Complete - parseNeighborConfig validates hold-time per RFC

### 9.2 Local Address 'auto' Keyword
- **Task:** Support 'auto' for dynamic local address binding
- **Files:** `pkg/config/bgp.go`
- **Test:** Bind to auto-selected address
- [x] Complete - LocalAddressAuto field, schema accepts TypeString

### 9.3 Extended-Message Capability Config
- **Task:** Add config option for extended-message
- **RFC:** 8654
- **Files:** `pkg/config/bgp.go`
- **Test:** Enable/disable via config
- [x] Complete - ExtendedMessage field in CapabilityConfig

### 9.4 Per-Family Add-Path Config
- **Task:** Configure add-path per AFI/SAFI
- **RFC:** 7911
- **Files:** `pkg/config/bgp.go`
- **Test:** Per-family add-path settings
- [x] Complete - AddPathFamilies slice with per-family send/receive

---

## Summary

| Phase | Items | New | Status |
|-------|-------|-----|--------|
| 1. Critical | 5 | +1 | ✅ Complete |
| 2. Capabilities | 3 | 0 | ✅ Complete |
| 3. Timers | 1 | 0 | ✅ Complete |
| 4. Attributes | 6 | +2 | ✅ Complete |
| 5. MP-NLRI | 4 | 0 | ✅ Complete |
| 6. NLRI Types | 7 | +2 | ✅ Complete |
| 8. Errors | 1 | 0 | ✅ Complete |
| 9. Config | 4 | 0 | ✅ Complete |
| **Total** | **31** | **+5** | **31/31 Complete** |

---

## Execution Order

1. ✅ Complete RFC annotation (`docs/plan/rfc-annotation.md`)
2. ✅ Merge violations into this plan
3. ✅ Implement Phase 1 (Critical) - All 5 items complete
4. ✅ Implement Phase 2 (Capabilities) - All 3 items complete
5. Next: Phase 3 (Timers)
6. Run `make test` after each item
