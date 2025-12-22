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
		// First MP_REACH_NLRI: flags=0x80 (optional), code=14, len=5, AFI=1, SAFI=1, NH_LEN=0, reserved=0
		0x80, 0x0e, 0x05, 0x00, 0x01, 0x01, 0x00, 0x00,
		// Second MP_REACH_NLRI (triggers error)
		0x80, 0x0e, 0x05, 0x00, 0x02, 0x01, 0x00, 0x00,
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
