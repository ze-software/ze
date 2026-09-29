package l2tp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// buildSCCRQBodyWithExtraAVP returns a well-formed SCCRQ body followed by one
// extra IETF AVP of attrType, with the M-bit set as given and, when reserved is
// true, one reserved bit of the AVP flags set.
func buildSCCRQBodyWithExtraAVP(t *testing.T, attrType AVPType, mandatory, reserved bool) []byte {
	t.Helper()
	buf := make([]byte, 256)
	off := 0
	write := func(m bool, attr AVPType, value []byte) {
		off += WriteAVPBytes(buf, off, m, 0, attr, value)
	}
	write(true, AVPMessageType, u16Bytes(uint16(MsgSCCRQ)))
	write(true, AVPProtocolVersion, []byte{0x01, 0x00})
	write(true, AVPFramingCapabilities, []byte{0x00, 0x00, 0x00, 0x03})
	write(true, AVPHostName, []byte("peer"))
	write(true, AVPAssignedTunnelID, u16Bytes(7))
	extra := off
	write(mandatory, attrType, []byte{0xde, 0xad})
	if reserved {
		buf[extra] |= 0x08
	}
	return buf[:off]
}

// TestSCCRQUnrecognizedAVPErrorCode checks the refusal parseSCCRQ picks for
// each cause of an unrecognized AVP with the M-bit set: an IETF Attribute Type
// RFC 2661 does not define gives Error Code 8, and a reserved flag bit gives
// Error Code 3. With the M-bit clear the undefined AVP is ignored.
//
// VALIDATES: parseSCCRQ tells the two causes of FlagUnrecognized apart.
// PREVENTS: an unknown IETF AVP refused as a field out of range (Error Code 3).
func TestSCCRQUnrecognizedAVPErrorCode(t *testing.T) {
	const undefinedIETFType AVPType = 40

	_, err := parseSCCRQ(buildSCCRQBodyWithExtraAVP(t, undefinedIETFType, true, false))
	require.ErrorIs(t, err, errSCCRQUnknownMandatoryAVP, "undefined IETF type with M=1 is Error Code 8")

	_, err = parseSCCRQ(buildSCCRQBodyWithExtraAVP(t, AVPReceiveWindowSize, true, true))
	require.ErrorIs(t, err, errSCCRQMandatoryReservedBits, "reserved bit with M=1 is Error Code 3")

	info, err := parseSCCRQ(buildSCCRQBodyWithExtraAVP(t, undefinedIETFType, false, false))
	require.NoError(t, err, "undefined IETF type with M=0 is ignored")
	require.EqualValues(t, 7, info.AssignedTunnelID)
}
