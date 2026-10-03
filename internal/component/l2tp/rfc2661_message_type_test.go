// Design: docs/architecture/wire/l2tp.md -- RFC 2661 conformance coverage
//
// Proves RFC 2661 Section 4.4.1 on a Message Type ze does not implement: with
// the M-bit set the tunnel is cleared with a StopCCN, and with the M-bit clear
// the message is ignored and the tunnel stays where it was.

package l2tp

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// messageTypeBody builds a body holding one Message Type AVP of the given
// value, with the M-bit as asked.
func messageTypeBody(mandatory bool, msgType uint16) []byte {
	value := make([]byte, 2)
	binary.BigEndian.PutUint16(value, msgType)
	return avpBody(mandatory, AVPMessageType, value)
}

// TestUnknownMessageTypeClearsTunnelWhenMandatory delivers a control message
// whose Message Type ze does not implement to a dialed tunnel, once with the
// M-bit set and once clear, and reads the tunnel state and the wire after
// each.
//
// RFC requirement: RFC2661-4.4.1-1 positive — an unknown Message Type with the M-bit set moves the tunnel to closed and emits one StopCCN with Result Code 2 and Error Code 8.
// RFC requirement: RFC2661-4.4.1-1 negative — the same unknown Message Type with the M-bit clear emits no StopCCN and leaves the tunnel established.
func TestUnknownMessageTypeClearsTunnelWhenMandatory(t *testing.T) {
	const unknownType uint16 = 0x7FFF

	t.Run("M-bit set clears the tunnel", func(t *testing.T) {
		now := time.Now()
		tun, defaults := dialedTunnel(t, now)

		pkt := controlBody(messageTypeBody(true, unknownType))
		hdr, err := ParseMessageHeader(pkt)
		require.NoError(t, err)
		out := tun.Process(hdr, pkt[hdr.PayloadOff:hdr.Length], now, defaults, nil)

		require.Equal(t, L2TPTunnelClosed, tun.state,
			"an unknown Message Type with the M-bit set clears the control connection")
		require.Len(t, out, 1, "the clearing emits one StopCCN")
		shdr, perr := ParseMessageHeader(out[0].bytes)
		require.NoError(t, perr)
		info, serr := parseStopCCN(out[0].bytes[shdr.PayloadOff:shdr.Length])
		require.NoError(t, serr)
		require.EqualValues(t, resultProtocolError, info.Result, "Result Code 2")
		require.EqualValues(t, errorUnknownMandatoryAVP, info.Error, "Error Code 8")
		require.Equal(t, detailUnknownMessageType, info.Message)
	})

	t.Run("M-bit clear is ignored", func(t *testing.T) {
		now := time.Now()
		tun, defaults := dialedTunnel(t, now)
		before := tun.state

		pkt := controlBody(messageTypeBody(false, unknownType))
		hdr, err := ParseMessageHeader(pkt)
		require.NoError(t, err)
		out := tun.Process(hdr, pkt[hdr.PayloadOff:hdr.Length], now, defaults, nil)

		require.Equal(t, before, tun.state, "an optional unknown Message Type changes no state")
		for _, o := range out {
			shdr, perr := ParseMessageHeader(o.bytes)
			require.NoError(t, perr)
			require.NotEqual(t, MsgStopCCN, messageTypeOf(t, o.bytes[shdr.PayloadOff:shdr.Length]),
				"an optional unknown Message Type draws no StopCCN")
		}
	})
}

// messageTypeOf reads the Message Type of a control body, 0 for a ZLB.
func messageTypeOf(t *testing.T, body []byte) MessageType {
	t.Helper()
	if len(body) == 0 {
		return 0
	}
	iter := NewAVPIterator(body)
	_, attr, _, value, ok := iter.Next()
	require.True(t, ok)
	require.Equal(t, AVPMessageType, attr)
	return MessageType(binary.BigEndian.Uint16(value))
}

// TestHiddenMessageTypeClearsTunnel prevents ciphertext from being
// dispatched as a known message or ignored as an optional unknown type.
func TestHiddenMessageTypeClearsTunnel(t *testing.T) {
	for _, msgType := range []uint16{uint16(MsgHello), 0x7FFF} {
		now := time.Now()
		tun, defaults := dialedTunnel(t, now)
		body := messageTypeBody(false, msgType)
		body[0] |= 0x40
		pkt := controlBody(body)
		hdr, err := ParseMessageHeader(pkt)
		require.NoError(t, err)
		out := tun.Process(hdr, pkt[hdr.PayloadOff:hdr.Length], now, defaults, nil)
		require.Equal(t, L2TPTunnelClosed, tun.state)
		require.Len(t, out, 1)
		reply, err := ParseMessageHeader(out[0].bytes)
		require.NoError(t, err)
		info, err := parseStopCCN(out[0].bytes[reply.PayloadOff:reply.Length])
		require.NoError(t, err)
		require.EqualValues(t, resultProtocolError, info.Result)
		require.EqualValues(t, errorValueOutOfRange, info.Error)
	}
}
