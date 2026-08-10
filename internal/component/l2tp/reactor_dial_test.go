package l2tp

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

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

	tun := r.tunnelByLocalID(localTID)
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

	tun := r.tunnelByLocalID(localTID)
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

	tun := r.tunnelByLocalID(localTID)
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
		require.NotNil(t, r.tunnelByLocalID(localTID), "ze's initiator tunnel survives")
	} else {
		// Peer's tie breaker is lower -> ze's initiator tunnel is discarded and
		// a new answering tunnel is created for the peer's SCCRQ.
		waitForLog(t, logs, "tunnel discarded")
		require.Nil(t, r.tunnelByLocalID(localTID), "ze's initiator tunnel loses and is discarded")
	}
	// Never two tunnels to the same peer after a tie-broken crossed open.
	require.LessOrEqual(t, r.TunnelCount(), 1)
}

// syncCallResult bundles placeOutgoingCallSync's two returns for delivery
// over a channel from the goroutine that makes the blocking call.
type syncCallResult struct {
	outcome callOutcome
	err     error
}

// TestReactor_placeOutgoingCallSync_TieBreakerLoss -- AC-4 + AC-8 edge case.
//
// VALIDATES: a dialed tunnel whose pending outgoing call loses the
// simultaneous-open tie-breaker is discarded, and placeOutgoingCallSync
// surfaces that as a FAILURE outcome (not a silent drop, not a timeout). This
// is the edge case the session-state digest flagged: without resolvePendingCall
// in discardTunnelLocked the RPC would hang until timeout.
func TestReactor_placeOutgoingCallSync_TieBreakerLoss(t *testing.T) {
	ln, r, _, stop := buildLogReactor(t)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	resCh := make(chan syncCallResult, 1)
	go func() {
		o, err := r.placeOutgoingCallSync(DialTarget{Remote: clientAddrPort(t, client)},
			callParams{calledNumber: "5551234"}, 3*time.Second)
		resCh <- syncCallResult{o, err}
	}()

	// Read ze's SCCRQ, then cross it with a peer SCCRQ whose tie breaker is
	// all zeros -- guaranteed <= ze's, so ze's dialed tunnel loses (or ties;
	// either way it is discarded and its pending call fails).
	_ = readDatagram(t, client)
	peerTB := make([]byte, 8) // all zeros: minimal value
	client.Send(t, buildSCCRQWithTieBreaker(t, 42, "peer-x", peerTB))

	select {
	case rr := <-resCh:
		require.NoError(t, rr.err, "no transport error; the outcome carries the failure")
		require.Error(t, rr.outcome.err, "tie-breaker loss must surface as a call failure")
		require.NotErrorIs(t, rr.err, ErrCallTimeout, "must be a discard failure, not a timeout")
		require.Zero(t, rr.outcome.localSID, "call never placed on a session")
	case <-time.After(5 * time.Second):
		t.Fatal("placeOutgoingCallSync hung after tie-breaker loss (edge case regressed)")
	}
}

// TestReactor_placeOutgoingCallSync_Timeout -- AC-4 timeout path.
//
// VALIDATES: a dial whose peer never answers returns ErrCallTimeout rather
// than blocking forever.
func TestReactor_placeOutgoingCallSync_Timeout(t *testing.T) {
	ln, r, _, stop := buildLogReactor(t)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	o, err := r.placeOutgoingCallSync(DialTarget{Remote: clientAddrPort(t, client)},
		callParams{calledNumber: "555"}, 150*time.Millisecond)
	require.ErrorIs(t, err, ErrCallTimeout)
	require.Zero(t, o.localSID)
	_ = readDatagram(t, client) // ze did send the SCCRQ before we gave up
}

// TestReactor_placeOutgoingCallSync_AuthReject -- AC-4 auth-failure surfacing.
//
// VALIDATES: dialing a remote with a shared secret and receiving an SCCRP
// with no Challenge Response tears the tunnel down (StopCCN RC=4) and
// placeOutgoingCallSync reports the authentication failure with the result code.
func TestReactor_placeOutgoingCallSync_AuthReject(t *testing.T) {
	ln, r, _, stop := buildLogReactor(t)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	resCh := make(chan syncCallResult, 1)
	go func() {
		o, err := r.placeOutgoingCallSync(
			DialTarget{Remote: clientAddrPort(t, client), SharedSecret: "s3cr3t"},
			callParams{calledNumber: "555"}, 3*time.Second)
		resCh <- syncCallResult{o, err}
	}()

	sccrq := readDatagram(t, client)
	// buildSCCRPForSCCRQ answers with no Challenge Response; ze sent a
	// Challenge (secret set), so it rejects with StopCCN RC=4. The peer TID
	// here is immaterial (the tunnel never establishes).
	client.Send(t, buildSCCRPForSCCRQ(t, sccrq, 777))

	select {
	case rr := <-resCh:
		require.NoError(t, rr.err)
		require.ErrorIs(t, rr.outcome.err, errCallTunnelAuthFailed)
		require.EqualValues(t, resultNotAuthorized, rr.outcome.resultCode)
	case <-time.After(5 * time.Second):
		t.Fatal("placeOutgoingCallSync hung after auth reject")
	}
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
	tun := r.tunnelByLocalID(localTID)
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
// VALIDATES: placeIncomingCall dials, and on establishment the reactor
// auto-originates the incoming call (ICRQ) with a wait-reply session.
func TestReactor_PlaceIncomingCall_AutoICRQ(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()
	timer := newTunnelTimer(r.tickCh, r.updateCh)
	require.NoError(t, timer.Start())
	defer timer.Stop()

	client := newClient(t, ln)
	defer client.Close()

	localTID, err := r.placeIncomingCall(DialTarget{Remote: clientAddrPort(t, client)},
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

	tun := r.tunnelByLocalID(localTID)
	require.NotNil(t, tun)
	r.tunnelsMu.Lock()
	require.Equal(t, 1, len(tun.sessions))
	r.tunnelsMu.Unlock()
}
