// Design: internal/component/l2tp/reliable.go -- makeRecvEntry, which reads the Message Type AVP
// Related: internal/component/l2tp/tunnel_fsm.go -- handleMessage, which clears on a malformed one
// Related: plan/journal/silent-fall-through.md -- the row this file closes

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// controlBody wraps a control message body in the header a peer of the dialed
// tunnel sends: the tunnel's own local id, Ns 0 and Nr 1, which is what
// dialedTunnel's engine expects next.
func controlBody(body []byte) []byte {
	const ourLocalTID uint16 = 100
	pkt := make([]byte, ControlHeaderLen+len(body))
	WriteControlHeader(pkt, 0, uint16(ControlHeaderLen+len(body)), ourLocalTID, 0, 0, 1)
	copy(pkt[ControlHeaderLen:], body)
	return pkt
}

// avpBody serializes one AVP into a fresh body.
func avpBody(mandatory bool, attr AVPType, value []byte) []byte {
	body := make([]byte, 64)
	n := WriteAVPBytes(body, 0, mandatory, 0, attr, value)
	return body[:n]
}

// VALIDATES: a control message whose body does not open with a well-formed
//
//	Message Type AVP clears the control connection with a StopCCN, in each of
//	the three ways the body can fail to carry one.
//
// PREVENTS: the dispatch this replaced. makeRecvEntry read two octets at a
//
//	fixed offset, so a first AVP that was some other attribute routed the
//	message on whatever those two octets held, and a body too short to hold an
//	AVP was dispatched as message type 0 and dropped in silence.
//
// RFC 2661 Section 4.4.1: "The Message Type AVP MUST be the first AVP in a
// message, immediately following the control message header". Section 7.1:
// "Examples of a malformed control message include one that has an invalid
// value in its header, contains an AVP that is formatted incorrectly or whose
// value is out of range, or a message that is missing a required AVP", and
// "Receipt of an invalid or unrecoverable malformed control message should be
// logged appropriately and the control connection cleared to ensure recovery
// to a known state".
func TestMalformedControlMessageClearsTheControlConnection(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{"first AVP is not the Message Type", avpBody(true, AVPProtocolVersion, []byte{0x01, 0x00})},
		{"body is shorter than one AVP header", []byte{0x00, 0x08, 0x00, 0x00}},
		{"Message Type AVP value is not 2 octets", avpBody(true, AVPMessageType, []byte{0x00, 0x00, 0x00, 0x01})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			tun, defaults := dialedTunnel(t, now)

			pkt := controlBody(tc.body)
			hdr, err := ParseMessageHeader(pkt)
			require.NoError(t, err)
			out := tun.Process(hdr, pkt[hdr.PayloadOff:hdr.Length], now, defaults, nil)

			require.Equal(t, L2TPTunnelClosed, tun.state,
				"a malformed control message clears the control connection")
			require.Len(t, out, 1, "the clearing emits one StopCCN")

			shdr, perr := ParseMessageHeader(out[0].bytes)
			require.NoError(t, perr)
			info, serr := parseStopCCN(out[0].bytes[shdr.PayloadOff:shdr.Length])
			require.NoError(t, serr)
			require.EqualValues(t, resultProtocolError, info.Result, "Result Code 2")
			require.EqualValues(t, errorValueOutOfRange, info.Error, "Error Code 3")
			require.Equal(t, detailNoMessageType, info.Message,
				"the Error Message names what could not be read")
		})
	}
}

// VALIDATES: a Zero-Length Body message is not a malformed one, so the tunnel
//
//	answers it with no StopCCN and stays where it was.
//
// PREVENTS: a malformed test written on the Message Type VALUE rather than on
//
//	the body length. A ZLB carries no body at all, so a dispatcher that read
//	message type 0 as "malformed" would clear the control connection on every
//	acknowledgment the peer sends.
//
// RFC 2661 Section 5.8: "A ZLB message is a control packet with only an L2TP
// header. ZLB messages are used to acknowledge received messages when there
// are no messages waiting to be sent".
func TestZeroLengthBodyIsNotMalformed(t *testing.T) {
	now := time.Now()
	tun, defaults := dialedTunnel(t, now)

	pkt := controlBody(nil)
	hdr, err := ParseMessageHeader(pkt)
	require.NoError(t, err)
	out := tun.Process(hdr, pkt[hdr.PayloadOff:hdr.Length], now, defaults, nil)

	require.Equal(t, L2TPTunnelWaitCtlReply, tun.state, "a ZLB clears nothing")
	require.Empty(t, out, "a ZLB is acknowledged by nothing on the wire")
}
