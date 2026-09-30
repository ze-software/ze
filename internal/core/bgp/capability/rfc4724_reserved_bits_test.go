// Design: docs/architecture/wire/capabilities.md -- Graceful Restart capability
// RFC: rfc/short/rfc4724.md -- reserved Restart Flags and Address Family Flags bits (Section 3)
//
// RFC 4724 Section 3: "The remaining bits are reserved and MUST be set to zero
// by the sender and ignored by the receiver." The sentence appears twice in
// Section 3: once for the Restart Flags nibble (RFC4724-3-2), once for the
// Flags for Address Family byte (RFC4724-3-3). The sender half is proven by
// TestGracefulRestartEncodeReservedBits. These tests prove the receiver half:
// parseGracefulRestart, the parser every received OPEN reaches through Parse,
// gives the same answer whether a peer set the reserved bits or cleared them.
//
// VALIDATES: reserved Graceful Restart flag bits are ignored on receive.
// PREVENTS: a peer's reserved bit changing Ze's view of restart or forwarding state.

package capability

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// grCapabilityTLV returns a Graceful Restart capability TLV (code 64) whose
// Restart Flags nibble is restartFlags (the high four bits of the first value
// octet), with Restart Time 120 and one IPv4 unicast family whose Address
// Family Flags octet is familyFlags.
func grCapabilityTLV(restartFlags, familyFlags byte) []byte {
	return []byte{
		byte(CodeGracefulRestart), 6,
		restartFlags << 4, 120, // Restart Flags, Restart Time high nibble 0, Restart Time 120.
		0x00, 0x01, 0x01, familyFlags, // AFI 1, SAFI 1, Flags for Address Family.
	}
}

// parseSingleGR parses data and returns its only capability as GracefulRestart.
func parseSingleGR(t *testing.T, data []byte) *GracefulRestart {
	t.Helper()
	caps, err := Parse(data)
	require.NoError(t, err, "a reserved bit is never a reason to refuse the capability")
	require.Len(t, caps, 1)
	gr, ok := caps[0].(*GracefulRestart)
	require.True(t, ok, "code 64 must parse as GracefulRestart, got %T", caps[0])
	return gr
}

// TestRFC4724ReceiverIgnoresReservedRestartFlags proves the receiver half of
// RFC4724-3-2. Method: parse a capability whose reserved Restart Flags bits
// are clear and one whose reserved bits are set, with the R bit both clear and
// set, and require identical results. Only the two low bits of the nibble are
// used: RFC 8538 Section 2 assigned the second bit (N), so it is no longer
// reserved and a receiver that implements RFC 8538 reads it.
//
// RFC requirement: RFC4724-3-2 positive -- a received Restart Flags nibble with the reserved bits clear parses with no error to the R bit and Restart Time 120 it carries, for R clear and R set.
// RFC requirement: RFC4724-3-2 negative -- a received Restart Flags nibble whose sender set the reserved bits (0x3, the bits RFC 8538 left reserved) parses with no error to exactly the same R bit, Restart Time and family as the clean nibble, so the reserved bits are ignored.
func TestRFC4724ReceiverIgnoresReservedRestartFlags(t *testing.T) {
	t.Parallel()

	for _, restartBit := range []byte{0x0, 0x8} {
		clean := parseSingleGR(t, grCapabilityTLV(restartBit, 0x80))
		require.Equal(t, restartBit == 0x8, clean.RestartState, "R bit")
		require.Equal(t, uint16(120), clean.RestartTime, "Restart Time")

		dirty := parseSingleGR(t, grCapabilityTLV(restartBit|0x3, 0x80))
		require.Equal(t, clean, dirty, "reserved Restart Flags bits must not change the parsed capability (R=%x)", restartBit)
	}
}

// TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags proves the receiver half
// of RFC4724-3-3. Method: parse a capability whose Flags for Address Family
// octet has the seven reserved bits clear and one where they are set, with the
// F bit both clear and set, and require identical results.
//
// RFC requirement: RFC4724-3-3 positive -- a received Flags for Address Family octet with the reserved bits clear parses with no error to the F bit it carries (0x00 -> false, 0x80 -> true) for AFI 1 SAFI 1.
// RFC requirement: RFC4724-3-3 negative -- a received Flags for Address Family octet whose sender set all seven reserved bits (0x7F) parses with no error to exactly the same AFI, SAFI and F bit as the clean octet, so the reserved bits are ignored.
func TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags(t *testing.T) {
	t.Parallel()

	for _, forwardingBit := range []byte{0x00, 0x80} {
		clean := parseSingleGR(t, grCapabilityTLV(0x8, forwardingBit))
		require.Len(t, clean.Families, 1)
		require.Equal(t, AFIIPv4, clean.Families[0].AFI)
		require.Equal(t, SAFIUnicast, clean.Families[0].SAFI)
		require.Equal(t, forwardingBit == 0x80, clean.Families[0].ForwardingState, "F bit")

		dirty := parseSingleGR(t, grCapabilityTLV(0x8, forwardingBit|0x7F))
		require.Equal(t, clean, dirty, "reserved Address Family Flags bits must not change the parsed capability (F=%x)", forwardingBit)
	}
}
