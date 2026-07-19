package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// initiatorDefaults are the TunnelDefaults used across initiator tests.
func initiatorDefaults(secret string) TunnelDefaults {
	return TunnelDefaults{
		HostName:            "ze-lac",
		FramingCapabilities: 0x3,
		BearerCapabilities:  0x3,
		RecvWindow:          8,
		SharedSecret:        secret,
	}
}

// buildSCCRPWire assembles a full SCCRP control datagram addressed to our
// local tunnel ID, acking our SCCRQ (Nr=1) with the peer's first message
// (Ns=0). challenge and challengeResp map to the SCCRP Challenge and
// Challenge Response AVPs (nil = omitted).
func buildSCCRPWire(t *testing.T, ourLocalTID, peerAssignedTID uint16, challenge, challengeResp []byte) []byte {
	t.Helper()
	body := make([]byte, 512)
	n := writeSCCRPBody(body, peerAssignedTID, initiatorDefaults(""), challenge, challengeResp)
	pkt := make([]byte, ControlHeaderLen+n)
	WriteControlHeader(pkt, 0, uint16(ControlHeaderLen+n), ourLocalTID, 0, 0, 1)
	copy(pkt[ControlHeaderLen:], body[:n])
	return pkt
}

// --- Encoder / parser round-trips (AC-1 codecs) ---

func TestWriteSCCRQBody_RoundTrip(t *testing.T) {
	// VALIDATES: writeSCCRQBody produces a body the existing parseSCCRQ
	// accepts, with challenge + tie-breaker AVPs surviving.
	buf := make([]byte, 512)
	challenge := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	tb := []byte{0xA, 0xB, 0xC, 0xD, 0xE, 0xF, 0x10, 0x11}
	n := writeSCCRQBody(buf, 0xBEEF, initiatorDefaults(""), challenge, tb)

	info, err := parseSCCRQ(buf[:n])
	require.NoError(t, err)
	require.Equal(t, MsgSCCRQ, info.MessageType)
	require.EqualValues(t, 0xBEEF, info.AssignedTunnelID)
	require.Equal(t, "ze-lac", info.HostName)
	require.EqualValues(t, 0x3, info.FramingCapabilities)
	require.EqualValues(t, 8, info.RecvWindow)
	require.True(t, info.ChallengePresent)
	require.Equal(t, challenge, info.ChallengeValue)
	require.True(t, info.TieBreakerPresent)
	require.Equal(t, tb, info.TieBreakerValue)
}

func TestWriteSCCRQBody_NoOptionalAVPs(t *testing.T) {
	// VALIDATES: without a secret / tie-breaker the SCCRQ omits the optional
	// AVPs and still parses.
	buf := make([]byte, 512)
	n := writeSCCRQBody(buf, 5, initiatorDefaults(""), nil, nil)
	info, err := parseSCCRQ(buf[:n])
	require.NoError(t, err)
	require.False(t, info.ChallengePresent)
	require.False(t, info.TieBreakerPresent)
}

func TestWriteSCCCNBody_RoundTrip(t *testing.T) {
	// VALIDATES: writeSCCCNBody with/without a challenge response parses.
	buf := make([]byte, 128)
	resp := []byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5, 6}
	n := writeSCCCNBody(buf, resp)
	info, err := parseSCCCN(buf[:n])
	require.NoError(t, err)
	require.Equal(t, MsgSCCCN, info.MessageType)
	require.True(t, info.ChallengeResponsePresent)
	require.Equal(t, resp, info.ChallengeResponseValue)

	m := writeSCCCNBody(buf, nil)
	info2, err := parseSCCCN(buf[:m])
	require.NoError(t, err)
	require.False(t, info2.ChallengeResponsePresent)
}

func TestParseSCCRP_RoundTrip(t *testing.T) {
	// VALIDATES: parseSCCRP reads back a body produced by the existing
	// writeSCCRPBody encoder, capturing challenge + response + tunnel ID.
	buf := make([]byte, 512)
	challenge := []byte{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8}
	resp := []byte{8, 8, 7, 7, 6, 6, 5, 5, 4, 4, 3, 3, 2, 2, 1, 1}
	n := writeSCCRPBody(buf, 0x1234, initiatorDefaults(""), challenge, resp)

	info, err := parseSCCRP(buf[:n])
	require.NoError(t, err)
	require.Equal(t, MsgSCCRP, info.MessageType)
	require.EqualValues(t, 0x1234, info.AssignedTunnelID)
	require.Equal(t, "ze-lac", info.HostName)
	require.True(t, info.ChallengePresent)
	require.Equal(t, challenge, info.ChallengeValue)
	require.True(t, info.ChallengeResponsePresent)
	require.Equal(t, resp, info.ChallengeResponseValue)
}

// RFC requirement: RFC2661-24.10-1 negative -- an SCCRP whose Assigned Tunnel ID
// is 0 (or absent) is a protocol error: parseSCCRP rejects it, so no tunnel is
// adopted from a zero peer TID. (Ze drops the SCCRP here rather than emitting a
// StopCCN; this tags the code's actual behavior.)
func TestParseSCCRP_Rejects(t *testing.T) {
	// VALIDATES: parseSCCRP rejects an empty body and a missing/zero
	// Assigned Tunnel ID.
	_, err := parseSCCRP(nil)
	require.Error(t, err)

	// Message Type only, no Assigned Tunnel ID.
	buf := make([]byte, 32)
	n := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgSCCRP))
	_, err = parseSCCRP(buf[:n])
	require.Error(t, err)

	// Zero Assigned Tunnel ID.
	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgSCCRP))
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, 0)
	_, err = parseSCCRP(buf[:off])
	require.Error(t, err)
}

// --- Initiator FSM (AC-1, AC-2) ---

// RFC requirement: RFC2661-4.1-4 positive -- a tunnel-scoped control exchange with
// no unrecognized mandatory AVP is accepted: a clean SCCRP drives SCCCN emission
// and the tunnel reaches Established rather than being torn down with StopCCN.
// RFC requirement: RFC2661-24.10-1 positive -- an SCCRP carrying a non-zero
// Assigned Tunnel ID (555) is accepted and adopted as the peer's tunnel ID.
func TestTunnelInitiatorHandshake(t *testing.T) {
	// VALIDATES: AC-1 + AC-2 -- dial sends SCCRQ (peer TID 0, our local TID
	// in Assigned Tunnel ID) and enters wait-ctl-reply; the peer's SCCRP
	// drives SCCCN emission and established, adopting the peer's TID.
	logger := slog.Default()
	now := time.Now()
	defaults := initiatorDefaults("")
	tun := newTunnel(100, 0, netip.MustParseAddrPort("10.0.0.2:1701"),
		ReliableConfig{RecvWindow: 8}, logger, now)

	out := tun.initiate(now, defaults, nil)
	require.Len(t, out, 1, "initiate emits one SCCRQ")
	require.Equal(t, L2TPTunnelWaitCtlReply, tun.state)

	hdr, err := ParseMessageHeader(out[0].bytes)
	require.NoError(t, err)
	require.EqualValues(t, 0, hdr.TunnelID, "SCCRQ header carries PeerTunnelID=0")
	sccrq, err := parseSCCRQ(out[0].bytes[hdr.PayloadOff:hdr.Length])
	require.NoError(t, err)
	require.EqualValues(t, 100, sccrq.AssignedTunnelID)

	// Peer answers with SCCRP assigning TID 555.
	pkt := buildSCCRPWire(t, 100, 555, nil, nil)
	rhdr, err := ParseMessageHeader(pkt)
	require.NoError(t, err)
	out2 := tun.Process(rhdr, pkt[rhdr.PayloadOff:rhdr.Length], now, defaults, nil)

	require.Equal(t, L2TPTunnelEstablished, tun.state)
	require.EqualValues(t, 555, tun.remoteTID)
	require.Len(t, out2, 1, "SCCRP delivery emits one SCCCN")

	shdr, err := ParseMessageHeader(out2[0].bytes)
	require.NoError(t, err)
	require.EqualValues(t, 555, shdr.TunnelID, "SCCCN header carries the peer's TID")
	scccn, err := parseSCCCN(out2[0].bytes[shdr.PayloadOff:shdr.Length])
	require.NoError(t, err)
	require.Equal(t, MsgSCCCN, scccn.MessageType)
}

func TestInitiatorMutualChallenge(t *testing.T) {
	// VALIDATES: AC-2 -- with a shared secret, the SCCRQ carries a Challenge;
	// a valid SCCRP (correct response to our challenge + its own challenge)
	// yields an SCCCN carrying our response, and the tunnel establishes.
	logger := slog.Default()
	now := time.Now()
	secret := "s3cr3t"
	defaults := initiatorDefaults(secret)
	tun := newTunnel(100, 0, netip.MustParseAddrPort("10.0.0.2:1701"),
		ReliableConfig{RecvWindow: 8}, logger, now)

	out := tun.initiate(now, defaults, nil)
	require.Len(t, out, 1)
	require.NotNil(t, tun.ourChallenge, "secret set => we challenge the peer")
	ourChallenge := append([]byte(nil), tun.ourChallenge...)

	// Peer's SCCRP: answer our challenge, and issue its own challenge.
	peerResp := ChallengeResponse(ChapIDSCCRP, []byte(secret), ourChallenge)
	peerChallenge := []byte{0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xCB, 0xCC, 0xCD, 0xCE, 0xCF}
	pkt := buildSCCRPWire(t, 100, 555, peerChallenge, peerResp[:])
	rhdr, err := ParseMessageHeader(pkt)
	require.NoError(t, err)
	out2 := tun.Process(rhdr, pkt[rhdr.PayloadOff:rhdr.Length], now, defaults, nil)

	require.Equal(t, L2TPTunnelEstablished, tun.state)
	require.Len(t, out2, 1)
	shdr, err := ParseMessageHeader(out2[0].bytes)
	require.NoError(t, err)
	scccn, err := parseSCCCN(out2[0].bytes[shdr.PayloadOff:shdr.Length])
	require.NoError(t, err)
	require.True(t, scccn.ChallengeResponsePresent, "SCCCN answers the peer challenge")
	want := ChallengeResponse(ChapIDSCCCN, []byte(secret), peerChallenge)
	require.Equal(t, want[:], scccn.ChallengeResponseValue)
}

func TestInitiatorChallengeReject(t *testing.T) {
	// VALIDATES: AC-2 -- an SCCRP missing the Challenge Response (when we
	// challenged) tears the tunnel down with StopCCN and enters closed.
	logger := slog.Default()
	now := time.Now()
	defaults := initiatorDefaults("s3cr3t")
	tun := newTunnel(100, 0, netip.MustParseAddrPort("10.0.0.2:1701"),
		ReliableConfig{RecvWindow: 8}, logger, now)

	require.Len(t, tun.initiate(now, defaults, nil), 1)

	// SCCRP with NO challenge response.
	pkt := buildSCCRPWire(t, 100, 555, nil, nil)
	rhdr, err := ParseMessageHeader(pkt)
	require.NoError(t, err)
	out := tun.Process(rhdr, pkt[rhdr.PayloadOff:rhdr.Length], now, defaults, nil)

	require.Equal(t, L2TPTunnelClosed, tun.state)
	require.Len(t, out, 1, "a StopCCN is emitted")
	shdr, err := ParseMessageHeader(out[0].bytes)
	require.NoError(t, err)
	sc, err := parseStopCCN(out[0].bytes[shdr.PayloadOff:shdr.Length])
	require.NoError(t, err)
	require.EqualValues(t, resultNotAuthorized, sc.Result)
}

func TestInitiatorSCCRPWrongState(t *testing.T) {
	// VALIDATES: an SCCRP delivered to a tunnel that never dialed (idle) is
	// ignored, not acted upon.
	logger := slog.Default()
	now := time.Now()
	defaults := initiatorDefaults("")
	tun := newTunnel(100, 200, netip.MustParseAddrPort("10.0.0.2:1701"),
		ReliableConfig{RecvWindow: 8}, logger, now)
	// idle tunnel: handleSCCRP must no-op.
	out := tun.handleSCCRP(now, defaults, []byte{})
	require.Nil(t, out)
	require.Equal(t, L2TPTunnelIdle, tun.state)
}

// RFC requirement: RFC2661-4.1-4 negative -- an unrecognized mandatory (M=1)
// vendor AVP in a tunnel-scoped SCCCN makes parseSCCCN reject the body, and
// handleSCCCN tears the tunnel down with a StopCCN instead of establishing it.
func TestTunnelSCCCNUnknownMandatoryAVP_StopCCN(t *testing.T) {
	logger := slog.Default()
	now := time.Now()
	defaults := initiatorDefaults("")
	tun := newTunnel(100, 200, netip.MustParseAddrPort("10.0.0.2:1701"),
		ReliableConfig{RecvWindow: 8}, logger, now)
	tun.state = L2TPTunnelWaitCtlConn

	// SCCCN body: Message Type first (valid), then an unknown mandatory vendor AVP.
	var buf [128]byte
	off := 0
	off += WriteAVPUint16(buf[:], off, true, AVPMessageType, uint16(MsgSCCCN))
	off += WriteAVPBytes(buf[:], off, true, 9999, AVPType(1), []byte{0x01})

	out := tun.handleSCCCN(now, defaults, buf[:off])
	require.Len(t, out, 1, "unknown mandatory AVP must produce exactly one StopCCN")
	require.Equal(t, L2TPTunnelClosed, tun.state, "tunnel must be torn down")

	shdr, err := ParseMessageHeader(out[0].bytes)
	require.NoError(t, err)
	sc, err := parseStopCCN(out[0].bytes[shdr.PayloadOff:shdr.Length])
	require.NoError(t, err, "emitted message must parse as a StopCCN")
	require.EqualValues(t, resultNotAuthorized, sc.Result, "code's actual teardown Result Code")
}
