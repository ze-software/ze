package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC7606MalformedOriginLength verifies RFC 7606 Section 7.1.
func TestRFC7606MalformedOriginLength(t *testing.T) {
	// ORIGIN with wrong length (2 instead of 1)
	pathAttrs := []byte{
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN len=2 (invalid)
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(1), result.AttrCode)
	require.Contains(t, result.Description, "7.1")
}

// TestRFC7606MalformedCommunityLength verifies RFC 7606 Section 7.8.
func TestRFC7606MalformedCommunityLength(t *testing.T) {
	// Valid ORIGIN + AS_PATH + NEXT_HOP, then malformed Community
	pathAttrs := []byte{
		// ORIGIN = IGP
		0x40, 0x01, 0x01, 0x00,
		// AS_PATH (empty)
		0x40, 0x02, 0x00,
		// NEXT_HOP = 192.0.2.1
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
		// COMMUNITY with wrong length (5, not multiple of 4)
		0xc0, 0x08, 0x05, 0x00, 0x01, 0x00, 0x02, 0x03,
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(8), result.AttrCode)
	require.Contains(t, result.Description, "7.8")
}

// TestRFC7606MissingOrigin verifies RFC 7606 Section 3.d.
func TestRFC7606MissingOrigin(t *testing.T) {
	// Missing ORIGIN (only AS_PATH and NEXT_HOP)
	pathAttrs := []byte{
		// AS_PATH (empty)
		0x40, 0x02, 0x00,
		// NEXT_HOP = 192.0.2.1
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(1), result.AttrCode)
	require.Contains(t, result.Description, "ORIGIN")
}

// TestRFC7606MissingASPath verifies RFC 7606 Section 3.d.
func TestRFC7606MissingASPath(t *testing.T) {
	// Missing AS_PATH (only ORIGIN and NEXT_HOP)
	pathAttrs := []byte{
		// ORIGIN = IGP
		0x40, 0x01, 0x01, 0x00,
		// NEXT_HOP = 192.0.2.1
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(2), result.AttrCode)
	require.Contains(t, result.Description, "AS_PATH")
}

// TestRFC7606MalformedAtomicAggregate verifies RFC 7606 Section 7.6.
func TestRFC7606MalformedAtomicAggregate(t *testing.T) {
	// ATOMIC_AGGREGATE with wrong length (should be 0)
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x40, 0x06, 0x01, 0x00, // ATOMIC_AGGREGATE len=1 (invalid)
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action)
	require.Equal(t, uint8(6), result.AttrCode)
	require.Contains(t, result.Description, "7.6")
}

// TestRFC7606ValidUpdate verifies valid UPDATE passes validation.
func TestRFC7606ValidUpdate(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606EmptyWithdrawal verifies withdraw-only UPDATE is valid.
func TestRFC7606EmptyWithdrawal(t *testing.T) {
	// No path attributes, no NLRI = withdrawal only (valid)
	result := ValidateUpdateRFC7606(nil, false, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606MultipleMPReach verifies RFC 7606 Section 3.g.
func TestRFC7606MultipleMPReach(t *testing.T) {
	// Two MP_REACH_NLRI attributes (invalid per RFC 7606 Section 3.g)
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// First MP_REACH_NLRI: AFI=1, SAFI=1, NH_LEN=4, valid next-hop
		0x80, 0x0e, 0x09, // code=14, len=9
		0x00, 0x01, // AFI=1
		0x01,                   // SAFI=1
		0x04,                   // NH_LEN=4
		0xc0, 0x00, 0x02, 0x01, // 192.0.2.1
		0x00, // Reserved
		// Second MP_REACH_NLRI (triggers multiple MP_REACH error)
		0x80, 0x0e, 0x09, // code=14, len=9
		0x00, 0x01, // AFI=1 (same as first - both valid)
		0x01,                   // SAFI=1
		0x04,                   // NH_LEN=4
		0xc0, 0x00, 0x02, 0x02, // 192.0.2.2
		0x00, // Reserved
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionSessionReset, result.Action)
	require.Contains(t, result.Description, "3.g")
}

// TestRFC7606ExtendedCommunityLength verifies RFC 7606 Section 7.14.
func TestRFC7606ExtendedCommunityLength(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		// Extended Community with wrong length (5, not multiple of 8)
		0xc0, 0x10, 0x05, 0x00, 0x01, 0x00, 0x02, 0x03,
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(16), result.AttrCode)
	require.Contains(t, result.Description, "7.14")
}

// TestRFC7606LargeCommunityLength verifies RFC 8092 Section 5.
func TestRFC7606LargeCommunityLength(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		// Large Community with wrong length (10, not multiple of 12)
		0xc0, 0x20, 0x0a, 0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04, 0x00, 0x05,
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(32), result.AttrCode)
	require.Contains(t, result.Description, "8092")
}

// =============================================================================
// RFC 7606 Section 7.2 - AS_PATH Segment Validation Tests
// =============================================================================

// TestRFC7606ASPathValidSequence verifies valid AS_SEQUENCE passes.
//
// VALIDATES: AS_PATH with valid AS_SEQUENCE (type=2) is accepted.
// PREVENTS: False positives in AS_PATH validation.
func TestRFC7606ASPathValidSequence(t *testing.T) {
	// AS_PATH with AS_SEQUENCE: type=2, len=2 (2 ASes), AS 65001, AS 65002
	// Using 2-byte ASNs
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x06, // AS_PATH, len=6
		0x02, 0x02, // AS_SEQUENCE, 2 ASes
		0xfd, 0xe9, // AS 65001
		0xfd, 0xea, // AS 65002
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606ASPathValidSet verifies valid AS_SET passes.
//
// VALIDATES: AS_PATH with valid AS_SET (type=1) is accepted.
// PREVENTS: False positives in AS_PATH validation.
func TestRFC7606ASPathValidSet(t *testing.T) {
	// AS_PATH with AS_SET: type=1, len=2 (2 ASes)
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x06, // AS_PATH, len=6
		0x01, 0x02, // AS_SET, 2 ASes
		0xfd, 0xe9, // AS 65001
		0xfd, 0xea, // AS 65002
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606ASPathUnrecognizedSegmentType verifies RFC 7606 Section 7.2.
//
// VALIDATES: Unrecognized segment type triggers treat-as-withdraw.
// PREVENTS: Accepting AS_PATH with invalid segment types (security).
func TestRFC7606ASPathUnrecognizedSegmentType(t *testing.T) {
	// AS_PATH with invalid segment type=5 (only 1-4 are valid)
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x04, // AS_PATH, len=4
		0x05, 0x01, // Invalid type=5, 1 AS
		0xfd, 0xe9, // AS 65001
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(2), result.AttrCode)
	require.Contains(t, result.Description, "segment type")
}

// TestRFC7606ASPathSegmentOverrun verifies RFC 7606 Section 7.2.
//
// VALIDATES: Segment length exceeding attribute data triggers treat-as-withdraw.
// PREVENTS: Buffer overflow from malformed AS_PATH (security).
func TestRFC7606ASPathSegmentOverrun(t *testing.T) {
	// AS_PATH where segment claims 10 ASes but only has room for 1
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x04, // AS_PATH, len=4 (only room for segment header + 1 AS)
		0x02, 0x0a, // AS_SEQUENCE, 10 ASes (but only 2 bytes of AS data follow)
		0xfd, 0xe9, // AS 65001 (only 1 AS, not 10)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(2), result.AttrCode)
	require.Contains(t, result.Description, "overrun")
}

// TestRFC7606ASPathSegmentUnderrun verifies RFC 7606 Section 7.2.
//
// VALIDATES: Single trailing byte after segments triggers treat-as-withdraw.
// PREVENTS: Accepting malformed AS_PATH with partial segment header.
func TestRFC7606ASPathSegmentUnderrun(t *testing.T) {
	// AS_PATH with valid segment + 1 trailing byte (partial header)
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x05, // AS_PATH, len=5
		0x02, 0x01, // AS_SEQUENCE, 1 AS
		0xfd, 0xe9, // AS 65001
		0x02,                                     // Trailing byte - partial segment header (underrun)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(2), result.AttrCode)
	require.Contains(t, result.Description, "underrun")
}

// TestRFC7606ASPathZeroSegmentLength verifies RFC 7606 Section 7.2.
//
// VALIDATES: Zero segment length triggers treat-as-withdraw.
// PREVENTS: Accepting AS_PATH with empty segments (RFC violation).
func TestRFC7606ASPathZeroSegmentLength(t *testing.T) {
	// AS_PATH with segment length=0
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x02, // AS_PATH, len=2
		0x02, 0x00, // AS_SEQUENCE, 0 ASes (invalid)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(2), result.AttrCode)
	require.Contains(t, result.Description, "zero")
}

// =============================================================================
// RFC 7606 Section 7.1 - ORIGIN Value Validation Tests
// =============================================================================

// TestRFC7606OriginValueIGP verifies valid ORIGIN=IGP (0) passes.
//
// VALIDATES: ORIGIN value 0 (IGP) is accepted.
// PREVENTS: False positives in ORIGIN value validation.
func TestRFC7606OriginValueIGP(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP (0)
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606OriginValueEGP verifies valid ORIGIN=EGP (1) passes.
//
// VALIDATES: ORIGIN value 1 (EGP) is accepted.
// PREVENTS: False positives in ORIGIN value validation.
func TestRFC7606OriginValueEGP(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x01, // ORIGIN = EGP (1)
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606OriginValueIncomplete verifies valid ORIGIN=INCOMPLETE (2) passes.
//
// VALIDATES: ORIGIN value 2 (INCOMPLETE) is accepted.
// PREVENTS: False positives in ORIGIN value validation.
func TestRFC7606OriginValueIncomplete(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x02, // ORIGIN = INCOMPLETE (2)
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606OriginValueInvalid verifies RFC 7606 Section 7.1.
//
// VALIDATES: Invalid ORIGIN value (>2) triggers treat-as-withdraw.
// PREVENTS: Accepting UPDATE with undefined ORIGIN values.
func TestRFC7606OriginValueInvalid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x03, // ORIGIN = 3 (invalid)
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(1), result.AttrCode)
	require.Contains(t, result.Description, "7.1")
}

// =============================================================================
// RFC 7606 Section 7.11 - MP_REACH_NLRI Next-Hop Length Validation Tests
// =============================================================================

// TestRFC7606MPReachIPv6NextHopValid verifies valid IPv6 next-hop (16 bytes).
//
// VALIDATES: MP_REACH with 16-byte next-hop for IPv6/unicast is accepted.
// PREVENTS: False positives in MP_REACH validation.
func TestRFC7606MPReachIPv6NextHopValid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// MP_REACH_NLRI: flags=0x90 (optional, extended), code=14, len=21
		// AFI=2 (IPv6), SAFI=1 (unicast), NH_LEN=16, 16-byte next-hop, reserved=0
		0x90, 0x0e, 0x00, 0x15, // Optional, extended length, code=14, len=21
		0x00, 0x02, // AFI=2 (IPv6)
		0x01,                                           // SAFI=1 (unicast)
		0x10,                                           // NH_LEN=16
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, // 2001:db8::1
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, // Reserved (SNPA)
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606MPReachIPv6NextHopDualValid verifies valid dual IPv6 next-hop (32 bytes).
//
// VALIDATES: MP_REACH with 32-byte next-hop (global + link-local) is accepted.
// PREVENTS: False positives in MP_REACH validation.
func TestRFC7606MPReachIPv6NextHopDualValid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// MP_REACH_NLRI: len=37 (AFI+SAFI+NH_LEN+32+reserved)
		0x90, 0x0e, 0x00, 0x25, // code=14, len=37
		0x00, 0x02, // AFI=2 (IPv6)
		0x01, // SAFI=1 (unicast)
		0x20, // NH_LEN=32 (global + link-local)
		// Global: 2001:db8::1
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		// Link-local: fe80::1
		0xfe, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, // Reserved (SNPA)
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606MPReachIPv6NextHopInvalid verifies RFC 7606 Section 7.11.
//
// VALIDATES: MP_REACH with invalid next-hop length triggers session-reset.
// PREVENTS: Accepting MP_REACH with corrupt next-hop (can't parse NLRI).
func TestRFC7606MPReachIPv6NextHopInvalid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// MP_REACH_NLRI: AFI=2, SAFI=1, NH_LEN=5 (invalid for IPv6)
		0x90, 0x0e, 0x00, 0x0a, // code=14, len=10
		0x00, 0x02, // AFI=2 (IPv6)
		0x01,                         // SAFI=1 (unicast)
		0x05,                         // NH_LEN=5 (invalid - should be 16 or 32)
		0x01, 0x02, 0x03, 0x04, 0x05, // 5 bytes (invalid)
		0x00, // Reserved (SNPA)
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionSessionReset, result.Action)
	require.Equal(t, uint8(14), result.AttrCode)
	require.Contains(t, result.Description, "7.11")
}

// TestRFC7606MPReachIPv4NextHopValid verifies valid IPv4 next-hop (4 bytes).
//
// VALIDATES: MP_REACH with 4-byte next-hop for IPv4/unicast is accepted.
// PREVENTS: False positives in MP_REACH validation.
func TestRFC7606MPReachIPv4NextHopValid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// MP_REACH_NLRI: AFI=1, SAFI=1, NH_LEN=4
		0x90, 0x0e, 0x00, 0x09, // code=14, len=9
		0x00, 0x01, // AFI=1 (IPv4)
		0x01,                   // SAFI=1 (unicast)
		0x04,                   // NH_LEN=4
		0xc0, 0x00, 0x02, 0x01, // 192.0.2.1
		0x00, // Reserved (SNPA)
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606MPReachIPv4NextHopInvalid verifies RFC 7606 Section 7.11.
//
// VALIDATES: MP_REACH with invalid next-hop length for IPv4 triggers session-reset.
// PREVENTS: Accepting MP_REACH with corrupt next-hop.
func TestRFC7606MPReachIPv4NextHopInvalid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// MP_REACH_NLRI: AFI=1, SAFI=1, NH_LEN=3 (invalid for IPv4)
		0x90, 0x0e, 0x00, 0x08, // code=14, len=8
		0x00, 0x01, // AFI=1 (IPv4)
		0x01,             // SAFI=1 (unicast)
		0x03,             // NH_LEN=3 (invalid - should be 4)
		0xc0, 0x00, 0x02, // 3 bytes (invalid)
		0x00, // Reserved (SNPA)
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionSessionReset, result.Action)
	require.Equal(t, uint8(14), result.AttrCode)
	require.Contains(t, result.Description, "7.11")
}

// TestRFC7606MPReachVPNv4NextHopValid verifies valid VPNv4 next-hop (12 bytes).
//
// VALIDATES: MP_REACH with 12-byte next-hop for VPNv4 (SAFI=128) is accepted.
// PREVENTS: False positives in MP_REACH validation for L3VPN.
func TestRFC7606MPReachVPNv4NextHopValid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// MP_REACH_NLRI: AFI=1, SAFI=128 (VPN), NH_LEN=12 (8-byte RD + 4-byte IPv4)
		0x90, 0x0e, 0x00, 0x11, // code=14, len=17
		0x00, 0x01, // AFI=1 (IPv4)
		0x80, // SAFI=128 (VPN)
		0x0c, // NH_LEN=12
		// 8-byte RD (type 0, ASN 65000, assigned 1)
		0x00, 0x00, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01,
		// 4-byte IPv4 next-hop
		0xc0, 0x00, 0x02, 0x01, // 192.0.2.1
		0x00, // Reserved (SNPA)
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606MPReachVPNv4NextHopInvalid verifies RFC 7606 Section 7.11 for VPNv4.
//
// VALIDATES: MP_REACH with invalid next-hop length for VPNv4 triggers session-reset.
// PREVENTS: Accepting VPNv4 UPDATE with corrupt next-hop.
func TestRFC7606MPReachVPNv4NextHopInvalid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// MP_REACH_NLRI: AFI=1, SAFI=128 (VPN), NH_LEN=4 (wrong - should be 12)
		0x90, 0x0e, 0x00, 0x09, // code=14, len=9
		0x00, 0x01, // AFI=1 (IPv4)
		0x80,                   // SAFI=128 (VPN)
		0x04,                   // NH_LEN=4 (invalid - should be 12)
		0xc0, 0x00, 0x02, 0x01, // Only 4 bytes (no RD)
		0x00, // Reserved (SNPA)
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionSessionReset, result.Action)
	require.Equal(t, uint8(14), result.AttrCode)
	require.Contains(t, result.Description, "7.11")
}

// TestRFC7606MPReachTooShort verifies RFC 7606 Section 5.3.
//
// VALIDATES: MP_REACH with length < 5 triggers session-reset.
// PREVENTS: Buffer overflow from truncated MP_REACH.
func TestRFC7606MPReachTooShort(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// MP_REACH_NLRI: len=4 (too short, minimum is 5)
		0x80, 0x0e, 0x04, // code=14, len=4
		0x00, 0x01, // AFI=1
		0x01, // SAFI=1
		0x00, // NH_LEN (incomplete, no reserved byte)
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionSessionReset, result.Action)
	require.Equal(t, uint8(14), result.AttrCode)
}

// =============================================================================
// RFC 7606 Section 5.3 - NLRI Syntactic Validation Tests
// =============================================================================

// TestRFC7606NLRIPrefixLengthValidIPv4 verifies valid IPv4 NLRI.
//
// VALIDATES: IPv4 NLRI with prefix lengths 0-32 are accepted.
// PREVENTS: False positives in NLRI validation.
func TestRFC7606NLRIPrefixLengthValidIPv4(t *testing.T) {
	// Valid NLRI: /24 prefix (3 bytes) + /32 prefix (4 bytes)
	nlri := []byte{
		24, 192, 168, 1, // 192.168.1.0/24
		32, 10, 0, 0, 1, // 10.0.0.1/32
	}

	result := ValidateNLRISyntax(nlri, false)
	require.Nil(t, result)
}

// TestRFC7606NLRIPrefixLengthTooLongIPv4 verifies RFC 7606 Section 5.3.
//
// VALIDATES: IPv4 NLRI with prefix length > 32 triggers treat-as-withdraw.
// PREVENTS: Accepting invalid NLRI with impossible prefix length.
func TestRFC7606NLRIPrefixLengthTooLongIPv4(t *testing.T) {
	// Invalid NLRI: prefix length 33 (> 32)
	nlri := []byte{
		24, 192, 168, 1, // 192.168.1.0/24 (valid)
		33, 10, 0, 0, 1, 0, // prefix length 33 (invalid)
	}

	result := ValidateNLRISyntax(nlri, false)
	require.NotNil(t, result)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Contains(t, result.Description, "5.3")
}

// TestRFC7606NLRIPrefixLengthValidIPv6 verifies valid IPv6 NLRI.
//
// VALIDATES: IPv6 NLRI with prefix lengths 0-128 are accepted.
// PREVENTS: False positives in NLRI validation.
func TestRFC7606NLRIPrefixLengthValidIPv6(t *testing.T) {
	// Valid NLRI: /64 prefix (8 bytes) + /128 prefix (16 bytes)
	nlri := []byte{
		64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, // 2001:db8::/64
		128, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, // 2001:db8::1/128
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	}

	result := ValidateNLRISyntax(nlri, true)
	require.Nil(t, result)
}

// TestRFC7606NLRIPrefixLengthTooLongIPv6 verifies RFC 7606 Section 5.3.
//
// VALIDATES: IPv6 NLRI with prefix length > 128 triggers treat-as-withdraw.
// PREVENTS: Accepting invalid NLRI with impossible prefix length.
func TestRFC7606NLRIPrefixLengthTooLongIPv6(t *testing.T) {
	// Invalid NLRI: prefix length 129 (> 128)
	nlri := []byte{
		64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, // 2001:db8::/64 (valid)
		129, // prefix length 129 (invalid)
	}

	result := ValidateNLRISyntax(nlri, true)
	require.NotNil(t, result)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Contains(t, result.Description, "5.3")
}

// TestRFC7606NLRIOverrun verifies RFC 7606 Section 3(j) + Section 5.3.
//
// VALIDATES: NLRI overrun triggers session-reset (entire field not parseable).
// PREVENTS: Using treat-as-withdraw when NLRI can't be fully parsed.
//
// RFC 7606 Section 3(j): treat-as-withdraw requires the entire NLRI field to be
// successfully parsed. If not possible, session-reset MUST be followed.
func TestRFC7606NLRIOverrun(t *testing.T) {
	// Invalid NLRI: claims /24 but only 2 bytes follow (needs 3)
	nlri := []byte{
		24, 192, 168, // Missing last byte
	}

	result := ValidateNLRISyntax(nlri, false)
	require.NotNil(t, result)
	require.Equal(t, RFC7606ActionSessionReset, result.Action)
	require.Contains(t, result.Description, "overrun")
}

// TestRFC7606NLRIEmpty verifies empty NLRI is valid.
//
// VALIDATES: Empty NLRI field is accepted.
// PREVENTS: False positives on withdrawal-only UPDATEs.
func TestRFC7606NLRIEmpty(t *testing.T) {
	result := ValidateNLRISyntax(nil, false)
	require.Nil(t, result)

	result = ValidateNLRISyntax([]byte{}, false)
	require.Nil(t, result)
}

// TestRFC7606NLRIZeroPrefixLength verifies /0 prefix is valid.
//
// VALIDATES: Prefix length 0 (default route) is accepted.
// PREVENTS: Rejecting valid default route announcements.
func TestRFC7606NLRIZeroPrefixLength(t *testing.T) {
	// /0 prefix = 1 byte (just length, no prefix bytes)
	nlri := []byte{0}

	result := ValidateNLRISyntax(nlri, false)
	require.Nil(t, result)
}

// =============================================================================
// RFC 7606 Section 7.5 - LOCAL_PREF IBGP Context Tests
// =============================================================================

// TestRFC7606LocalPrefEBGPDiscard verifies RFC 7606 Section 7.5.
//
// VALIDATES: LOCAL_PREF from EBGP peer triggers attribute-discard.
// PREVENTS: Accepting LOCAL_PREF from external peers (RFC violation).
func TestRFC7606LocalPrefEBGPDiscard(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64, // LOCAL_PREF = 100
	}

	// EBGP session (isIBGP=false)
	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action)
	require.Equal(t, uint8(5), result.AttrCode)
	require.Contains(t, result.Description, "7.5")
}

// TestRFC7606LocalPrefIBGPValid verifies valid LOCAL_PREF in IBGP.
//
// VALIDATES: LOCAL_PREF with length=4 from IBGP peer is accepted.
// PREVENTS: False positives in LOCAL_PREF validation.
func TestRFC7606LocalPrefIBGPValid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64, // LOCAL_PREF = 100 (len=4)
	}

	// IBGP session (isIBGP=true)
	result := ValidateUpdateRFC7606(pathAttrs, true, true, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606LocalPrefIBGPInvalid verifies RFC 7606 Section 7.5.
//
// VALIDATES: LOCAL_PREF with invalid length from IBGP triggers treat-as-withdraw.
// PREVENTS: Accepting malformed LOCAL_PREF in IBGP.
func TestRFC7606LocalPrefIBGPInvalid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x40, 0x05, 0x03, 0x00, 0x00, 0x64, // LOCAL_PREF len=3 (invalid)
	}

	// IBGP session (isIBGP=true)
	result := ValidateUpdateRFC7606(pathAttrs, true, true, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(5), result.AttrCode)
	require.Contains(t, result.Description, "7.5")
}

// =============================================================================
// RFC 7606 Section 7.9 - ORIGINATOR_ID IBGP Context Tests
// =============================================================================

// TestRFC7606OriginatorIDEBGPDiscard verifies RFC 7606 Section 7.9.
//
// VALIDATES: ORIGINATOR_ID from EBGP peer triggers attribute-discard.
// PREVENTS: Accepting route reflector attributes from external peers.
func TestRFC7606OriginatorIDEBGPDiscard(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x80, 0x09, 0x04, 0x0a, 0x00, 0x00, 0x01, // ORIGINATOR_ID = 10.0.0.1
	}

	// EBGP session (isIBGP=false)
	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action)
	require.Equal(t, uint8(9), result.AttrCode)
	require.Contains(t, result.Description, "7.9")
}

// TestRFC7606OriginatorIDIBGPValid verifies valid ORIGINATOR_ID in IBGP.
//
// VALIDATES: ORIGINATOR_ID with length=4 from IBGP peer is accepted.
// PREVENTS: False positives in ORIGINATOR_ID validation.
func TestRFC7606OriginatorIDIBGPValid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x80, 0x09, 0x04, 0x0a, 0x00, 0x00, 0x01, // ORIGINATOR_ID = 10.0.0.1 (len=4)
	}

	// IBGP session (isIBGP=true)
	result := ValidateUpdateRFC7606(pathAttrs, true, true, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606OriginatorIDIBGPInvalid verifies RFC 7606 Section 7.9.
//
// VALIDATES: ORIGINATOR_ID with invalid length from IBGP triggers treat-as-withdraw.
// PREVENTS: Accepting malformed ORIGINATOR_ID in IBGP.
func TestRFC7606OriginatorIDIBGPInvalid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x80, 0x09, 0x05, 0x0a, 0x00, 0x00, 0x01, 0x00, // ORIGINATOR_ID len=5 (invalid)
	}

	// IBGP session (isIBGP=true)
	result := ValidateUpdateRFC7606(pathAttrs, true, true, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(9), result.AttrCode)
	require.Contains(t, result.Description, "7.9")
}

// =============================================================================
// RFC 7606 Section 7.10 - CLUSTER_LIST IBGP Context Tests
// =============================================================================

// TestRFC7606ClusterListEBGPDiscard verifies RFC 7606 Section 7.10.
//
// VALIDATES: CLUSTER_LIST from EBGP peer triggers attribute-discard.
// PREVENTS: Accepting route reflector attributes from external peers.
func TestRFC7606ClusterListEBGPDiscard(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x80, 0x0a, 0x04, 0x0a, 0x00, 0x00, 0x01, // CLUSTER_LIST = [10.0.0.1]
	}

	// EBGP session (isIBGP=false)
	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action)
	require.Equal(t, uint8(10), result.AttrCode)
	require.Contains(t, result.Description, "7.10")
}

// TestRFC7606ClusterListIBGPValid verifies valid CLUSTER_LIST in IBGP.
//
// VALIDATES: CLUSTER_LIST with length multiple of 4 from IBGP is accepted.
// PREVENTS: False positives in CLUSTER_LIST validation.
func TestRFC7606ClusterListIBGPValid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		// CLUSTER_LIST with 2 entries (8 bytes)
		0x80, 0x0a, 0x08,
		0x0a, 0x00, 0x00, 0x01, // Cluster ID 1
		0x0a, 0x00, 0x00, 0x02, // Cluster ID 2
	}

	// IBGP session (isIBGP=true)
	result := ValidateUpdateRFC7606(pathAttrs, true, true, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606ClusterListIBGPInvalid verifies RFC 7606 Section 7.10.
//
// VALIDATES: CLUSTER_LIST with invalid length from IBGP triggers treat-as-withdraw.
// PREVENTS: Accepting malformed CLUSTER_LIST in IBGP.
func TestRFC7606ClusterListIBGPInvalid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x80, 0x0a, 0x05, 0x0a, 0x00, 0x00, 0x01, 0x0a, // CLUSTER_LIST len=5 (invalid)
	}

	// IBGP session (isIBGP=true)
	result := ValidateUpdateRFC7606(pathAttrs, true, true, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(10), result.AttrCode)
	require.Contains(t, result.Description, "7.10")
}

// =============================================================================
// RFC 7606 Section 3.c - Attribute Flags Validation Tests
// =============================================================================

// TestRFC7606FlagsWellKnownMarkedOptional verifies RFC 7606 Section 3.c.
//
// VALIDATES: Well-known attribute with Optional bit set triggers treat-as-withdraw.
// PREVENTS: Accepting well-known attributes incorrectly marked as optional.
func TestRFC7606FlagsWellKnownMarkedOptional(t *testing.T) {
	pathAttrs := []byte{
		// ORIGIN with Optional bit set (0xc0 = optional + transitive)
		// Should be 0x40 (transitive only, not optional)
		0xc0, 0x01, 0x01, 0x00, // ORIGIN with Optional=1 (invalid)
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(1), result.AttrCode)
	require.Contains(t, result.Description, "3.c")
}

// TestRFC7606FlagsWellKnownNotTransitive verifies RFC 7606 Section 3.c.
//
// VALIDATES: Well-known attribute without Transitive bit triggers treat-as-withdraw.
// PREVENTS: Accepting well-known attributes incorrectly marked as non-transitive.
func TestRFC7606FlagsWellKnownNotTransitive(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN (valid)
		// AS_PATH without Transitive bit (0x00 instead of 0x40)
		0x00, 0x02, 0x00, // AS_PATH with Transitive=0 (invalid)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(2), result.AttrCode)
	require.Contains(t, result.Description, "3.c")
}

// TestRFC7606FlagsOptionalAttributeValid verifies optional attributes pass.
//
// VALIDATES: Optional attribute with Optional bit set is accepted.
// PREVENTS: False positives on correctly flagged optional attributes.
func TestRFC7606FlagsOptionalAttributeValid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		// MED (code=4) is optional non-transitive: 0x80 (optional, not transitive)
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED=100
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// =============================================================================
// RFC 7606 Section 3.g - Duplicate Attribute Tests
// =============================================================================

// TestRFC7606DuplicateOrigin verifies RFC 7606 Section 3.g.
//
// VALIDATES: Duplicate well-known attribute discards all but first.
// PREVENTS: Processing conflicting attribute values.
func TestRFC7606DuplicateOrigin(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP (first)
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x40, 0x01, 0x01, 0x02, // ORIGIN = INCOMPLETE (duplicate - should be discarded)
	}

	// Per RFC 7606 3.g: discard all but first, no error
	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606DuplicateMED verifies RFC 7606 Section 3.g for optional attrs.
//
// VALIDATES: Duplicate optional attribute discards all but first.
// PREVENTS: Processing conflicting attribute values.
func TestRFC7606DuplicateMED(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED=100 (first)
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0xc8, // MED=200 (duplicate)
	}

	// Per RFC 7606 3.g: discard all but first, no error
	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// =============================================================================
// RFC 7606 Section 7.7 - AGGREGATOR with 4-Octet AS Context Tests
// =============================================================================

// TestRFC7606AggregatorLen6NoASN4 verifies valid AGGREGATOR without 4-octet AS.
//
// VALIDATES: AGGREGATOR with length 6 is valid when asn4=false.
// PREVENTS: False positives for correctly sized AGGREGATOR.
func TestRFC7606AggregatorLen6NoASN4(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		// AGGREGATOR: flags=0xc0 (optional transitive), code=7, len=6
		0xc0, 0x07, 0x06,
		0xfd, 0xe9, // AS 65001 (2 bytes)
		0xc0, 0x00, 0x02, 0x01, // Router ID 192.0.2.1
	}

	// asn4=false, so length 6 is correct
	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606AggregatorLen8WithASN4 verifies valid AGGREGATOR with 4-octet AS.
//
// VALIDATES: AGGREGATOR with length 8 is valid when asn4=true.
// PREVENTS: False positives for correctly sized AGGREGATOR.
func TestRFC7606AggregatorLen8WithASN4(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		// AGGREGATOR: flags=0xc0 (optional transitive), code=7, len=8
		0xc0, 0x07, 0x08,
		0x00, 0x00, 0xfd, 0xe9, // AS 65001 (4 bytes)
		0xc0, 0x00, 0x02, 0x01, // Router ID 192.0.2.1
	}

	// asn4=true, so length 8 is correct
	result := ValidateUpdateRFC7606(pathAttrs, true, false, true)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606AggregatorLen8NoASN4Invalid verifies RFC 7606 Section 7.7.
//
// VALIDATES: AGGREGATOR with length 8 when asn4=false triggers attribute-discard.
// PREVENTS: Accepting AGGREGATOR with wrong size for capability.
func TestRFC7606AggregatorLen8NoASN4Invalid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		// AGGREGATOR: len=8 but asn4=false (expects 6)
		0xc0, 0x07, 0x08,
		0x00, 0x00, 0xfd, 0xe9, // AS (4 bytes - wrong)
		0xc0, 0x00, 0x02, 0x01, // Router ID
	}

	// asn4=false but length is 8 (should be 6) - attribute-discard
	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action)
	require.Equal(t, uint8(7), result.AttrCode)
	require.Contains(t, result.Description, "7.7")
}

// TestRFC7606AggregatorLen6WithASN4Invalid verifies RFC 7606 Section 7.7.
//
// VALIDATES: AGGREGATOR with length 6 when asn4=true triggers attribute-discard.
// PREVENTS: Accepting AGGREGATOR with wrong size for capability.
func TestRFC7606AggregatorLen6WithASN4Invalid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		// AGGREGATOR: len=6 but asn4=true (expects 8)
		0xc0, 0x07, 0x06,
		0xfd, 0xe9, // AS (2 bytes - wrong)
		0xc0, 0x00, 0x02, 0x01, // Router ID
	}

	// asn4=true but length is 6 (should be 8) - attribute-discard
	result := ValidateUpdateRFC7606(pathAttrs, true, false, true)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action)
	require.Equal(t, uint8(7), result.AttrCode)
	require.Contains(t, result.Description, "7.7")
}

// TestRFC7606ASPath4ByteASN verifies AS_PATH validation with 4-byte ASNs.
//
// VALIDATES: AS_PATH segment validation works correctly with asn4=true.
// PREVENTS: AS_PATH validation errors with 4-byte ASN sessions.
func TestRFC7606ASPath4ByteASN(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		// AS_PATH with 4-byte ASNs: type=2, len=2, AS 65001, AS 65002
		0x40, 0x02, 0x0a, // AS_PATH, len=10 (2+4+4)
		0x02, 0x02, // AS_SEQUENCE, 2 ASes
		0x00, 0x00, 0xfd, 0xe9, // AS 65001 (4 bytes)
		0x00, 0x00, 0xfd, 0xea, // AS 65002 (4 bytes)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	// asn4=true - should validate correctly
	result := ValidateUpdateRFC7606(pathAttrs, true, false, true)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// =============================================================================
// RFC 7606 Validation Gap Tests (new — Section 5.2, 5.3, 3.h)
// =============================================================================

// TestRFC7606MPUnreachTooShort verifies RFC 7606 Section 5.3.
//
// VALIDATES: MP_UNREACH_NLRI with length < 3 triggers session-reset.
// PREVENTS: Accepting truncated MP_UNREACH that can't contain AFI+SAFI.
// BOUNDARY: 2 (invalid, minimum is 3), 3 (valid).
func TestRFC7606MPUnreachTooShort(t *testing.T) {
	// MP_UNREACH_NLRI with length 2 (needs at least 3: AFI=2 + SAFI=1)
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// MP_UNREACH_NLRI: code=15, len=2 (too short, minimum 3)
		0x80, 0x0f, 0x02,
		0x00, 0x01, // AFI=1 (only 2 bytes, missing SAFI)
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionSessionReset, result.Action)
	require.Equal(t, uint8(15), result.AttrCode)
	require.Contains(t, result.Description, "5.3")
}

// TestRFC7606MPUnreachMinValid verifies MP_UNREACH_NLRI with length exactly 3 is valid.
//
// VALIDATES: MP_UNREACH_NLRI with minimum valid length (3) passes.
// PREVENTS: False positives at the boundary.
// BOUNDARY: 3 (valid, exactly AFI+SAFI).
func TestRFC7606MPUnreachMinValid(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		// MP_UNREACH_NLRI: code=15, len=3 (minimum valid: AFI=2 + SAFI=1)
		0x80, 0x0f, 0x03,
		0x00, 0x01, // AFI=1
		0x01, // SAFI=1
	}

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606NoNLRIEscalation verifies RFC 7606 Section 5.2.
//
// VALIDATES: UPDATE with attrs but no NLRI + treat-as-withdraw error → session-reset.
// PREVENTS: Accepting malformed attrs when NLRI can't be confirmed parseable.
func TestRFC7606NoNLRIEscalation(t *testing.T) {
	// UPDATE with path attributes but no NLRI, and a malformed ORIGIN (treat-as-withdraw)
	// Section 5.2: "session reset" MUST be used since we can't confirm NLRI parsed correctly
	pathAttrs := []byte{
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN len=2 (malformed → treat-as-withdraw)
		0x40, 0x02, 0x00, // AS_PATH (empty)
	}

	// hasNLRI=false, no MP_REACH → Section 5.2 escalation
	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionSessionReset, result.Action)
	require.Contains(t, result.Description, "5.2")
}

// TestRFC7606NoNLRIAttributeDiscardNoEscalation verifies Section 5.2 exception.
//
// VALIDATES: UPDATE with attrs but no NLRI + only attribute-discard errors → NOT escalated.
// PREVENTS: Over-escalating when the only errors are harmless discards.
func TestRFC7606NoNLRIAttributeDiscardNoEscalation(t *testing.T) {
	// UPDATE with path attributes but no NLRI, only attribute-discard error
	// Section 5.2 says: escalate only for errors OTHER than attribute-discard
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x06, 0x01, 0x00, // ATOMIC_AGG len=1 (attribute-discard)
	}

	// hasNLRI=false, only attribute-discard → NOT escalated
	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action)
}

// TestRFC7606MultipleErrorsStrongest verifies RFC 7606 Section 3.h.
//
// VALIDATES: When multiple errors exist, strongest action (treat-as-withdraw) wins.
// PREVENTS: Using weaker action when stronger is required.
func TestRFC7606MultipleErrorsStrongest(t *testing.T) {
	// UPDATE with:
	// - ATOMIC_AGG wrong length → attribute-discard (weaker)
	// - Community wrong length → treat-as-withdraw (stronger)
	// Section 3.h: use strongest action
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x40, 0x06, 0x01, 0x00, // ATOMIC_AGG len=1 (attribute-discard)
		0xc0, 0x08, 0x05, 0x00, 0x01, 0x00, 0x02, 0x03, // Community len=5 (treat-as-withdraw)
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(8), result.AttrCode) // Community caused the strongest error
}

// TestRFC7606CollectAllErrors verifies all attribute-discard entries are collected.
//
// VALIDATES: Multiple attribute-discard errors populate DiscardEntries with all codes and reasons.
// PREVENTS: Stripping only the first bad attribute when multiple need stripping.
func TestRFC7606CollectAllErrors(t *testing.T) {
	// UPDATE with two attribute-discard errors:
	// - ATOMIC_AGG wrong length → discard (reason: invalid length)
	// - AGGREGATOR wrong length → discard (reason: invalid length)
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
		0x40, 0x06, 0x01, 0x00, // ATOMIC_AGG len=1 (discard)
		// AGGREGATOR wrong length (asn4=false, expects 6 but got 8)
		0xc0, 0x07, 0x08,
		0x00, 0x00, 0xfd, 0xe9,
		0xc0, 0x00, 0x02, 0x01,
	}

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action)
	require.Len(t, result.DiscardEntries, 2)
	// Both should have reason code DiscardReasonInvalidLength (2).
	codes := make(map[uint8]uint8)
	for _, e := range result.DiscardEntries {
		codes[e.Code] = e.Reason
	}
	require.Equal(t, DiscardReasonInvalidLength, codes[6]) // ATOMIC_AGG
	require.Equal(t, DiscardReasonInvalidLength, codes[7]) // AGGREGATOR
}

// =============================================================================
// Systematic Length Corruption — RFC 7606 Recovery
// =============================================================================

// TestRFC7606SystematicLengthCorruption takes a complete valid attribute set
// and systematically corrupts each attribute's length field to be larger or
// smaller than its actual content. This exercises:
//   - Per-attribute validation when the length is wrong but structurally parseable
//   - Cascading misparse when a shifted boundary causes subsequent attributes
//     to be read starting at wrong offsets
//   - Structural bounds checking when inflated length exceeds remaining data
//
// VALIDATES: Every attribute type handles both inflated and deflated length
//
//	fields with the correct RFC 7606 action (no panics, no silent acceptance).
//
// PREVENTS: Buffer overflows, panics from cascading misparse, or silent
//
//	acceptance of corrupted UPDATEs that should trigger error handling.
func TestRFC7606SystematicLengthCorruption(t *testing.T) {
	// encodeAttr builds flags + code + length + value.
	encodeAttr := func(flags, code byte, value []byte) []byte {
		out := []byte{flags, code, byte(len(value))}
		return append(out, value...)
	}

	// assemble concatenates attributes, overriding one length byte.
	// corruptIdx < 0 means no corruption.
	assemble := func(attrs [][]byte, corruptIdx int, newLen byte) []byte {
		var buf []byte
		for i, a := range attrs {
			dup := make([]byte, len(a))
			copy(dup, a)
			if i == corruptIdx {
				dup[2] = newLen
			}
			buf = append(buf, dup...)
		}
		return buf
	}

	// EBGP attribute set: 7 attributes covering well-known + optional + transitive.
	ebgpSet := [][]byte{
		encodeAttr(0x40, 1, []byte{0x00}),                               // [0] ORIGIN=IGP (len=1)
		encodeAttr(0x40, 2, []byte{0x02, 0x01, 0xfd, 0xe9}),             // [1] AS_PATH=[65001] (len=4)
		encodeAttr(0x40, 3, []byte{0xc0, 0x00, 0x02, 0x01}),             // [2] NEXT_HOP=192.0.2.1 (len=4)
		encodeAttr(0x80, 4, []byte{0x00, 0x00, 0x00, 0x64}),             // [3] MED=100 (len=4)
		encodeAttr(0x40, 6, []byte{}),                                   // [4] ATOMIC_AGG (len=0)
		encodeAttr(0xc0, 7, []byte{0xfd, 0xe9, 0xc0, 0x00, 0x02, 0x01}), // [5] AGGREGATOR (len=6)
		encodeAttr(0xc0, 8, []byte{0xfd, 0xe8, 0x00, 0x64}),             // [6] COMMUNITY=65000:100 (len=4)
	}

	// IBGP attribute set: adds LOCAL_PREF, ORIGINATOR_ID, CLUSTER_LIST.
	ibgpSet := [][]byte{
		encodeAttr(0x40, 1, []byte{0x00}),                                            // [0] ORIGIN=IGP
		encodeAttr(0x40, 2, []byte{0x02, 0x01, 0xfd, 0xe9}),                          // [1] AS_PATH=[65001]
		encodeAttr(0x40, 3, []byte{0xc0, 0x00, 0x02, 0x01}),                          // [2] NEXT_HOP=192.0.2.1
		encodeAttr(0x40, 5, []byte{0x00, 0x00, 0x00, 0x64}),                          // [3] LOCAL_PREF=100 (len=4)
		encodeAttr(0x80, 9, []byte{0x0a, 0x00, 0x00, 0x01}),                          // [4] ORIGINATOR_ID (len=4)
		encodeAttr(0x80, 10, []byte{0x0a, 0x00, 0x00, 0x01, 0x0a, 0x00, 0x00, 0x02}), // [5] CLUSTER_LIST (len=8)
		encodeAttr(0xc0, 8, []byte{0xfd, 0xe8, 0x00, 0x64}),                          // [6] COMMUNITY=65000:100
	}

	// Verify baselines are valid before corrupting anything.
	t.Run("Baseline_EBGP", func(t *testing.T) {
		result := ValidateUpdateRFC7606(assemble(ebgpSet, -1, 0), true, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action, "EBGP baseline must pass")
	})
	t.Run("Baseline_IBGP", func(t *testing.T) {
		result := ValidateUpdateRFC7606(assemble(ibgpSet, -1, 0), true, true, false)
		require.Equal(t, RFC7606ActionNone, result.Action, "IBGP baseline must pass")
	})

	tests := []struct {
		name    string
		attrs   [][]byte
		isIBGP  bool
		idx     int  // Attribute index to corrupt
		newLen  byte // Corrupted length value
		wantMin RFC7606Action
	}{
		// ==================================================================
		// EBGP: Length inflation (too big)
		// ==================================================================
		// Middle attrs: inflated length steals bytes from next attribute,
		// shifting all following attribute boundaries → cascading misparse.
		{"EBGP/ORIGIN_inflate+1", ebgpSet, false, 0, 2, RFC7606ActionTreatAsWithdraw},
		{"EBGP/AS_PATH_inflate+1", ebgpSet, false, 1, 5, RFC7606ActionTreatAsWithdraw},
		{"EBGP/NEXT_HOP_inflate+1", ebgpSet, false, 2, 5, RFC7606ActionTreatAsWithdraw},
		{"EBGP/MED_inflate+1", ebgpSet, false, 3, 5, RFC7606ActionTreatAsWithdraw},
		// Attribute-discard attrs: per-attr validation gives discard,
		// but cascade from shifted boundary may escalate to withdraw.
		{"EBGP/ATOMIC_AGG_inflate+1", ebgpSet, false, 4, 1, RFC7606ActionAttributeDiscard},
		{"EBGP/AGGREGATOR_inflate+1", ebgpSet, false, 5, 7, RFC7606ActionAttributeDiscard},
		// Last attr: inflated length exceeds remaining data (structural error).
		{"EBGP/COMMUNITY_inflate+1_last", ebgpSet, false, 6, 5, RFC7606ActionTreatAsWithdraw},
		// Extreme: len 255 on a 1-byte attribute → far exceeds remaining.
		{"EBGP/ORIGIN_inflate_to_255", ebgpSet, false, 0, 255, RFC7606ActionTreatAsWithdraw},

		// ==================================================================
		// EBGP: Length deflation (too small)
		// ==================================================================
		// Per-attribute validation catches wrong length, then leftover bytes
		// from the real attribute value cascade as phantom attribute headers.
		{"EBGP/ORIGIN_deflate_to_0", ebgpSet, false, 0, 0, RFC7606ActionTreatAsWithdraw},
		{"EBGP/AS_PATH_deflate_to_0", ebgpSet, false, 1, 0, RFC7606ActionTreatAsWithdraw},
		{"EBGP/AS_PATH_deflate_to_1", ebgpSet, false, 1, 1, RFC7606ActionTreatAsWithdraw},
		{"EBGP/AS_PATH_deflate_to_2", ebgpSet, false, 1, 2, RFC7606ActionTreatAsWithdraw},
		{"EBGP/AS_PATH_deflate-1", ebgpSet, false, 1, 3, RFC7606ActionTreatAsWithdraw},
		{"EBGP/NEXT_HOP_deflate-1", ebgpSet, false, 2, 3, RFC7606ActionTreatAsWithdraw},
		{"EBGP/NEXT_HOP_deflate_to_0", ebgpSet, false, 2, 0, RFC7606ActionTreatAsWithdraw},
		{"EBGP/MED_deflate-1", ebgpSet, false, 3, 3, RFC7606ActionTreatAsWithdraw},
		{"EBGP/MED_deflate_to_0", ebgpSet, false, 3, 0, RFC7606ActionTreatAsWithdraw},
		{"EBGP/AGGREGATOR_deflate-1", ebgpSet, false, 5, 5, RFC7606ActionAttributeDiscard},
		{"EBGP/COMMUNITY_deflate-1", ebgpSet, false, 6, 3, RFC7606ActionTreatAsWithdraw},
		{"EBGP/COMMUNITY_deflate_to_0", ebgpSet, false, 6, 0, RFC7606ActionTreatAsWithdraw},

		// ==================================================================
		// IBGP: Length inflation
		// ==================================================================
		{"IBGP/LOCAL_PREF_inflate+1", ibgpSet, true, 3, 5, RFC7606ActionTreatAsWithdraw},
		{"IBGP/ORIGINATOR_ID_inflate+1", ibgpSet, true, 4, 5, RFC7606ActionTreatAsWithdraw},
		{"IBGP/CLUSTER_LIST_inflate+1", ibgpSet, true, 5, 9, RFC7606ActionTreatAsWithdraw},

		// ==================================================================
		// IBGP: Length deflation
		// ==================================================================
		{"IBGP/LOCAL_PREF_deflate-1", ibgpSet, true, 3, 3, RFC7606ActionTreatAsWithdraw},
		{"IBGP/ORIGINATOR_ID_deflate-1", ibgpSet, true, 4, 3, RFC7606ActionTreatAsWithdraw},
		{"IBGP/CLUSTER_LIST_deflate-1", ibgpSet, true, 5, 7, RFC7606ActionTreatAsWithdraw},
		{"IBGP/CLUSTER_LIST_deflate_to_5", ibgpSet, true, 5, 5, RFC7606ActionTreatAsWithdraw},
		{"IBGP/CLUSTER_LIST_deflate_to_0", ibgpSet, true, 5, 0, RFC7606ActionTreatAsWithdraw},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pa := assemble(tt.attrs, tt.idx, tt.newLen)
			result := ValidateUpdateRFC7606(pa, true, tt.isIBGP, false)
			require.GreaterOrEqual(t, int(result.Action), int(tt.wantMin),
				"want at least %s, got %s: %s", tt.wantMin, result.Action, result.Description)
		})
	}

	// ==================================================================
	// Padded attributes: length is wrong but the extra bytes are present
	// in the buffer (garbage padding), so structural parsing succeeds
	// (attribute boundaries still align). Only per-attribute validation
	// catches the error. This is different from the cases above where
	// inflated length steals bytes from the next attribute.
	//
	// Each case builds a complete path-attributes blob where the corrupted
	// attribute appears ONCE (not as a duplicate). For well-known mandatory
	// attrs (ORIGIN, AS_PATH, NEXT_HOP), the base omits that attribute.
	// ==================================================================
	bOrigin := []byte{0x40, 0x01, 0x01, 0x00}                    // ORIGIN = IGP
	bASPath := []byte{0x40, 0x02, 0x00}                          // AS_PATH (empty)
	bNextHop := []byte{0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01} // NEXT_HOP = 192.0.2.1
	join := func(parts ...[]byte) []byte {
		var buf []byte
		for _, p := range parts {
			buf = append(buf, p...)
		}
		return buf
	}

	paddedTests := []struct {
		name     string
		pathAttr []byte // Complete path-attributes blob
		isIBGP   bool
		wantMin  RFC7606Action
		wantCode uint8
	}{
		// ORIGIN len=5: valid value (0x00) + 4 garbage bytes.
		// Per-attribute: length 5 != 1 → treat-as-withdraw.
		{
			"padded/ORIGIN_len5_garbage",
			join(
				[]byte{0x40, 0x01, 0x05, 0x00, 0xff, 0xff, 0xff, 0xff},
				bASPath, bNextHop,
			),
			false, RFC7606ActionTreatAsWithdraw, 1,
		},
		// NEXT_HOP len=8: valid 4-byte IP + 4 garbage bytes.
		// Per-attribute: length 8 != 4 → treat-as-withdraw.
		{
			"padded/NEXT_HOP_len8_garbage",
			join(
				bOrigin, bASPath,
				[]byte{0x40, 0x03, 0x08, 0xc0, 0x00, 0x02, 0x01, 0xff, 0xff, 0xff, 0xff},
			),
			false, RFC7606ActionTreatAsWithdraw, 3,
		},
		// MED len=8: valid 4 bytes + 4 garbage bytes.
		// Per-attribute: length 8 != 4 → treat-as-withdraw.
		{
			"padded/MED_len8_garbage",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0x80, 0x04, 0x08, 0x00, 0x00, 0x00, 0x64, 0xff, 0xff, 0xff, 0xff},
			),
			false, RFC7606ActionTreatAsWithdraw, 4,
		},
		// COMMUNITY len=5: valid 4-byte community + 1 garbage byte.
		// Per-attribute: length 5 not multiple of 4 → treat-as-withdraw.
		{
			"padded/COMMUNITY_len5_garbage",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0xc0, 0x08, 0x05, 0xfd, 0xe8, 0x00, 0x64, 0xff},
			),
			false, RFC7606ActionTreatAsWithdraw, 8,
		},
		// COMMUNITY len=8: two communities, second is all 0xFF.
		// Per-attribute: length 8 IS multiple of 4 → passes length check.
		// Garbage community 0xFFFFFFFF is syntactically valid per RFC 7606.
		{
			"padded/COMMUNITY_len8_garbage_valid",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0xc0, 0x08, 0x08, 0xfd, 0xe8, 0x00, 0x64, 0xff, 0xff, 0xff, 0xff},
			),
			false, RFC7606ActionNone, 0,
		},
		// AS_PATH with extra garbage after valid segment.
		// Valid segment: type=2, count=1, AS=65001 (4 bytes).
		// Garbage tail: 0xFF, 0xFF → parsed as segment type=0xFF (unrecognized).
		// Per-attribute: AS_PATH segment type > 4 → treat-as-withdraw.
		{
			"padded/AS_PATH_garbage_segment",
			join(
				bOrigin,
				[]byte{0x40, 0x02, 0x06, 0x02, 0x01, 0xfd, 0xe9, 0xff, 0xff},
				bNextHop,
			),
			false, RFC7606ActionTreatAsWithdraw, 2,
		},
		// AS_PATH with garbage that looks like a zero-length segment.
		// Valid segment + type=2, count=0 → zero segment length → treat-as-withdraw.
		{
			"padded/AS_PATH_zero_segment_tail",
			join(
				bOrigin,
				[]byte{0x40, 0x02, 0x06, 0x02, 0x01, 0xfd, 0xe9, 0x02, 0x00},
				bNextHop,
			),
			false, RFC7606ActionTreatAsWithdraw, 2,
		},
		// ATOMIC_AGGREGATE len=4: should be 0, has 4 garbage bytes.
		// Per-attribute: length 4 != 0 → attribute-discard.
		{
			"padded/ATOMIC_AGG_len4_garbage",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0x40, 0x06, 0x04, 0xff, 0xff, 0xff, 0xff},
			),
			false, RFC7606ActionAttributeDiscard, 6,
		},
		// AGGREGATOR len=10 (asn4=false, expects 6): 6 valid + 4 garbage.
		// Per-attribute: length 10 != 6 → attribute-discard.
		{
			"padded/AGGREGATOR_len10_garbage",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0xc0, 0x07, 0x0a, 0xfd, 0xe9, 0xc0, 0x00, 0x02, 0x01, 0xff, 0xff, 0xff, 0xff},
			),
			false, RFC7606ActionAttributeDiscard, 7,
		},
		// LOCAL_PREF len=8 in IBGP: valid 4 bytes + 4 garbage.
		// Per-attribute: length 8 != 4 → treat-as-withdraw.
		{
			"padded/LOCAL_PREF_len8_IBGP",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0x40, 0x05, 0x08, 0x00, 0x00, 0x00, 0x64, 0xff, 0xff, 0xff, 0xff},
			),
			true, RFC7606ActionTreatAsWithdraw, 5,
		},
		// ORIGINATOR_ID len=8 in IBGP: valid 4 bytes + 4 garbage.
		// Per-attribute: length 8 != 4 → treat-as-withdraw.
		{
			"padded/ORIGINATOR_ID_len8_IBGP",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0x80, 0x09, 0x08, 0x0a, 0x00, 0x00, 0x01, 0xff, 0xff, 0xff, 0xff},
			),
			true, RFC7606ActionTreatAsWithdraw, 9,
		},
		// CLUSTER_LIST len=5 in IBGP: valid 4 bytes + 1 garbage byte.
		// Per-attribute: length 5 not multiple of 4 → treat-as-withdraw.
		{
			"padded/CLUSTER_LIST_len5_IBGP",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0x80, 0x0a, 0x05, 0x0a, 0x00, 0x00, 0x01, 0xff},
			),
			true, RFC7606ActionTreatAsWithdraw, 10,
		},
	}

	for _, tt := range paddedTests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateUpdateRFC7606(tt.pathAttr, true, tt.isIBGP, false)
			require.GreaterOrEqual(t, int(result.Action), int(tt.wantMin),
				"want at least %s, got %s: %s", tt.wantMin, result.Action, result.Description)
			if tt.wantCode != 0 {
				require.Equal(t, tt.wantCode, result.AttrCode)
			}
		})
	}

	// ==================================================================
	// Duplicate attribute × corruption interaction tests.
	//
	// RFC 7606 Section 3.g: "discard all but the first occurrence" for
	// duplicate attributes (except MP_REACH/MP_UNREACH → session-reset).
	//
	// The duplicate check (line 234 in rfc7606.go) runs AFTER flag
	// validation but BEFORE per-attribute validation. This means:
	//
	//   - Valid first + corrupted duplicate → duplicate skipped before
	//     validation, no error detected from the duplicate.
	//   - Corrupted first + valid duplicate → first is validated (error
	//     recorded), duplicate skipped. Error propagates.
	//   - Both corrupted → first validated (error), second skipped.
	//   - MP_REACH duplicates → session-reset regardless of corruption.
	//
	// All cases use padded buffers so structural parsing succeeds and
	// tests focus purely on the duplicate-skip vs validation ordering.
	// ==================================================================
	dupTests := []struct {
		name     string
		pathAttr []byte
		isIBGP   bool
		wantMin  RFC7606Action
		wantCode uint8 // 0 = don't check
	}{
		// Valid ORIGIN first, corrupted ORIGIN duplicate (len=5, padded).
		// Duplicate is silently discarded before validateAttribute runs.
		{
			"dup/valid_first_corrupted_second_ORIGIN",
			join(
				bOrigin, // valid ORIGIN len=1
				bASPath, bNextHop,
				[]byte{0x40, 0x01, 0x05, 0x00, 0xff, 0xff, 0xff, 0xff}, // ORIGIN len=5 (dup, padded)
			),
			false, RFC7606ActionNone, 0,
		},
		// Corrupted ORIGIN first (len=5, padded), valid ORIGIN duplicate.
		// First ORIGIN is validated → treat-as-withdraw (len != 1).
		// Second ORIGIN is skipped as duplicate → error propagates.
		{
			"dup/corrupted_first_valid_second_ORIGIN",
			join(
				[]byte{0x40, 0x01, 0x05, 0x00, 0xff, 0xff, 0xff, 0xff}, // ORIGIN len=5 (padded)
				bASPath, bNextHop,
				bOrigin, // valid ORIGIN (dup, skipped)
			),
			false, RFC7606ActionTreatAsWithdraw, 1,
		},
		// Both ORIGINs corrupted. First validated (error), second skipped.
		{
			"dup/both_corrupted_ORIGIN",
			join(
				[]byte{0x40, 0x01, 0x05, 0x00, 0xff, 0xff, 0xff, 0xff}, // ORIGIN len=5 (padded)
				bASPath, bNextHop,
				[]byte{0x40, 0x01, 0x03, 0x00, 0xff, 0xff}, // ORIGIN len=3 (dup, padded)
			),
			false, RFC7606ActionTreatAsWithdraw, 1,
		},
		// Valid MED first, corrupted MED duplicate (len=8, padded).
		// Duplicate skipped → no error from the corrupted copy.
		{
			"dup/valid_first_corrupted_second_MED",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64},                         // MED=100 (valid)
				[]byte{0x80, 0x04, 0x08, 0x00, 0x00, 0x00, 0xc8, 0xff, 0xff, 0xff, 0xff}, // MED len=8 (dup, padded)
			),
			false, RFC7606ActionNone, 0,
		},
		// Corrupted MED first (len=8, padded), valid MED duplicate.
		// First MED is validated → treat-as-withdraw (len != 4).
		{
			"dup/corrupted_first_valid_second_MED",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0x80, 0x04, 0x08, 0x00, 0x00, 0x00, 0x64, 0xff, 0xff, 0xff, 0xff}, // MED len=8 (padded)
				[]byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0xc8},                         // MED=200 (dup, skipped)
			),
			false, RFC7606ActionTreatAsWithdraw, 4,
		},
		// Three ORIGINs: valid, corrupted, corrupted.
		// Only first is validated (passes). Second and third skipped.
		{
			"dup/triple_ORIGIN_valid_first",
			join(
				bOrigin, // valid ORIGIN
				bASPath, bNextHop,
				[]byte{0x40, 0x01, 0x05, 0x00, 0xff, 0xff, 0xff, 0xff}, // ORIGIN len=5 (dup 1)
				[]byte{0x40, 0x01, 0x03, 0x00, 0xff, 0xff},             // ORIGIN len=3 (dup 2)
			),
			false, RFC7606ActionNone, 0,
		},
		// Duplicate ORIGIN + duplicate MED in same UPDATE.
		// Both valid firsts, both corrupted duplicates skipped.
		{
			"dup/mixed_ORIGIN_and_MED_duplicates",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64},                         // MED=100 (valid)
				[]byte{0x40, 0x01, 0x05, 0x00, 0xff, 0xff, 0xff, 0xff},                   // ORIGIN len=5 (dup)
				[]byte{0x80, 0x04, 0x08, 0x00, 0x00, 0x00, 0xc8, 0xff, 0xff, 0xff, 0xff}, // MED len=8 (dup)
			),
			false, RFC7606ActionNone, 0,
		},
		// Duplicate MP_REACH_NLRI: always session-reset per RFC 7606 3.g,
		// even when both copies are valid. AttrCode is 0 because this is
		// a structural multiplicity error, not a per-attribute validation.
		{
			"dup/MP_REACH_duplicate_session_reset",
			join(
				bOrigin, bASPath,
				[]byte{0x80, 0x0e, 0x09, // MP_REACH, len=9
					0x00, 0x01, 0x01, // AFI=1, SAFI=1
					0x04, 0xc0, 0x00, 0x02, 0x01, // NH_LEN=4, 192.0.2.1
					0x00}, // Reserved
				[]byte{0x80, 0x0e, 0x09, // MP_REACH duplicate
					0x00, 0x01, 0x01,
					0x04, 0xc0, 0x00, 0x02, 0x02,
					0x00},
			),
			false, RFC7606ActionSessionReset, 0,
		},
	}

	for _, tt := range dupTests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateUpdateRFC7606(tt.pathAttr, true, tt.isIBGP, false)
			require.GreaterOrEqual(t, int(result.Action), int(tt.wantMin),
				"want at least %s, got %s: %s", tt.wantMin, result.Action, result.Description)
			if tt.wantCode != 0 {
				require.Equal(t, tt.wantCode, result.AttrCode)
			}
		})
	}

	// ==================================================================
	// Gap 1: Extended-length attributes (flags & 0x10).
	//
	// The parser has a separate branch for 2-byte lengths (rfc7606.go
	// lines 191-199). Different arithmetic, different overflow boundary.
	// All previous tests used 1-byte length encoding.
	//
	// Extended-length flag: 0x10. COMMUNITY flags normally 0xC0
	// (optional + transitive), so extended-length COMMUNITY = 0xD0.
	// ==================================================================
	extLenTests := []struct {
		name     string
		pathAttr []byte
		isIBGP   bool
		wantMin  RFC7606Action
		wantCode uint8
	}{
		// Baseline: extended-length COMMUNITY with valid 2-byte length 0x0004.
		{
			"extlen/COMMUNITY_valid_baseline",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0xd0, 0x08, 0x00, 0x04, 0xfd, 0xe8, 0x00, 0x64},
			),
			false, RFC7606ActionNone, 0,
		},
		// 2-byte length inflated to 0x0008: exceeds remaining buffer
		// (only 4 bytes of value present) → structural error.
		{
			"extlen/COMMUNITY_inflate_to_8",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0xd0, 0x08, 0x00, 0x08, 0xfd, 0xe8, 0x00, 0x64},
			),
			false, RFC7606ActionTreatAsWithdraw, 8,
		},
		// 2-byte length deflated to 0x0002: per-attribute validation
		// catches COMMUNITY length 2 (not multiple of 4). Trailing bytes
		// are then parsed as a garbage attribute, causing a structural
		// error that returns immediately — AttrCode may not be 8.
		{
			"extlen/COMMUNITY_deflate_to_2",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0xd0, 0x08, 0x00, 0x02, 0xfd, 0xe8, 0x00, 0x64},
			),
			false, RFC7606ActionTreatAsWithdraw, 0,
		},
		// 2-byte length inflated to 0x0100 (256): far exceeds buffer.
		{
			"extlen/COMMUNITY_inflate_to_256",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0xd0, 0x08, 0x01, 0x00, 0xfd, 0xe8, 0x00, 0x64},
			),
			false, RFC7606ActionTreatAsWithdraw, 8,
		},
		// 2-byte length zeroed to 0x0000: COMMUNITY length 0 → invalid.
		// Trailing 4 bytes parsed as garbage with extended-length flag
		// (0xFD & 0x10 != 0), causing a structural error that returns
		// immediately with the garbage attribute's code.
		{
			"extlen/COMMUNITY_deflate_to_0",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0xd0, 0x08, 0x00, 0x00, 0xfd, 0xe8, 0x00, 0x64},
			),
			false, RFC7606ActionTreatAsWithdraw, 0,
		},
		// Extended-length truncated: flags+code present but only 1 byte
		// remaining for the 2-byte length field.
		{
			"extlen/truncated_length_field",
			join(
				bOrigin, bASPath, bNextHop,
				[]byte{0xd0, 0x08, 0x00}, // Only 1 of 2 length bytes
			),
			false, RFC7606ActionTreatAsWithdraw, 0,
		},
	}

	for _, tt := range extLenTests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateUpdateRFC7606(tt.pathAttr, true, tt.isIBGP, false)
			require.GreaterOrEqual(t, int(result.Action), int(tt.wantMin),
				"want at least %s, got %s: %s", tt.wantMin, result.Action, result.Description)
			if tt.wantCode != 0 {
				require.Equal(t, tt.wantCode, result.AttrCode)
			}
		})
	}

	// ==================================================================
	// Gap 2: asn4=true corruption.
	//
	// AS_PATH segment validation uses asSize=4 instead of 2. This
	// changes overrun/underrun thresholds: segDataLen = count * 4
	// instead of count * 2. AGGREGATOR expects length 8 not 6.
	// ==================================================================
	bASPath4 := []byte{0x40, 0x02, 0x06, // AS_PATH, len=6
		0x02, 0x01, // AS_SEQUENCE, count=1
		0x00, 0x00, 0xfd, 0xe9} // AS 65001 (4 bytes)
	bAggregator4 := []byte{0xc0, 0x07, 0x08, // AGGREGATOR, len=8
		0x00, 0x00, 0xfd, 0xe9, // AS 65001 (4 bytes)
		0xc0, 0x00, 0x02, 0x01} // Router ID 192.0.2.1

	asn4Tests := []struct {
		name     string
		pathAttr []byte
		wantMin  RFC7606Action
		wantCode uint8
	}{
		// Baseline: valid with asn4=true.
		{
			"asn4/baseline_valid",
			join(bOrigin, bASPath4, bNextHop, bAggregator4),
			RFC7606ActionNone, 0,
		},
		// AS_PATH deflate to 5: segDataLen = 1*4 = 4, but after header
		// only 3 bytes remain (5 - 2 header = 3) → overrun.
		// With asn4=false this would be underrun (2 bytes per ASN fits).
		// Trailing byte causes cascading structural error → AttrCode lost.
		{
			"asn4/AS_PATH_deflate_to_5",
			join(
				bOrigin,
				[]byte{0x40, 0x02, 0x05, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9},
				bNextHop,
			),
			RFC7606ActionTreatAsWithdraw, 0,
		},
		// AS_PATH inflate+1 (len=7): steals 1 byte from NEXT_HOP header,
		// after segment (header=2 + 1*4=6), 1 trailing byte → underrun.
		// Cascading misparse of remaining bytes → AttrCode from garbage.
		{
			"asn4/AS_PATH_inflate_to_7",
			join(
				bOrigin,
				[]byte{0x40, 0x02, 0x07, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9},
				bNextHop, bAggregator4,
			),
			RFC7606ActionTreatAsWithdraw, 0,
		},
		// AGGREGATOR deflate to 6 (asn4=true expects 8) → attribute-discard.
		// Trailing 2 bytes cause structural error → escalates action.
		// AttrCode from garbage, not from AGGREGATOR.
		{
			"asn4/AGGREGATOR_deflate_to_6",
			join(
				bOrigin, bASPath4, bNextHop,
				[]byte{0xc0, 0x07, 0x06,
					0x00, 0x00, 0xfd, 0xe9,
					0xc0, 0x00, 0x02, 0x01},
			),
			RFC7606ActionAttributeDiscard, 0,
		},
		// AGGREGATOR inflate to 10 (asn4=true expects 8, padded garbage).
		{
			"asn4/AGGREGATOR_inflate_to_10_padded",
			join(
				bOrigin, bASPath4, bNextHop,
				[]byte{0xc0, 0x07, 0x0a,
					0x00, 0x00, 0xfd, 0xe9,
					0xc0, 0x00, 0x02, 0x01,
					0xff, 0xff},
			),
			RFC7606ActionAttributeDiscard, 7,
		},
	}

	for _, tt := range asn4Tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateUpdateRFC7606(tt.pathAttr, true, false, true)
			require.GreaterOrEqual(t, int(result.Action), int(tt.wantMin),
				"want at least %s, got %s: %s", tt.wantMin, result.Action, result.Description)
			if tt.wantCode != 0 {
				require.Equal(t, tt.wantCode, result.AttrCode)
			}
		})
	}

	// ==================================================================
	// Gap 3: Duplicate MP_UNREACH_NLRI.
	//
	// rfc7606.go:269 checks mpUnreachCount > 1 → session-reset.
	// Only MP_REACH duplicate was tested; MP_UNREACH was not.
	// ==================================================================
	t.Run("dup/MP_UNREACH_duplicate_session_reset", func(t *testing.T) {
		pathAttr := join(
			bOrigin, bASPath, bNextHop,
			[]byte{0x80, 0x0f, 0x03, // MP_UNREACH, code=15, len=3
				0x00, 0x01, 0x01}, // AFI=1, SAFI=1
			[]byte{0x80, 0x0f, 0x03, // MP_UNREACH duplicate
				0x00, 0x01, 0x01},
		)
		result := ValidateUpdateRFC7606(pathAttr, true, false, false)
		require.Equal(t, RFC7606ActionSessionReset, result.Action)
		require.Contains(t, result.Description, "3.g")
	})

	// ==================================================================
	// Gap 4: hasNLRI=false escalation (RFC 7606 Section 5.2).
	//
	// When an UPDATE has path attributes but NO NLRI (hasNLRI=false,
	// mpReachCount=0), errors stronger than attribute-discard are
	// escalated to session-reset. Same corruption → different outcome.
	// ==================================================================
	t.Run("no_nlri/treat_as_withdraw_escalates_to_session_reset", func(t *testing.T) {
		// ORIGIN with corrupted length (5 instead of 1) → treat-as-withdraw.
		// But hasNLRI=false → Section 5.2 escalates to session-reset.
		pathAttr := join(
			[]byte{0x40, 0x01, 0x05, 0x00, 0xff, 0xff, 0xff, 0xff}, // ORIGIN len=5 (padded)
			bASPath, bNextHop,
		)
		result := ValidateUpdateRFC7606(pathAttr, false, false, false)
		require.Equal(t, RFC7606ActionSessionReset, result.Action)
		require.Contains(t, result.Description, "5.2")
	})

	t.Run("no_nlri/attribute_discard_NOT_escalated", func(t *testing.T) {
		// ATOMIC_AGGREGATE with corrupted length (4 instead of 0) →
		// attribute-discard. hasNLRI=false but attribute-discard is NOT
		// escalated per Section 5.2 (only stronger actions escalate).
		pathAttr := join(
			bOrigin, bASPath, bNextHop,
			[]byte{0x40, 0x06, 0x04, 0xff, 0xff, 0xff, 0xff}, // ATOMIC_AGG len=4 (padded)
		)
		result := ValidateUpdateRFC7606(pathAttr, false, false, false)
		require.Equal(t, RFC7606ActionAttributeDiscard, result.Action)
	})

	// ==================================================================
	// Gap 5: Missing mandatory attributes with MP_REACH_NLRI.
	//
	// rfc7606.go:304-318 checks for missing ORIGIN/AS_PATH when
	// mpReachCount > 0. Existing tests only cover the hasNLRI=true
	// path (lines 279-301). This tests the MP_REACH path.
	// ==================================================================
	bMPReach := []byte{0x80, 0x0e, 0x09, // MP_REACH, code=14, len=9
		0x00, 0x01, 0x01, // AFI=1, SAFI=1
		0x04, 0xc0, 0x00, 0x02, 0x01, // NH_LEN=4, 192.0.2.1
		0x00} // Reserved

	t.Run("mp_reach/missing_ORIGIN", func(t *testing.T) {
		// MP_REACH present, AS_PATH present, but no ORIGIN.
		pathAttr := join(bASPath, bMPReach)
		result := ValidateUpdateRFC7606(pathAttr, false, false, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
		require.Equal(t, uint8(1), result.AttrCode) // ORIGIN code
		require.Contains(t, result.Description, "ORIGIN")
	})

	t.Run("mp_reach/missing_AS_PATH", func(t *testing.T) {
		// MP_REACH present, ORIGIN present, but no AS_PATH.
		pathAttr := join(bOrigin, bMPReach)
		result := ValidateUpdateRFC7606(pathAttr, false, false, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
		require.Equal(t, uint8(2), result.AttrCode) // AS_PATH code
		require.Contains(t, result.Description, "AS_PATH")
	})

	t.Run("mp_reach/missing_both_ORIGIN_and_AS_PATH", func(t *testing.T) {
		// MP_REACH present but both ORIGIN and AS_PATH missing.
		// Two errors collected; strongest is treat-as-withdraw.
		pathAttr := join(bMPReach)
		result := ValidateUpdateRFC7606(pathAttr, false, false, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	})

	// ==================================================================
	// Gap 6: Flag error + length error on same attribute.
	//
	// validateAttributeFlags (line 226) runs BEFORE validateAttribute
	// (line 240). If flags are wrong, the flag error is recorded and
	// the function continues to the next attribute — length validation
	// is NEVER reached. This tests that interaction.
	// ==================================================================
	t.Run("flags_and_length/flag_error_preempts_length_check", func(t *testing.T) {
		// ORIGIN with wrong flags (0x80 = optional bit set, should be 0x40)
		// AND wrong length (5 instead of 1).
		// Flag validation fires first → treat-as-withdraw for "optional".
		// Length validation is never reached.
		pathAttr := join(
			[]byte{0x80, 0x01, 0x05, 0x00, 0xff, 0xff, 0xff, 0xff}, // ORIGIN: flags=optional, len=5
			bASPath, bNextHop,
		)
		result := ValidateUpdateRFC7606(pathAttr, true, false, false)
		require.GreaterOrEqual(t, int(result.Action), int(RFC7606ActionTreatAsWithdraw))
		require.Contains(t, result.Description, "3.c") // Flag error, not length
	})

	t.Run("flags_and_length/non_transitive_preempts_length_check", func(t *testing.T) {
		// ORIGIN with flags=0x00 (not transitive, not optional) AND
		// wrong length (0). Flag error (not transitive) fires first.
		pathAttr := join(
			[]byte{0x00, 0x01, 0x00}, // ORIGIN: flags=0x00, len=0
			bASPath, bNextHop,
		)
		result := ValidateUpdateRFC7606(pathAttr, true, false, false)
		require.GreaterOrEqual(t, int(result.Action), int(RFC7606ActionTreatAsWithdraw))
		require.Contains(t, result.Description, "3.c")        // Flag error
		require.Contains(t, result.Description, "transitive") // Specifically "not transitive"
	})
}

// TestRFC7606ASPath4ByteASNOverrun verifies AS_PATH overrun with 4-byte ASNs.
//
// VALIDATES: AS_PATH segment overrun detected with asn4=true.
// PREVENTS: Buffer overflow from malformed AS_PATH with 4-byte ASNs.
func TestRFC7606ASPath4ByteASNOverrun(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		// AS_PATH claims 3 ASes but only room for 1 (4 bytes)
		0x40, 0x02, 0x06, // AS_PATH, len=6
		0x02, 0x03, // AS_SEQUENCE, 3 ASes (would need 12 bytes)
		0x00, 0x00, 0xfd, 0xe9, // AS 65001 (only 1 AS fits)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP
	}

	// asn4=true - should detect overrun
	result := ValidateUpdateRFC7606(pathAttrs, true, false, true)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
	require.Equal(t, uint8(2), result.AttrCode)
	require.Contains(t, result.Description, "overrun")
}
