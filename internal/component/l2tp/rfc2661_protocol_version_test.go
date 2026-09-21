// Related: tunnel_fsm.go -- handleSCCRQ, which judges the Protocol Version AVP
// Related: reactor_sccrq_mandatory_avp_test.go -- the reactor harness reused here

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// buildSCCRQAtVersion returns a complete SCCRQ whose Protocol Version AVP
// carries ver and rev, so a case differs from an accepted SCCRQ in that one
// value.
func buildSCCRQAtVersion(t *testing.T, ver, rev byte, peerTID uint16, hostName string) []byte {
	t.Helper()
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	buf := *bodyBuf
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, []byte{ver, rev})
	off += WriteAVPBytes(buf, off, true, 0, AVPFramingCapabilities, []byte{0x00, 0x00, 0x00, 0x03})
	off += WriteAVPBytes(buf, off, true, 0, AVPBearerCapabilities, []byte{0x00, 0x00, 0x00, 0x00})
	off += WriteAVPString(buf, off, true, AVPHostName, hostName)
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, peerTID)
	off += WriteAVPUint16(buf, off, true, AVPReceiveWindowSize, 8)

	total := ControlHeaderLen + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), 0, 0, 0, 0)
	copy(pkt[ControlHeaderLen:], buf[:off])
	return pkt
}

// RFC requirement: RFC2661-7.2.1-1 positive -- an SCCRQ announcing a version
// ze does not speak (0.0, 1.1, 3.0) is answered with a StopCCN carrying
// Result Code 5 and the version ze speaks, and no tunnel is allocated.
// VALIDATES: an SCCRQ announcing a protocol version ze does not speak is
//
//	answered with StopCCN Result Code 5 carrying the version ze does speak,
//	and establishes no tunnel.
//
// PREVENTS: the read that never happened. parseSCCRQ stored
// info.ProtocolVersion and handleSCCRQ consulted nothing, so a peer
// announcing any version at all was answered with an SCCRP and given a
// tunnel.
//
// RFC 2661 Section 4.4.3: "The Ver field is a 1 octet unsigned integer
// containing the value 1. Rev field is a 1 octet unsigned integer containing
// 0. This pertains to L2TP protocol version 1, revision 0." RFC 2661 Section
// 4.4.2 reserves the answer: "5 - The protocol version of the requester is not
// supported / Error Code indicates highest version supported".
func TestSCCRQAtAnUnsupportedProtocolVersionIsRefused(t *testing.T) {
	cases := []struct {
		name string
		ver  byte
		rev  byte
	}{
		{"l2tpv3", 0x03, 0x00},
		{"version zero", 0x00, 0x00},
		{"revision ze does not speak", 0x01, 0x01},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clk := newTestClock(time.Unix(1_700_000_000, 0))
			ln, r, _, stop := buildZeroTIDReactor(t, clk)
			defer stop()

			client := newClient(t, ln)
			defer client.Close()

			client.Send(t, buildSCCRQAtVersion(t, tc.ver, tc.rev, 4242, "version-peer"))

			reply, got := recvControl(t, client, replyWait)
			require.True(t, got, "an unsupported protocol version must be answered, not dropped in silence")

			hdr, err := ParseMessageHeader(reply)
			require.NoError(t, err)
			info, err := parseStopCCN(reply[hdr.PayloadOff:hdr.Length])
			require.NoError(t, err)
			require.EqualValues(t, resultVersionUnsupported, info.Result,
				"RFC 2661 Section 4.4.2 reserves Result Code 5 for an unsupported protocol version")
			require.EqualValues(t, protocolVersionSupported, info.Error,
				"the Error Code carries the version ze speaks")

			require.Equal(t, 0, r.TunnelCount(), "the refusal allocates no tunnel")
		})
	}
}

// RFC requirement: RFC2661-7.2.1-1 negative -- an SCCRQ at the supported
// version 1.0 is answered with an SCCRP, not a StopCCN, and a tunnel exists.
// VALIDATES: an SCCRQ at version 1 revision 0 still establishes, so the
//
//	refusal above is keyed on the value and does not refuse every peer.
//
// RFC 2661 Section 4.4.3 fixes that pair as "L2TP protocol version 1,
// revision 0", which is the version every outbound message of ze's own
// writes.
func TestSCCRQAtTheSupportedProtocolVersionEstablishes(t *testing.T) {
	clk := newTestClock(time.Unix(1_700_000_000, 0))
	ln, r, _, stop := buildZeroTIDReactor(t, clk)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	client.Send(t, buildSCCRQAtVersion(t, 0x01, 0x00, 4242, "version-peer"))

	reply, got := recvControl(t, client, replyWait)
	require.True(t, got, "a complete SCCRQ must be answered")

	hdr, err := ParseMessageHeader(reply)
	require.NoError(t, err)
	require.EqualValues(t, MsgSCCRP, extractMsgType(reply[hdr.PayloadOff:hdr.Length]),
		"version 1 revision 0 is the version ze speaks, so it gets SCCRP")
	require.Equal(t, 1, r.TunnelCount())
}
