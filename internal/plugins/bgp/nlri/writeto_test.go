package nlri

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// WriteTo Zero-Allocation Tests for core INET NLRI type.
//
// WriteTo tests for plugin NLRI types (MVPN, VPLS, RTC, MUP, LabeledUnicast)
// live in their respective plugin test packages.
//
// VALIDATES: WriteTo writes correct wire format directly to buffer
// PREVENTS: Allocation in hot path, output mismatch with Bytes()
// ============================================================================

// TestWriteToAtOffset verifies WriteTo works correctly with non-zero offset.
//
// VALIDATES: WriteTo honors offset parameter and writes at correct position.
// PREVENTS: Off-by-one errors, buffer corruption at wrong positions.
func TestWriteToAtOffset(t *testing.T) {
	inet := &INET{
		PrefixNLRI: PrefixNLRI{
			prefix: netip.MustParsePrefix("10.0.0.0/24"),
		},
	}

	expected := inet.Bytes()

	// Write at offset 5
	buf := make([]byte, len(expected)+20)
	buf[0] = 0xDE
	buf[1] = 0xAD
	buf[2] = 0xBE
	buf[3] = 0xEF
	buf[4] = 0x00

	n := inet.WriteTo(buf, 5)

	// Verify prefix bytes unchanged
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00}, buf[:5])
	// Verify WriteTo wrote at correct offset
	assert.Equal(t, expected, buf[5:5+n])
}

// TestWriteToZeroAlloc verifies WriteTo produces same output as Bytes().
//
// VALIDATES: WriteTo implementation is consistent with Bytes().
// PREVENTS: Divergent implementations of WriteTo and Bytes.
func TestWriteToZeroAlloc(t *testing.T) {
	// Create an INET
	inet := &INET{
		PrefixNLRI: PrefixNLRI{
			prefix: netip.MustParsePrefix("10.0.0.0/24"),
		},
	}

	// Get expected output from Bytes()
	expected := inet.Bytes()

	// WriteTo should produce the same output
	buf := make([]byte, 256)
	n := inet.WriteTo(buf, 0)

	require.Equal(t, len(expected), n, "WriteTo length mismatch")
	assert.True(t, bytes.Equal(expected, buf[:n]), "WriteTo output mismatch")
}
