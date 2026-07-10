package l2tp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// --- Session-message encoder / parser round-trips (AC-3, AC-4 codecs) ---

func TestWriteICRQBody_RoundTrip(t *testing.T) {
	// VALIDATES: writeICRQBody produces a body the existing parseICRQ
	// accepts, with the required + optional AVPs surviving.
	buf := make([]byte, 512)
	n := writeICRQBody(buf, 0x0501, 12345, 6, "5551000", "5552000")
	info, err := parseICRQ(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 0x0501, info.assignedSessionID)
	require.EqualValues(t, 12345, info.callSerialNumber)
	require.EqualValues(t, 6, info.bearerType)
	require.Equal(t, "5551000", info.calledNumber)
	require.Equal(t, "5552000", info.callingNumber)
}

func TestWriteICRQBody_MinimalRoundTrip(t *testing.T) {
	// VALIDATES: omitting optional called/calling numbers still parses.
	buf := make([]byte, 256)
	n := writeICRQBody(buf, 7, 1, 0, "", "")
	info, err := parseICRQ(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 7, info.assignedSessionID)
	require.EqualValues(t, 1, info.callSerialNumber)
}

func TestWriteICCNBody_RoundTrip(t *testing.T) {
	// VALIDATES: writeICCNBody round-trips through parseICCN.
	buf := make([]byte, 256)
	n := writeICCNBody(buf, 64000, 0x1)
	info, err := parseICCN(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 64000, info.txConnectSpeed)
	require.EqualValues(t, 0x1, info.framingType)
}

func TestWriteOCRQBody_RoundTrip(t *testing.T) {
	// VALIDATES: writeOCRQBody round-trips through parseOCRQ with all the
	// RFC 2661 S7.9 required AVPs present.
	buf := make([]byte, 512)
	n := writeOCRQBody(buf, 0x0A0B, 999, 9600, 128000, 1, 0x1, "5559999")
	info, err := parseOCRQ(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 0x0A0B, info.assignedSessionID)
	require.EqualValues(t, 999, info.callSerialNumber)
	require.EqualValues(t, 9600, info.minimumBPS)
	require.EqualValues(t, 128000, info.maximumBPS)
	require.EqualValues(t, 1, info.bearerType)
	require.EqualValues(t, 0x1, info.framingType)
	require.Equal(t, "5559999", info.calledNumber)
}

func TestParseICRP_RoundTrip(t *testing.T) {
	// VALIDATES: parseICRP reads back a body from the existing writeICRPBody
	// encoder and captures the Assigned Session ID.
	buf := make([]byte, 64)
	n := writeICRPBody(buf, 0x4242)
	info, err := parseICRP(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 0x4242, info.assignedSessionID)
}

func TestParseICRP_Rejects(t *testing.T) {
	// VALIDATES: parseICRP rejects a missing / zero Assigned Session ID.
	_, err := parseICRP(nil)
	require.Error(t, err)

	buf := make([]byte, 32)
	n := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgICRP))
	_, err = parseICRP(buf[:n])
	require.Error(t, err)

	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgICRP))
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, 0)
	_, err = parseICRP(buf[:off])
	require.Error(t, err)
}

func TestParseOCRP_RoundTrip(t *testing.T) {
	// VALIDATES: parseOCRP reads back a body from writeOCRPBody.
	buf := make([]byte, 64)
	n := writeOCRPBody(buf, 0x9001)
	info, err := parseOCRP(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 0x9001, info.assignedSessionID)
}

func TestParseOCRP_Rejects(t *testing.T) {
	_, err := parseOCRP(nil)
	require.Error(t, err)

	buf := make([]byte, 32)
	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgOCRP))
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, 0)
	_, err = parseOCRP(buf[:off])
	require.Error(t, err)
}
