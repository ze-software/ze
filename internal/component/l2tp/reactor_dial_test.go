package l2tp

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// clientAddrPort returns the test client's bound addr:port as a netip value.
func clientAddrPort(t *testing.T, c *testClient) netip.AddrPort {
	t.Helper()
	ap, err := netip.ParseAddrPort(c.conn.LocalAddr().String())
	require.NoError(t, err)
	return ap
}

// buildSCCRPForSCCRQ answers ze's SCCRQ: it reads ze's Assigned Tunnel ID
// (the header TunnelID we must address the reply to) and builds an SCCRP
// whose body assigns peerAssignedTID as the peer's own tunnel ID.
func buildSCCRPForSCCRQ(t *testing.T, sccrq []byte, peerAssignedTID uint16) []byte {
	t.Helper()
	zeTIDBytes := extractAVP(t, sccrq, AVPAssignedTunnelID)
	zeTID := uint16(zeTIDBytes[0])<<8 | uint16(zeTIDBytes[1])

	body := make([]byte, 512)
	n := writeSCCRPBody(body, peerAssignedTID, TunnelDefaults{HostName: "peer-lns", FramingCapabilities: 0x3, RecvWindow: 8}, nil, nil)
	pkt := make([]byte, ControlHeaderLen+n)
	WriteControlHeader(pkt, 0, uint16(ControlHeaderLen+n), zeTID, 0, 0, 1)
	copy(pkt[ControlHeaderLen:], body[:n])
	return pkt
}

// TestReactor_DialCreatesInitiatorTunnel -- R-2 (single-goroutine ownership).
//
// VALIDATES: Dial (called from a foreign goroutine) marshals a request onto
// the reactor's single goroutine, which creates the tunnel and sends the
// SCCRQ. The whole suite runs under -race, so a second tunnel-map writer
// would trip the detector here.
func TestReactor_DialCreatesInitiatorTunnel(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	localTID, err := r.Dial(DialTarget{Remote: clientAddrPort(t, client)})
	require.NoError(t, err)
	require.NotZero(t, localTID)
	waitForLog(t, logs, "SCCRQ sent; tunnel now wait-ctl-reply")

	// The client received a well-formed SCCRQ addressed from ze's listener.
	sccrq := readDatagram(t, client)
	hdr, err := ParseMessageHeader(sccrq)
	require.NoError(t, err)
	require.EqualValues(t, 0, hdr.TunnelID)
	info, err := parseSCCRQ(sccrq[hdr.PayloadOff:hdr.Length])
	require.NoError(t, err)
	require.EqualValues(t, localTID, info.AssignedTunnelID)

	tun := r.TunnelByLocalID(localTID)
	require.NotNil(t, tun)
	r.tunnelsMu.Lock()
	state := tun.state
	hasTB := tun.tieBreaker != nil
	r.tunnelsMu.Unlock()
	require.Equal(t, L2TPTunnelWaitCtlReply, state)
	require.True(t, hasTB, "dial installs a tie breaker for simultaneous-open resolution")
}

// TestReactor_DialLoopbackHandshake -- AC-1 + AC-2 end-to-end over UDP.
//
// VALIDATES: ze dials, sends SCCRQ, and on the peer's SCCRP emits SCCCN and
// reaches established -- the initiator half of a full loopback handshake.
func TestReactor_DialLoopbackHandshake(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	timer := newTunnelTimer(r.tickCh, r.updateCh)
	require.NoError(t, timer.Start())
	defer timer.Stop()

	client := newClient(t, ln)
	defer client.Close()

	localTID, err := r.Dial(DialTarget{Remote: clientAddrPort(t, client)})
	require.NoError(t, err)

	sccrq := readDatagram(t, client)
	client.Send(t, buildSCCRPForSCCRQ(t, sccrq, 909))
	waitForLog(t, logs, "tunnel now established (initiator)")

	// ze sent an SCCCN whose header carries the peer's assigned TID (909).
	scccn := readDatagram(t, client)
	shdr, err := ParseMessageHeader(scccn)
	require.NoError(t, err)
	require.EqualValues(t, 909, shdr.TunnelID)

	tun := r.TunnelByLocalID(localTID)
	require.NotNil(t, tun)
	r.tunnelsMu.Lock()
	state := tun.state
	remoteTID := tun.remoteTID
	r.tunnelsMu.Unlock()
	require.Equal(t, L2TPTunnelEstablished, state)
	require.EqualValues(t, 909, remoteTID)
}

// TestReactor_InitiatorTieBreaker -- AC-8 (simultaneous open).
//
// VALIDATES: after ze dials (creating an initiator tunnel with a tie
// breaker), a crossed SCCRQ from the same peer carrying its own tie breaker
// resolves to exactly one tunnel per RFC 2661 S9.5 -- the lower value wins.
func TestReactor_InitiatorTieBreaker(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	localTID, err := r.Dial(DialTarget{Remote: clientAddrPort(t, client)})
	require.NoError(t, err)
	_ = readDatagram(t, client) // drain ze's SCCRQ

	tun := r.TunnelByLocalID(localTID)
	require.NotNil(t, tun)
	r.tunnelsMu.Lock()
	zeTB := append([]byte(nil), tun.tieBreaker...)
	r.tunnelsMu.Unlock()
	require.Len(t, zeTB, 8)

	// Build a peer tie breaker guaranteed different from ze's (flip a bit),
	// then assert the outcome dictated by byte-wise comparison. Lower wins.
	peerTB := append([]byte(nil), zeTB...)
	peerTB[7] ^= 0x01

	client.Send(t, buildSCCRQWithTieBreaker(t, 42, "peer-x", peerTB))

	if bytes.Compare(peerTB, zeTB) > 0 {
		// Peer's tie breaker is higher -> peer's SCCRQ is discarded; ze keeps
		// its initiator tunnel.
		waitForLog(t, logs, "new SCCRQ discarded by tie breaker")
		require.NotNil(t, r.TunnelByLocalID(localTID), "ze's initiator tunnel survives")
	} else {
		// Peer's tie breaker is lower -> ze's initiator tunnel is discarded and
		// a new answering tunnel is created for the peer's SCCRQ.
		waitForLog(t, logs, "tunnel discarded")
		require.Nil(t, r.TunnelByLocalID(localTID), "ze's initiator tunnel loses and is discarded")
	}
	// Never two tunnels to the same peer after a tie-broken crossed open.
	require.LessOrEqual(t, r.TunnelCount(), 1)
}

// msgTypeOf returns the L2TP message type of a control datagram.
func msgTypeOf(t *testing.T, pkt []byte) MessageType {
	t.Helper()
	hdr, err := ParseMessageHeader(pkt)
	require.NoError(t, err)
	return extractMsgType(pkt[hdr.PayloadOff:hdr.Length])
}

// readUntilMsgType reads datagrams from the client until it sees the wanted
// message type or runs out of attempts.
func readUntilMsgType(t *testing.T, c *testClient, want MessageType, attempts int) []byte {
	t.Helper()
	for range attempts {
		pkt := readDatagram(t, c)
		if msgTypeOf(t, pkt) == want {
			return pkt
		}
	}
	t.Fatalf("did not observe message type %d after %d datagrams", want, attempts)
	return nil
}

// TestReactor_PlaceOutgoingCall_AutoOCRQ -- AC-4 orchestration seam.
//
// VALIDATES: PlaceOutgoingCall dials, and the moment the tunnel establishes
// (peer SCCRP) the reactor auto-originates the outgoing call (OCRQ) and a
// wait-reply session appears -- proving the dial -> establish -> place-call
// orchestration on the single reactor goroutine.
func TestReactor_PlaceOutgoingCall_AutoOCRQ(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()
	timer := newTunnelTimer(r.tickCh, r.updateCh)
	require.NoError(t, timer.Start())
	defer timer.Stop()

	client := newClient(t, ln)
	defer client.Close()

	localTID, err := r.PlaceOutgoingCall(DialTarget{Remote: clientAddrPort(t, client)},
		callParams{callSerial: 7, minBPS: 9600, maxBPS: 128000, bearerType: 1, framingType: 1, calledNumber: "5551234"})
	require.NoError(t, err)

	sccrq := readDatagram(t, client)
	client.Send(t, buildSCCRPForSCCRQ(t, sccrq, 909))
	waitForLog(t, logs, "OCRQ sent; session wait-reply (LNS outgoing)")

	// After SCCRP: ze emits SCCCN then the auto-placed OCRQ.
	ocrq := readUntilMsgType(t, client, MsgOCRQ, 3)
	hdr, err := ParseMessageHeader(ocrq)
	require.NoError(t, err)
	info, err := parseOCRQ(ocrq[hdr.PayloadOff:hdr.Length])
	require.NoError(t, err)
	require.Equal(t, "5551234", info.calledNumber)

	// A wait-reply session (lnsMode true) exists on the tunnel.
	tun := r.TunnelByLocalID(localTID)
	require.NotNil(t, tun)
	r.tunnelsMu.Lock()
	require.Equal(t, 1, len(tun.sessions))
	var sess *L2TPSession
	for _, s := range tun.sessions {
		sess = s
	}
	r.tunnelsMu.Unlock()
	require.NotNil(t, sess)
	require.Equal(t, L2TPSessionWaitReply, sess.State())
	require.True(t, sess.lnsMode)
}

// TestReactor_PlaceIncomingCall_AutoICRQ -- AC-3 orchestration seam.
//
// VALIDATES: PlaceIncomingCall dials, and on establishment the reactor
// auto-originates the incoming call (ICRQ) with a wait-reply session.
func TestReactor_PlaceIncomingCall_AutoICRQ(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()
	timer := newTunnelTimer(r.tickCh, r.updateCh)
	require.NoError(t, timer.Start())
	defer timer.Stop()

	client := newClient(t, ln)
	defer client.Close()

	localTID, err := r.PlaceIncomingCall(DialTarget{Remote: clientAddrPort(t, client)},
		callParams{callSerial: 42, bearerType: 1, framingType: 1, txConnectSpeed: 1_000_000, calledNumber: "5559000"})
	require.NoError(t, err)

	sccrq := readDatagram(t, client)
	client.Send(t, buildSCCRPForSCCRQ(t, sccrq, 909))
	waitForLog(t, logs, "ICRQ sent; session wait-reply (LAC incoming)")

	icrq := readUntilMsgType(t, client, MsgICRQ, 3)
	hdr, err := ParseMessageHeader(icrq)
	require.NoError(t, err)
	info, err := parseICRQ(icrq[hdr.PayloadOff:hdr.Length])
	require.NoError(t, err)
	require.EqualValues(t, 42, info.callSerialNumber)

	tun := r.TunnelByLocalID(localTID)
	require.NotNil(t, tun)
	r.tunnelsMu.Lock()
	require.Equal(t, 1, len(tun.sessions))
	r.tunnelsMu.Unlock()
}
