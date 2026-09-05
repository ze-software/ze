package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// buildSCCRQOmitting returns an SCCRQ datagram carrying every AVP RFC 2661
// Section 6.1 makes mandatory, except omit, which is left out. Everything
// else matches buildSCCRQ (reactor_test.go), so a case differs from an
// accepted SCCRQ in exactly one AVP.
func buildSCCRQOmitting(t *testing.T, omit AVPType, peerTID uint16, hostName string) []byte {
	t.Helper()
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	buf := *bodyBuf
	off := 0
	write := func(attr AVPType, value []byte) {
		if attr == omit {
			return
		}
		off += WriteAVPBytes(buf, off, true, 0, attr, value)
	}
	write(AVPMessageType, u16Bytes(uint16(MsgSCCRQ)))
	write(AVPProtocolVersion, []byte{0x01, 0x00})
	write(AVPFramingCapabilities, []byte{0x00, 0x00, 0x00, 0x03})
	write(AVPBearerCapabilities, []byte{0x00, 0x00, 0x00, 0x00})
	write(AVPHostName, []byte(hostName))
	write(AVPAssignedTunnelID, u16Bytes(peerTID))
	write(AVPReceiveWindowSize, u16Bytes(8))

	total := ControlHeaderLen + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), 0, 0, 0, 0)
	copy(pkt[ControlHeaderLen:], buf[:off])
	return pkt
}

// u16Bytes is the two-octet big-endian form of v, so buildSCCRQOmitting can
// write every AVP through one helper.
func u16Bytes(v uint16) []byte { return []byte{byte(v >> 8), byte(v)} }

// RFC requirement: RFC2661-6.1-1 negative -- an SCCRQ that omits one of the five
// AVPs RFC 2661 Section 6.1 makes mandatory is answered with a StopCCN carrying
// Result Code 2, Error Code 3 and an Error Message naming the AVP, and creates
// no tunnel. One case per AVP, each differing from an accepted SCCRQ in that
// one AVP.
//
// VALIDATES: the reply reaches the peer through the reactor's own UDP socket,
// so the path under test is the one a peer drives.
// PREVENTS: the silent drop this test was written for. Until this change ze
// answered only a zero-valued Assigned Tunnel ID; a missing Framing
// Capabilities or Protocol Version AVP was not detected at all, and the tunnel
// established with FramingCapabilities left at 0.
func TestSCCRQMissingMandatoryAVPIsAnswered(t *testing.T) {
	cases := []struct {
		name   string
		omit   AVPType
		detail string
	}{
		{"message type", AVPMessageType, "SCCRQ Message Type AVP must be the first AVP"},
		{"protocol version", AVPProtocolVersion, "SCCRQ missing Protocol Version AVP"},
		{"host name", AVPHostName, "SCCRQ missing Host Name AVP"},
		{"framing capabilities", AVPFramingCapabilities, "SCCRQ missing Framing Capabilities AVP"},
		{"assigned tunnel id", AVPAssignedTunnelID, "SCCRQ missing Assigned Tunnel ID AVP"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clk := newTestClock(time.Unix(1_700_000_000, 0))
			ln, r, _, stop := buildZeroTIDReactor(t, clk)
			defer stop()

			client := newClient(t, ln)
			defer client.Close()

			client.Send(t, buildSCCRQOmitting(t, tc.omit, 4242, "no-avp-peer"))

			reply, got := recvControl(t, client, replyWait)
			require.True(t, got, "an SCCRQ missing a mandatory AVP must be answered, not dropped in silence")

			hdr, err := ParseMessageHeader(reply)
			require.NoError(t, err)
			require.True(t, hdr.IsControl)
			// RFC 2661 Section 4.4.3: with no Assigned Tunnel ID from the peer,
			// the header MUST carry Tunnel ID 0.
			require.EqualValues(t, 0, hdr.TunnelID)
			require.EqualValues(t, 0, hdr.Ns, "first control message ze sends on this connection")
			require.EqualValues(t, 1, hdr.Nr, "acknowledges the SCCRQ's Ns=0")

			info, err := parseStopCCN(reply[hdr.PayloadOff:hdr.Length])
			require.NoError(t, err)
			require.EqualValues(t, resultProtocolError, info.Result, "Result Code 2")
			require.EqualValues(t, errorValueOutOfRange, info.Error, "Error Code 3")
			require.Equal(t, tc.detail, info.Message, "the Error Message names the AVP that is missing")
			require.EqualValues(t, tidNoTunnel, info.AssignedTunnelID)

			require.Equal(t, 0, r.TunnelCount(), "the refusal allocates no tunnel")
			r.tunnelsMu.Lock()
			peerMapLen := len(r.tunnelsByPeer)
			r.tunnelsMu.Unlock()
			require.Equal(t, 0, peerMapLen)
		})
	}
}

// RFC requirement: RFC2661-6.1-1 positive -- an SCCRQ carrying all five AVPs of
// RFC 2661 Section 6.1 is accepted on the same path: ze answers SCCRP, creates
// the tunnel, and stores the Framing Capabilities mask the peer sent rather
// than a default.
//
// VALIDATES: the refusal is keyed on the absent AVP and nothing else, so the
// negative polarity above cannot pass by refusing every SCCRQ.
// PREVENTS: a presence check that reads the parsed VALUE. 0 is a legal Framing
// Capabilities mask, so a value test would refuse a peer that sends one.
func TestSCCRQWithEveryMandatoryAVPEstablishes(t *testing.T) {
	clk := newTestClock(time.Unix(1_700_000_000, 0))
	ln, r, _, stop := buildZeroTIDReactor(t, clk)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	client.Send(t, buildSCCRQ(t, 4242, "complete-peer"))

	reply, got := recvControl(t, client, replyWait)
	require.True(t, got, "a complete SCCRQ must be answered")

	hdr, err := ParseMessageHeader(reply)
	require.NoError(t, err)
	require.EqualValues(t, 4242, hdr.TunnelID, "the reply is addressed with the peer's Assigned Tunnel ID")
	require.EqualValues(t, MsgSCCRP, extractMsgType(reply[hdr.PayloadOff:hdr.Length]),
		"a complete SCCRQ gets SCCRP, never StopCCN")

	require.Equal(t, 1, r.TunnelCount())
	r.tunnelsMu.Lock()
	var framing uint32
	var hostName string
	for _, tunnel := range r.tunnelsByLocalID {
		framing = tunnel.peerFraming
		hostName = tunnel.peerHostName
	}
	r.tunnelsMu.Unlock()
	require.EqualValues(t, 0x3, framing, "the mask buildSCCRQ sent, read off the AVP rather than defaulted")
	require.Equal(t, "complete-peer", hostName)
}

// RFC requirement: RFC2661-6.1-1 negative -- an SCCRQ whose Framing
// Capabilities AVP carries three octets instead of four is answered with a
// StopCCN naming that AVP, rather than establishing a tunnel on a mask of 0.
//
// VALIDATES: RFC 2661 Section 7.1 treats an AVP "formatted incorrectly" and a
// "message that is missing a required AVP" in one sentence, so ze answers both.
// PREVENTS: the discarded read error this test was written for. parseSCCRQ
// swallowed the length error and left FramingCapabilities at 0, which is a
// legal mask, so the tunnel established on a value no peer sent.
func TestSCCRQWithShortFramingCapabilitiesIsAnswered(t *testing.T) {
	clk := newTestClock(time.Unix(1_700_000_000, 0))
	ln, r, _, stop := buildZeroTIDReactor(t, clk)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	buf := *bodyBuf
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, []byte{0x01, 0x00})
	off += WriteAVPBytes(buf, off, true, 0, AVPFramingCapabilities, []byte{0x00, 0x00, 0x03})
	off += WriteAVPString(buf, off, true, AVPHostName, "short-framing-peer")
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, 4242)
	total := ControlHeaderLen + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), 0, 0, 0, 0)
	copy(pkt[ControlHeaderLen:], buf[:off])
	client.Send(t, pkt)

	reply, got := recvControl(t, client, replyWait)
	require.True(t, got, "a malformed mandatory AVP must be answered, not dropped in silence")

	hdr, err := ParseMessageHeader(reply)
	require.NoError(t, err)
	info, err := parseStopCCN(reply[hdr.PayloadOff:hdr.Length])
	require.NoError(t, err)
	require.EqualValues(t, resultProtocolError, info.Result, "Result Code 2")
	require.EqualValues(t, errorValueOutOfRange, info.Error, "Error Code 3")
	require.Equal(t, "SCCRQ Framing Capabilities AVP must be 4 octets", info.Message)

	require.Equal(t, 0, r.TunnelCount(), "the refusal allocates no tunnel")
}
