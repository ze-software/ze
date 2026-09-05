package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// avpNone names no AVP, so buildSCCRPWithout writes the complete set. RFC 2661
// Section 4.1 draws the Attribute Type as a 2-octet field and Section 4.4 uses
// 0 for Message Type, so a sentinel has to sit outside the assigned range
// rather than at zero.
const avpNone AVPType = 0xffff

// buildSCCRPWithout assembles an SCCRP datagram carrying every AVP RFC 2661
// Section 6.2 makes mandatory except omit, which is left out. It writes the
// body here rather than through writeSCCRPBody (tunnel_fsm.go) because that
// encoder is the one ze uses on the wire and always emits the whole set.
func buildSCCRPWithout(t *testing.T, omit AVPType, ourLocalTID, peerAssignedTID uint16) []byte {
	t.Helper()
	body := make([]byte, 512)
	off := 0
	write := func(attr AVPType, value []byte) {
		if attr == omit {
			return
		}
		off += WriteAVPBytes(body, off, true, 0, attr, value)
	}
	write(AVPMessageType, u16Bytes(uint16(MsgSCCRP)))
	write(AVPProtocolVersion, []byte{0x01, 0x00})
	write(AVPFramingCapabilities, []byte{0x00, 0x00, 0x00, 0x03})
	write(AVPHostName, []byte("peer-lns"))
	write(AVPAssignedTunnelID, u16Bytes(peerAssignedTID))
	write(AVPReceiveWindowSize, u16Bytes(8))

	pkt := make([]byte, ControlHeaderLen+off)
	WriteControlHeader(pkt, 0, uint16(ControlHeaderLen+off), ourLocalTID, 0, 0, 1)
	copy(pkt[ControlHeaderLen:], body[:off])
	return pkt
}

// dialedTunnel returns a tunnel that has sent its SCCRQ and is waiting for the
// peer's SCCRP, which is the state RFC 2661 Section 7.2.1 calls wait-ctl-reply.
func dialedTunnel(t *testing.T, now time.Time) (*L2TPTunnel, TunnelDefaults) {
	t.Helper()
	defaults := initiatorDefaults("")
	tun := newTunnel(100, 0, netip.MustParseAddrPort("10.0.0.2:1701"),
		ReliableConfig{RecvWindow: 8}, slog.Default(), now)
	out := tun.initiate(now, defaults, nil)
	require.Len(t, out, 1, "the dial emits one SCCRQ")
	require.Equal(t, L2TPTunnelWaitCtlReply, tun.state)
	return tun, defaults
}

// RFC requirement: RFC2661-6.2-1 negative -- an SCCRP that omits the Protocol
// Version, the Framing Capabilities, the Host Name or the Assigned Tunnel ID
// AVP tears the dialed tunnel down with a StopCCN instead of establishing it.
// One case per AVP, each differing from an accepted SCCRP in that one AVP.
//
// The fifth AVP of RFC 2661 Section 6.2, the Message Type, is not in the table:
// the tunnel dispatcher routes a delivered control message by the value of its
// FIRST AVP, so a body carrying no Message Type AVP never reaches parseSCCRP.
// Ze drops it, which is a gap against the Section 7.1 SHOULD, recorded in
// plan/journal/silent-fall-through.md.
//
// VALIDATES: the refusal reaches the wire as a StopCCN, and the tunnel leaves
// wait-ctl-reply for closed rather than established.
// PREVENTS: the establishment this test was written for. parseSCCRP checked
// only the Assigned Tunnel ID, discarded the Framing Capabilities read error,
// and left the mask at 0, which is a legal value no peer sent.
func TestSCCRPMissingMandatoryAVPTearsTheTunnelDown(t *testing.T) {
	cases := []struct {
		name string
		omit AVPType
	}{
		{"protocol version", AVPProtocolVersion},
		{"framing capabilities", AVPFramingCapabilities},
		{"host name", AVPHostName},
		{"assigned tunnel id", AVPAssignedTunnelID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			tun, defaults := dialedTunnel(t, now)

			pkt := buildSCCRPWithout(t, tc.omit, 100, 555)
			hdr, err := ParseMessageHeader(pkt)
			require.NoError(t, err)
			out := tun.Process(hdr, pkt[hdr.PayloadOff:hdr.Length], now, defaults, nil)

			require.Equal(t, L2TPTunnelClosed, tun.state, "an unacceptable SCCRP closes the tunnel")
			require.Len(t, out, 1, "the refusal emits one StopCCN")

			shdr, err := ParseMessageHeader(out[0].bytes)
			require.NoError(t, err)
			info, perr := parseStopCCN(out[0].bytes[shdr.PayloadOff:shdr.Length])
			require.NoError(t, perr)
			require.EqualValues(t, resultGeneralError, info.Result,
				"RFC 2661 Section 4.4.2 Result Code 1: general request to clear the control connection")
		})
	}
}

// RFC requirement: RFC2661-6.2-1 positive -- an SCCRP carrying all five AVPs of
// RFC 2661 Section 6.2 is accepted on the same path: the tunnel establishes,
// adopts the peer's Assigned Tunnel ID, and stores the Framing Capabilities
// mask the peer sent rather than a default.
//
// VALIDATES: the refusal is keyed on the absent AVP and nothing else, so the
// negative polarity above cannot pass by refusing every SCCRP.
// PREVENTS: a presence check that reads the parsed VALUE, which would refuse a
// peer whose framing mask is legitimately 0.
func TestSCCRPWithEveryMandatoryAVPEstablishes(t *testing.T) {
	now := time.Now()
	tun, defaults := dialedTunnel(t, now)

	pkt := buildSCCRPWithout(t, avpNone, 100, 555)
	hdr, err := ParseMessageHeader(pkt)
	require.NoError(t, err)
	out := tun.Process(hdr, pkt[hdr.PayloadOff:hdr.Length], now, defaults, nil)

	require.Equal(t, L2TPTunnelEstablished, tun.state)
	require.EqualValues(t, 555, tun.remoteTID, "the peer's Assigned Tunnel ID is adopted")
	require.EqualValues(t, 0x3, tun.peerFraming, "the mask the SCCRP carried, read off the AVP")
	require.Equal(t, "peer-lns", tun.peerHostName)
	require.Len(t, out, 1, "an accepted SCCRP emits the SCCCN")
	shdr, err := ParseMessageHeader(out[0].bytes)
	require.NoError(t, err)
	require.EqualValues(t, MsgSCCCN, extractMsgType(out[0].bytes[shdr.PayloadOff:shdr.Length]))
}
