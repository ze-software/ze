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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// TestRFC7606EmptyWithdrawal verifies withdraw-only UPDATE is valid.
func TestRFC7606EmptyWithdrawal(t *testing.T) {
	// No path attributes, no NLRI = withdrawal only (valid)
	result := ValidateUpdateRFC7606(nil, false)
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

	result := ValidateUpdateRFC7606(pathAttrs, false)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, true)
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

	result := ValidateUpdateRFC7606(pathAttrs, false)
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

	result := ValidateUpdateRFC7606(pathAttrs, false)
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

	result := ValidateUpdateRFC7606(pathAttrs, false)
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

	result := ValidateUpdateRFC7606(pathAttrs, false)
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

	result := ValidateUpdateRFC7606(pathAttrs, false)
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

	result := ValidateUpdateRFC7606(pathAttrs, false)
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

	result := ValidateUpdateRFC7606(pathAttrs, false)
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

	result := ValidateUpdateRFC7606(pathAttrs, false)
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

// TestRFC7606NLRIOverrun verifies RFC 7606 Section 5.3.
//
// VALIDATES: NLRI with prefix bytes exceeding field triggers treat-as-withdraw.
// PREVENTS: Buffer overflow from malformed NLRI.
func TestRFC7606NLRIOverrun(t *testing.T) {
	// Invalid NLRI: claims /24 but only 2 bytes follow (needs 3)
	nlri := []byte{
		24, 192, 168, // Missing last byte
	}

	result := ValidateNLRISyntax(nlri, false)
	require.NotNil(t, result)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)
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
