package l2tp

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// lockedBuffer serializes writes and reads so a test can race-safely
// inspect log output produced by the reactor goroutine.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (lb *lockedBuffer) Write(p []byte) (int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(p)
}

func (lb *lockedBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.String()
}

// waitForLogTimeout is the fixed deadline for waitForLog. Tests do not
// need per-call tuning: 2s is comfortably above the observed dispatch
// latency (sub-ms) and still fails fast under a real bug.
const waitForLogTimeout = 2 * time.Second

// waitForLog polls until `needle` appears in the captured log output or
// waitForLogTimeout elapses. Replaces the earlier `time.Sleep(100ms)`
// pattern which hid timing bugs (see
// .claude/memory/feedback_sleep_hides_races.md); the poll exits as soon
// as the reactor's log line lands rather than waiting for a fixed delay.
func waitForLog(t *testing.T, logs *lockedBuffer, needle string) {
	t.Helper()
	deadline := time.Now().Add(waitForLogTimeout)
	for time.Now().Before(deadline) {
		if strings.Contains(logs.String(), needle) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for log substring %q; got:\n%s", needle, logs.String())
}

// send submits `payload` from a fresh loopback client socket to the
// reactor's bound listener. Each call uses a new ephemeral source port;
// tests that need two sends to share a source port (SCCRQ retransmit
// dedup) should use newClient + client.Send instead.
func send(t *testing.T, ln *UDPListener, payload []byte) {
	t.Helper()
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	defer client.Close() //nolint:errcheck // test cleanup

	srvAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(ln.Addr().Port())}
	_, err = client.WriteToUDP(payload, srvAddr)
	require.NoError(t, err)
}

// testClient is a persistent UDP socket for tests that need multiple
// packets to share the same source addr:port.
type testClient struct {
	conn    *net.UDPConn
	srvAddr *net.UDPAddr
}

func newClient(t *testing.T, ln *UDPListener) *testClient {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	return &testClient{
		conn:    conn,
		srvAddr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(ln.Addr().Port())},
	}
}

func (c *testClient) Send(t *testing.T, payload []byte) {
	t.Helper()
	_, err := c.conn.WriteToUDP(payload, c.srvAddr)
	require.NoError(t, err)
}

func (c *testClient) Close() { _ = c.conn.Close() }

// buildLogReactor constructs a listener + reactor pair whose logger writes
// into a locked buffer for race-safe assertion. The caller must call
// stop() when done.
func buildLogReactor(t *testing.T) (*UDPListener, *L2TPReactor, *lockedBuffer, func()) {
	t.Helper()
	buf := &lockedBuffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ln := NewUDPListener(netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0), logger)
	require.NoError(t, ln.Start(context.Background()))
	r := NewL2TPReactor(ln, logger, ReactorParams{
		Defaults: TunnelDefaults{HostName: "ze-test", FramingCapabilities: 0x3, RecvWindow: 16},
	})
	require.NoError(t, r.Start())

	stop := func() {
		r.Stop()
		_ = ln.Stop()
	}
	return ln, r, buf, stop
}

// TestReactor_ShortDatagramDropped — AC-3.
//
// VALIDATES: AC-3 -- datagrams < 6 bytes silently dropped; debug log
// recorded; no panic.
func TestReactor_ShortDatagramDropped(t *testing.T) {
	ln, _, logs, stop := buildLogReactor(t)
	defer stop()

	send(t, ln, []byte{0x01, 0x02})
	waitForLog(t, logs, "short datagram dropped")
}

// TestReactor_V1Dropped — AC-5. L2F (Ver=1) is silently discarded.
//
// VALIDATES: AC-5 -- L2F packets dropped without crash or response.
func TestReactor_V1Dropped(t *testing.T) {
	ln, _, logs, stop := buildLogReactor(t)
	defer stop()

	// Control message with T=1,L=1,S=1,Ver=1. Flag word = 0xC801. Minimum
	// 12-byte header (T=1,L=1,S=1,O=0).
	pkt := []byte{0xC8, 0x01, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	send(t, ln, pkt)
	waitForLog(t, logs, "unsupported version")
	require.NotContains(t, logs.String(), "L2TPv3")
}

// TestReactor_V3Dropped — AC-4 phase-2 partial. Ver=3 (L2TPv3) is logged
// at WARN level but StopCCN RC=5 emission lands in phase 3.
//
// VALIDATES: AC-4 (phase 2) -- L2TPv3 peer recognized; reactor does not
// crash and logs a warn so operators can observe v3 rollout.
func TestReactor_V3Dropped(t *testing.T) {
	ln, _, logs, stop := buildLogReactor(t)
	defer stop()

	// Control message with Ver=3. Flag word lower nibble = 3. Upper byte
	// carries T=1,L=1,S=1,O=0,P=0 = 0xC8; lower byte carries 0x03.
	pkt := []byte{0xC8, 0x03, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	send(t, ln, pkt)
	waitForLog(t, logs, "L2TPv3 peer rejected")
}

// TestReactor_MalformedDropped — short header with valid version.
//
// VALIDATES: malformed v2 packets drop without panic; debug log recorded.
func TestReactor_MalformedDropped(t *testing.T) {
	ln, _, logs, stop := buildLogReactor(t)
	defer stop()

	// Flag word 0xC802 implies S=1,L=1,O=0 so header >= 12 bytes, but we
	// send only 8. ParseMessageHeader rejects as ErrShortBuffer.
	pkt := []byte{0xC8, 0x02, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00}
	send(t, ln, pkt)
	waitForLog(t, logs, "malformed header dropped")
}

// TestReactor_EmptyControlBodyDropped — a valid Ver=2 header with no
// AVP body is dropped as "unparseable" because the Message Type AVP is
// required (RFC 2661 S4.1).
//
// VALIDATES: the phase-2-era minimum-header packet no longer advances
// the reactor past AVP parsing; it drops with a debug log instead of
// reaching the FSM.
func TestReactor_EmptyControlBodyDropped(t *testing.T) {
	ln, _, logs, stop := buildLogReactor(t)
	defer stop()

	// Flag word 0xC802 + Length=12 + TunnelID=0 + SessionID=0 + Ns=0 +
	// Nr=0 = the smallest well-formed control header. No AVPs follow.
	pkt := []byte{0xC8, 0x02, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	send(t, ln, pkt)
	waitForLog(t, logs, "TunnelID=0 packet with malformed body dropped")
}

// buildSCCRQ returns a complete Ver=2 control datagram carrying a valid
// SCCRQ. peerTID is the Assigned Tunnel ID the test peer is claiming;
// hostName is the peer's Host Name AVP value.
func buildSCCRQ(t *testing.T, peerTID uint16, hostName string) []byte {
	t.Helper()
	// Encode AVP body using phase-1 helpers into a pooled buffer.
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	off := 0
	buf := *bodyBuf
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, []byte{0x01, 0x00})
	off += WriteAVPUint32(buf, off, true, AVPFramingCapabilities, 0x3)
	off += WriteAVPUint32(buf, off, true, AVPBearerCapabilities, 0x0)
	off += WriteAVPString(buf, off, true, AVPHostName, hostName)
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, peerTID)
	off += WriteAVPUint16(buf, off, true, AVPReceiveWindowSize, 8)

	// Wrap in control header. TunnelID=0 (peer has not received SCCRP
	// yet so does not know our local TID); Ns=0, Nr=0 for first message.
	total := 12 + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), 0, 0, 0, 0)
	copy(pkt[12:], buf[:off])
	return pkt
}

// TestReactor_TunnelCreatedFromSCCRQ — AC-6. A valid SCCRQ creates a
// tunnel, moves it to wait-ctl-conn, and emits SCCRP.
//
// VALIDATES: AC-6 -- idle -> wait-ctl-conn on valid SCCRQ; SCCRP sent
// with allocated local TunnelID.
func TestReactor_TunnelCreatedFromSCCRQ(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	pkt := buildSCCRQ(t, 42, "peer-a")
	send(t, ln, pkt)
	waitForLog(t, logs, "SCCRP sent; tunnel now wait-ctl-conn")

	require.Equal(t, 1, r.TunnelCount(), "exactly one tunnel should exist")

	// Walk the map under tunnelsMu to avoid racing with any subsequent
	// reactor-goroutine work and copy out the fields we need to assert
	// so we can release the lock before calling require.*.
	r.tunnelsMu.Lock()
	var state L2TPTunnelState
	var remoteTID uint16
	var peerHost string
	for _, t2 := range r.tunnelsByLocalID {
		state = t2.state
		remoteTID = t2.remoteTID
		peerHost = t2.peerHostName
	}
	r.tunnelsMu.Unlock()

	require.Equal(t, L2TPTunnelWaitCtlConn, state)
	require.Equal(t, uint16(42), remoteTID)
	require.Equal(t, "peer-a", peerHost)
}

// TestReactor_MalformedSCCRQCreatesNoTunnel — BLOCKER-fix regression.
// A peer that sends a TunnelID=0 packet whose AVP body fails the full
// parseSCCRQ validation MUST NOT consume a tunnel slot. Prior to the
// fix, locateTunnelLocked inserted the tunnel into both maps before
// handleSCCRQ re-parsed and failed, leaving a permanent entry.
//
// VALIDATES: phase-3 DoS mitigation -- reactor maps empty after a
// stream of malformed SCCRQs.
func TestReactor_MalformedSCCRQCreatesNoTunnel(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	// Build an SCCRQ body but corrupt the first AVP so parseSCCRQ
	// rejects it (Message Type AVP not first). We do this by
	// swapping the order of Message Type and Host Name AVPs in the
	// payload.
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	off := 0
	b := *bodyBuf
	// Put Host Name FIRST (wrong; Message Type MUST be first).
	off += WriteAVPString(b, off, true, AVPHostName, "bad-peer")
	off += WriteAVPUint16(b, off, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPUint16(b, off, true, AVPAssignedTunnelID, 333)
	total := 12 + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), 0, 0, 0, 0)
	copy(pkt[12:], b[:off])

	client.Send(t, pkt)
	waitForLog(t, logs, "malformed body dropped")
	require.Equal(t, 0, r.TunnelCount(), "malformed SCCRQ must not create a tunnel")
	r.tunnelsMu.Lock()
	peerMapLen := len(r.tunnelsByPeer)
	r.tunnelsMu.Unlock()
	require.Equal(t, 0, peerMapLen)
}

// TestReactor_SCCRQDedupBySecondaryIndex — AC-7. A retransmitted SCCRQ
// with the same (peer, peer-assigned-TID) must route to the existing
// tunnel's reliable engine rather than creating a second object.
//
// VALIDATES: AC-7 -- exactly one tunnel in both maps after the
// retransmit; reliable engine's duplicate-ACK path handles the second
// SCCRQ.
func TestReactor_SCCRQDedupBySecondaryIndex(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	// Both sends MUST share the same UDP source port, otherwise the
	// secondary key (peer addr:port, peer-assigned-TID) differs and
	// the reactor correctly creates two tunnels.
	client := newClient(t, ln)
	defer client.Close()

	pkt := buildSCCRQ(t, 77, "peer-b")
	client.Send(t, pkt)
	waitForLog(t, logs, "SCCRP sent")
	require.Equal(t, 1, r.TunnelCount())

	// Retransmit identical SCCRQ from the same source port. Secondary
	// index must match and route to the existing tunnel.
	client.Send(t, pkt)
	waitForLog(t, logs, "SCCRQ retransmit matched existing tunnel")
	require.Equal(t, 1, r.TunnelCount(), "retransmit must not create a second tunnel")
	r.tunnelsMu.Lock()
	peerMapLen := len(r.tunnelsByPeer)
	r.tunnelsMu.Unlock()
	require.Equal(t, 1, peerMapLen)
}

// TestReactor_TwoTunnelsSamePeer — AC-17. Two SCCRQs from the same peer
// address but with distinct Assigned Tunnel ID AVPs create two distinct
// tunnels.
//
// VALIDATES: AC-17 -- lookup key is local TID; peer addr alone does not
// disambiguate.
func TestReactor_TwoTunnelsSamePeer(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	// Single client = same source addr:port. Two SCCRQs differ only in
	// their Assigned Tunnel ID AVP, so the secondary index stores two
	// distinct entries and two tunnels result.
	client := newClient(t, ln)
	defer client.Close()

	client.Send(t, buildSCCRQ(t, 10, "peer-c"))
	waitForLog(t, logs, "local-tid=1")
	client.Send(t, buildSCCRQ(t, 20, "peer-c"))
	waitForLog(t, logs, "local-tid=2")

	require.Equal(t, 2, r.TunnelCount(), "two distinct Assigned TIDs -> two tunnels")
	r.tunnelsMu.Lock()
	peerMapLen := len(r.tunnelsByPeer)
	r.tunnelsMu.Unlock()
	require.Equal(t, 2, peerMapLen)
}

// buildHello returns a minimal Hello control message addressed to
// destTID with the given Ns/Nr. Body = single Message Type AVP.
func buildHello(t *testing.T, destTID, ns, nr uint16) []byte {
	t.Helper()
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	buf := *bodyBuf
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgHello))
	total := 12 + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), destTID, 0, ns, nr)
	copy(pkt[12:], buf[:off])
	return pkt
}

// TestReactor_RememberPeerAddrPort — AC-16. When a peer reaches us from
// a different UDP source port than the SCCRQ, the reactor updates the
// tunnel's recorded peer addr:port so subsequent outbound messages go
// to the new port. RFC 2661 S24.19.
func TestReactor_RememberPeerAddrPort(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	// First SCCRQ from client A -> creates tunnel with local-tid=1.
	clientA := newClient(t, ln)
	defer clientA.Close()
	clientA.Send(t, buildSCCRQ(t, 99, "peer-e"))
	waitForLog(t, logs, "SCCRP sent")
	tunnel := r.TunnelByLocalID(1)
	require.NotNil(t, tunnel)
	r.tunnelsMu.Lock()
	firstAddr := tunnel.peerAddr
	r.tunnelsMu.Unlock()

	// Second message (Hello, Ns=1) from client B on a different
	// ephemeral source port. Phase 3's FSM logs Hello and drops, which
	// suits this test: we only care that the reactor recorded the new
	// peerAddr.
	clientB := newClient(t, ln)
	defer clientB.Close()
	clientB.Send(t, buildHello(t, 1, 1, 0))
	waitForLog(t, logs, "Hello received")

	r.tunnelsMu.Lock()
	secondAddr := tunnel.peerAddr
	r.tunnelsMu.Unlock()
	require.NotEqual(t, firstAddr.Port(), secondAddr.Port(),
		"reactor must update peerAddr when peer uses a different source port")
}

// TestReactor_MaxTunnelsLimit — AC-18. When the configured max-tunnels
// limit is reached, subsequent SCCRQs are dropped.
//
// VALIDATES: AC-18 -- creation blocked at the limit; existing tunnels
// unaffected.
func TestReactor_MaxTunnelsLimit(t *testing.T) {
	// buildLogReactor defaults to MaxTunnels=0 (unbounded); construct a
	// reactor with MaxTunnels=1 directly.
	var buf lockedBuffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ln := NewUDPListener(netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0), logger)
	require.NoError(t, ln.Start(context.Background()))
	defer ln.Stop() //nolint:errcheck // test cleanup
	r := NewL2TPReactor(ln, logger, ReactorParams{
		MaxTunnels: 1,
		Defaults:   TunnelDefaults{HostName: "ze-test", FramingCapabilities: 0x3, RecvWindow: 16},
	})
	require.NoError(t, r.Start())
	defer r.Stop()

	send(t, ln, buildSCCRQ(t, 100, "peer-d"))
	waitForLog(t, &buf, "SCCRP sent")
	require.Equal(t, 1, r.TunnelCount())

	send(t, ln, buildSCCRQ(t, 200, "peer-d"))
	waitForLog(t, &buf, "max-tunnels limit reached")
	require.Equal(t, 1, r.TunnelCount(), "limit-reached SCCRQ must not create a tunnel")
}

// TestReactor_StopIdempotent calls Stop twice safely.
func TestReactor_StopIdempotent(t *testing.T) {
	ln, r, _, stop := buildLogReactor(t)
	defer stop()
	r.Stop()
	r.Stop()
	_ = ln // keep ln used; stop() handles the teardown
}

// buildLogReactorSecret constructs a listener + reactor pair whose
// TunnelDefaults carries the given shared secret. Mirrors buildLogReactor
// otherwise. Phase-4 auth tests call this instead of buildLogReactor.
//
//nolint:unparam // phase 4 auth tests all use the same secret value; keeping the parameter makes call sites self-documenting and leaves room for future auth-variant tests.
func buildLogReactorSecret(t *testing.T, secret string) (*UDPListener, *L2TPReactor, *lockedBuffer, func()) {
	t.Helper()
	buf := &lockedBuffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ln := NewUDPListener(netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0), logger)
	require.NoError(t, ln.Start(context.Background()))
	r := NewL2TPReactor(ln, logger, ReactorParams{
		Defaults: TunnelDefaults{HostName: "ze-test", FramingCapabilities: 0x3, RecvWindow: 16, SharedSecret: secret},
	})
	require.NoError(t, r.Start())
	stop := func() {
		r.Stop()
		_ = ln.Stop()
	}
	return ln, r, buf, stop
}

// buildSCCRQWithChallenge returns an SCCRQ datagram that includes a
// Challenge AVP (type 11) carrying the given 16-byte peer challenge.
// Otherwise identical to buildSCCRQ.
func buildSCCRQWithChallenge(t *testing.T, peerTID uint16, hostName string, peerChallenge []byte) []byte {
	t.Helper()
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	buf := *bodyBuf
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, []byte{0x01, 0x00})
	off += WriteAVPUint32(buf, off, true, AVPFramingCapabilities, 0x3)
	off += WriteAVPUint32(buf, off, true, AVPBearerCapabilities, 0x0)
	off += WriteAVPString(buf, off, true, AVPHostName, hostName)
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, peerTID)
	off += WriteAVPUint16(buf, off, true, AVPReceiveWindowSize, 8)
	off += WriteAVPBytes(buf, off, true, 0, AVPChallenge, peerChallenge)

	total := 12 + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), 0, 0, 0, 0)
	copy(pkt[12:], buf[:off])
	return pkt
}

// buildSCCRQWithTieBreaker returns an SCCRQ datagram that includes a
// Tie Breaker AVP (type 5) carrying the given 8-byte value.
func buildSCCRQWithTieBreaker(t *testing.T, peerTID uint16, hostName string, tb []byte) []byte {
	t.Helper()
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	buf := *bodyBuf
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, []byte{0x01, 0x00})
	off += WriteAVPUint32(buf, off, true, AVPFramingCapabilities, 0x3)
	off += WriteAVPUint32(buf, off, true, AVPBearerCapabilities, 0x0)
	off += WriteAVPString(buf, off, true, AVPHostName, hostName)
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, peerTID)
	off += WriteAVPUint16(buf, off, true, AVPReceiveWindowSize, 8)
	off += WriteAVPBytes(buf, off, true, 0, AVPTieBreaker, tb)

	total := 12 + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), 0, 0, 0, 0)
	copy(pkt[12:], buf[:off])
	return pkt
}

// buildSCCCN returns an SCCCN datagram for the given destination Tunnel
// ID with Ns/Nr and an optional Challenge Response AVP. localChallengeResp
// is nil when the test does not need to carry a Response.
//
//nolint:unparam // all phase-4 tests ACK a single SCCRP at Nr=1; keeping the parameter explicit leaves room for retransmit / out-of-order variants in phase 5.
func buildSCCCN(t *testing.T, destTID, ns, nr uint16, localChallengeResp []byte) []byte {
	t.Helper()
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	buf := *bodyBuf
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCCN))
	if localChallengeResp != nil {
		off += WriteAVPBytes(buf, off, true, 0, AVPChallengeResponse, localChallengeResp)
	}
	total := 12 + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), destTID, 0, ns, nr)
	copy(pkt[12:], buf[:off])
	return pkt
}

// readDatagram reads one datagram from the UDP socket with the standard
// retry loop. Returns the bytes received.
func readDatagram(t *testing.T, c *testClient) []byte {
	t.Helper()
	if err := c.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	rbuf := make([]byte, 1500)
	n, _, err := c.conn.ReadFromUDP(rbuf)
	require.NoError(t, err)
	return rbuf[:n]
}

// extractAVP walks the AVP stream in the given control-message payload
// and returns the bytes of the first AVP matching attrType, or nil if
// not present.
func extractAVP(t *testing.T, pkt []byte, attrType AVPType) []byte {
	t.Helper()
	require.GreaterOrEqual(t, len(pkt), 12, "packet too short for control header")
	body := pkt[12:]
	iter := NewAVPIterator(body)
	for {
		vendorID, at, _, value, ok := iter.Next()
		if !ok {
			require.NoError(t, iter.Err())
			return nil
		}
		if vendorID == 0 && at == attrType {
			return append([]byte(nil), value...)
		}
	}
}

// TestReactor_ChallengeResponseEmitted -- phase 4 bonus AC.
//
// VALIDATES: when peer sends SCCRQ with Challenge AVP and shared-secret
// is configured, SCCRP carries both a Challenge AVP (our 16-byte random)
// and a Challenge Response AVP whose MD5 value equals
// MD5(SCCRP_MsgType || secret || peer_challenge).
func TestReactor_ChallengeResponseEmitted(t *testing.T) {
	const secret = "topsecret"
	ln, _, _, stop := buildLogReactorSecret(t, secret)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	peerChallenge := bytes.Repeat([]byte{0xAB}, 16)
	client.Send(t, buildSCCRQWithChallenge(t, 77, "peer-auth", peerChallenge))

	sccrp := readDatagram(t, client)

	// Expected Challenge Response per RFC 2661 S4.2:
	// MD5(CHAP_ID=SCCRP || secret || peer_challenge).
	want := ChallengeResponse(ChapIDSCCRP, []byte(secret), peerChallenge)

	got := extractAVP(t, sccrp, AVPChallengeResponse)
	require.NotNil(t, got, "SCCRP must carry Challenge Response AVP")
	require.Equal(t, want[:], got, "Challenge Response bytes must match MD5(CHAP_ID||secret||peer_challenge)")

	ours := extractAVP(t, sccrp, AVPChallenge)
	require.NotNil(t, ours, "SCCRP must carry our Challenge AVP when peer challenged us")
	require.Len(t, ours, 16, "our Challenge must be 16 bytes")
}

// TestTunnelFSM_SCCCNEstablishes -- AC-8.
//
// VALIDATES: AC-8 -- the full SCCRQ -> SCCRP -> SCCCN exchange with
// matching Challenge Response drives the tunnel to established.
func TestTunnelFSM_SCCCNEstablishes(t *testing.T) {
	const secret = "topsecret"
	ln, r, logs, stop := buildLogReactorSecret(t, secret)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	peerChallenge := bytes.Repeat([]byte{0xCD}, 16)
	client.Send(t, buildSCCRQWithChallenge(t, 101, "peer-est", peerChallenge))
	sccrp := readDatagram(t, client)

	// Extract our tunnel's local TID and our Challenge so we can build
	// the matching SCCCN.
	ourChallenge := extractAVP(t, sccrp, AVPChallenge)
	require.Len(t, ourChallenge, 16)

	ourLocalTIDBytes := extractAVP(t, sccrp, AVPAssignedTunnelID)
	require.Len(t, ourLocalTIDBytes, 2)
	ourLocalTID := uint16(ourLocalTIDBytes[0])<<8 | uint16(ourLocalTIDBytes[1])

	// Compute the correct Challenge Response for SCCCN (CHAP_ID = 3).
	resp := ChallengeResponse(ChapIDSCCCN, []byte(secret), ourChallenge)

	// Peer Ns=1 (SCCRQ was Ns=0); Nr=1 (ACKs our SCCRP at Ns=0).
	client.Send(t, buildSCCCN(t, ourLocalTID, 1, 1, resp[:]))

	waitForLog(t, logs, "tunnel now established")

	tunnel := r.TunnelByLocalID(ourLocalTID)
	require.NotNil(t, tunnel)
	r.tunnelsMu.Lock()
	state := tunnel.state
	r.tunnelsMu.Unlock()
	require.Equal(t, L2TPTunnelEstablished, state)
}

// TestTunnelFSM_BadChallengeResponse_StopCCN -- AC-9.
//
// VALIDATES: AC-9 -- SCCCN with wrong Challenge Response causes the
// reactor to emit StopCCN with Result Code 4 and close the tunnel.
func TestTunnelFSM_BadChallengeResponse_StopCCN(t *testing.T) {
	const secret = "topsecret"
	ln, r, logs, stop := buildLogReactorSecret(t, secret)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	peerChallenge := bytes.Repeat([]byte{0xEF}, 16)
	client.Send(t, buildSCCRQWithChallenge(t, 202, "peer-bad", peerChallenge))
	sccrp := readDatagram(t, client)

	ourLocalTIDBytes := extractAVP(t, sccrp, AVPAssignedTunnelID)
	require.Len(t, ourLocalTIDBytes, 2)
	ourLocalTID := uint16(ourLocalTIDBytes[0])<<8 | uint16(ourLocalTIDBytes[1])

	// Send SCCCN with a DELIBERATELY WRONG Response.
	wrong := make([]byte, 16)
	client.Send(t, buildSCCCN(t, ourLocalTID, 1, 1, wrong))

	stopccn := readDatagram(t, client)

	waitForLog(t, logs, "Challenge Response did not verify")

	// StopCCN body: Message Type AVP first (value = 4 = StopCCN).
	msgType := extractAVP(t, stopccn, AVPMessageType)
	require.Len(t, msgType, 2)
	require.Equal(t, uint16(MsgStopCCN), uint16(msgType[0])<<8|uint16(msgType[1]))

	// Result Code compound AVP (first two bytes = result code = 4).
	rc := extractAVP(t, stopccn, AVPResultCode)
	require.GreaterOrEqual(t, len(rc), 2, "Result Code AVP must carry at least a 2-byte result")
	require.Equal(t, uint16(4), uint16(rc[0])<<8|uint16(rc[1]), "Result Code must be 4 (Not Authorized)")

	// Tunnel should be in closed state.
	tunnel := r.TunnelByLocalID(ourLocalTID)
	require.NotNil(t, tunnel)
	r.tunnelsMu.Lock()
	state := tunnel.state
	r.tunnelsMu.Unlock()
	require.Equal(t, L2TPTunnelClosed, state)
}

// TestTunnelFSM_TieBreakerLocalLoses -- AC-10.
//
// VALIDATES: AC-10 -- when two SCCRQs arrive from the same peer with
// Tie Breaker AVPs, the tunnel whose tie-breaker is HIGHER is discarded.
// The one with the LOWER tie-breaker survives (RFC 2661 S9.5 lower wins).
// Here, the FIRST SCCRQ has the higher value and is torn down.
func TestTunnelFSM_TieBreakerLocalLoses(t *testing.T) {
	ln, r, _, stop := buildLogReactor(t)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	// First SCCRQ with HIGH tie-breaker value.
	highTB := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	client.Send(t, buildSCCRQWithTieBreaker(t, 111, "peer-hi", highTB))
	// Consume SCCRP so the socket does not buffer it across sends.
	_ = readDatagram(t, client)

	require.Equal(t, 1, r.TunnelCount())
	firstTID := uint16(0)
	r.tunnelsMu.Lock()
	for tid := range r.tunnelsByLocalID {
		firstTID = tid
	}
	r.tunnelsMu.Unlock()

	// Second SCCRQ with LOW tie-breaker (lower wins -> first is discarded).
	lowTB := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	client.Send(t, buildSCCRQWithTieBreaker(t, 222, "peer-lo", lowTB))
	_ = readDatagram(t, client)

	// Wait until the reactor has processed the second SCCRQ and the
	// map reflects the new state.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r.TunnelCount() == 1 {
			r.tunnelsMu.Lock()
			_, firstStillThere := r.tunnelsByLocalID[firstTID]
			r.tunnelsMu.Unlock()
			if !firstStillThere {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
	}

	require.Equal(t, 1, r.TunnelCount(), "loser must be removed, winner stays")
	r.tunnelsMu.Lock()
	_, firstStillThere := r.tunnelsByLocalID[firstTID]
	r.tunnelsMu.Unlock()
	require.False(t, firstStillThere, "first tunnel (higher tie-breaker) must be discarded")
}

// TestReactor_ZeroLengthChallengeRejected -- regression for the blocker
// where a peer-sent SCCRQ carrying a header-only (value_len=0) Challenge
// AVP crashed the daemon via the ChallengeResponse panic guard.
//
// VALIDATES: parseSCCRQ rejects empty Challenge AVP at the reactor edge,
// before any tunnel state is allocated. Exploitable pre-fix; safely
// dropped post-fix.
func TestReactor_ZeroLengthChallengeRejected(t *testing.T) {
	const secret = "topsecret"
	ln, r, logs, stop := buildLogReactorSecret(t, secret)
	defer stop()

	// Build an SCCRQ whose Challenge AVP carries a zero-byte value. The
	// AVP header reports totalLen=6 (AVPHeaderLen only). Wire-legal;
	// semantically illegal per RFC 2661 S5.12 ("at least one octet").
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	buf := *bodyBuf
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, []byte{0x01, 0x00})
	off += WriteAVPUint32(buf, off, true, AVPFramingCapabilities, 0x3)
	off += WriteAVPUint32(buf, off, true, AVPBearerCapabilities, 0x0)
	off += WriteAVPString(buf, off, true, AVPHostName, "peer-empty-challenge")
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, 555)
	off += WriteAVPUint16(buf, off, true, AVPReceiveWindowSize, 8)
	off += WriteAVPEmpty(buf, off, true, 0, AVPChallenge)

	total := 12 + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), 0, 0, 0, 0)
	copy(pkt[12:], buf[:off])

	send(t, ln, pkt)
	waitForLog(t, logs, "malformed body dropped")
	require.Equal(t, 0, r.TunnelCount(), "zero-length Challenge must not create a tunnel")
}

// TestReactor_ShortTieBreakerRejected -- regression that a Tie Breaker
// AVP with the wrong length (RFC 2661 S4.4.2 fixes it at 8 bytes) is
// rejected at parse time.
//
// VALIDATES: parseSCCRQ treats non-8-byte Tie Breaker as malformed so a
// peer cannot win collisions by sending a shorter (always-lower) value.
func TestReactor_ShortTieBreakerRejected(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	buf := *bodyBuf
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, []byte{0x01, 0x00})
	off += WriteAVPUint32(buf, off, true, AVPFramingCapabilities, 0x3)
	off += WriteAVPUint32(buf, off, true, AVPBearerCapabilities, 0x0)
	off += WriteAVPString(buf, off, true, AVPHostName, "peer-short-tb")
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, 666)
	off += WriteAVPUint16(buf, off, true, AVPReceiveWindowSize, 8)
	// 1-byte Tie Breaker -- illegal per RFC.
	off += WriteAVPBytes(buf, off, true, 0, AVPTieBreaker, []byte{0x00})

	total := 12 + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), 0, 0, 0, 0)
	copy(pkt[12:], buf[:off])

	send(t, ln, pkt)
	waitForLog(t, logs, "malformed body dropped")
	require.Equal(t, 0, r.TunnelCount(), "wrong-length Tie Breaker must not create a tunnel")
}

// TestTunnelFSM_SCCCNMissingResponseWhenChallenged -- explicit coverage
// for the "!scccn.ChallengeResponsePresent" branch in handleSCCCN.
//
// VALIDATES: an SCCCN that carries NO Challenge Response AVP on a
// tunnel we challenged is treated as an auth failure (StopCCN RC=4).
func TestTunnelFSM_SCCCNMissingResponseWhenChallenged(t *testing.T) {
	const secret = "topsecret"
	ln, r, logs, stop := buildLogReactorSecret(t, secret)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	peerChallenge := bytes.Repeat([]byte{0x5A}, 16)
	client.Send(t, buildSCCRQWithChallenge(t, 707, "peer-missing-resp", peerChallenge))
	sccrp := readDatagram(t, client)

	ourLocalTIDBytes := extractAVP(t, sccrp, AVPAssignedTunnelID)
	require.Len(t, ourLocalTIDBytes, 2)
	ourLocalTID := uint16(ourLocalTIDBytes[0])<<8 | uint16(ourLocalTIDBytes[1])

	// SCCCN with NO Challenge Response AVP.
	client.Send(t, buildSCCCN(t, ourLocalTID, 1, 1, nil))

	stopccn := readDatagram(t, client)
	waitForLog(t, logs, "SCCCN missing Challenge Response")

	msgType := extractAVP(t, stopccn, AVPMessageType)
	require.Equal(t, uint16(MsgStopCCN), uint16(msgType[0])<<8|uint16(msgType[1]))

	tunnel := r.TunnelByLocalID(ourLocalTID)
	require.NotNil(t, tunnel)
	r.tunnelsMu.Lock()
	state := tunnel.state
	r.tunnelsMu.Unlock()
	require.Equal(t, L2TPTunnelClosed, state)
}

// TestTunnelFSM_SCCCNIgnoredOnEstablished -- defense-in-depth per
// handover landmine #13. A second SCCCN with a different Ns delivered
// after the tunnel is established must not re-run verification; it is
// dropped with a debug log.
//
// VALIDATES: the state != wait-ctl-conn branch of handleSCCCN drops
// cleanly without mutating state or emitting anything.
func TestTunnelFSM_SCCCNIgnoredOnEstablished(t *testing.T) {
	const secret = "topsecret"
	ln, r, logs, stop := buildLogReactorSecret(t, secret)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	// First: drive the tunnel to established via the normal handshake.
	peerChallenge := bytes.Repeat([]byte{0x3C}, 16)
	client.Send(t, buildSCCRQWithChallenge(t, 808, "peer-doubled", peerChallenge))
	sccrp := readDatagram(t, client)
	ourChallenge := extractAVP(t, sccrp, AVPChallenge)
	ourLocalTIDBytes := extractAVP(t, sccrp, AVPAssignedTunnelID)
	ourLocalTID := uint16(ourLocalTIDBytes[0])<<8 | uint16(ourLocalTIDBytes[1])
	resp := ChallengeResponse(ChapIDSCCCN, []byte(secret), ourChallenge)
	client.Send(t, buildSCCCN(t, ourLocalTID, 1, 1, resp[:]))
	waitForLog(t, logs, "tunnel now established")
	// Drain the engine's ZLB for the SCCCN.
	_ = readDatagram(t, client)

	// Second SCCCN (new Ns=2) must be delivered by the engine and dropped
	// by the FSM state check; state stays established.
	client.Send(t, buildSCCCN(t, ourLocalTID, 2, 1, resp[:]))
	waitForLog(t, logs, "SCCCN on non-wait-ctl-conn tunnel ignored")

	tunnel := r.TunnelByLocalID(ourLocalTID)
	require.NotNil(t, tunnel)
	r.tunnelsMu.Lock()
	state := tunnel.state
	r.tunnelsMu.Unlock()
	require.Equal(t, L2TPTunnelEstablished, state, "established tunnel must not revert on a duplicate-Ns SCCCN")
}

// TestTunnelFSM_SCCCNWithResponseUnchallenged -- when the peer did not
// challenge us (no Challenge AVP in SCCRQ) we emit SCCRP without our own
// Challenge. If the peer's SCCCN nevertheless carries a Challenge
// Response AVP, that AVP is ignored and the tunnel still reaches
// established (RFC: unrecognized non-mandatory AVPs silently skipped).
//
// VALIDATES: handleSCCCN's "t.ourChallenge == nil" branch accepts a
// spurious Challenge Response without rejecting the tunnel.
func TestTunnelFSM_SCCCNWithResponseUnchallenged(t *testing.T) {
	// No shared-secret -> we never challenge the peer even if it
	// challenges us. To test an "unchallenged" tunnel we use an SCCRQ
	// that does not carry a Challenge AVP (buildSCCRQ) and a secret-less
	// reactor.
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	client.Send(t, buildSCCRQ(t, 909, "peer-no-auth"))
	sccrp := readDatagram(t, client)
	ourLocalTIDBytes := extractAVP(t, sccrp, AVPAssignedTunnelID)
	ourLocalTID := uint16(ourLocalTIDBytes[0])<<8 | uint16(ourLocalTIDBytes[1])

	// SCCCN with a spurious Challenge Response AVP -- should be ignored.
	spurious := bytes.Repeat([]byte{0xAA}, 16)
	client.Send(t, buildSCCCN(t, ourLocalTID, 1, 1, spurious))

	waitForLog(t, logs, "tunnel now established")
	tunnel := r.TunnelByLocalID(ourLocalTID)
	require.NotNil(t, tunnel)
	r.tunnelsMu.Lock()
	state := tunnel.state
	r.tunnelsMu.Unlock()
	require.Equal(t, L2TPTunnelEstablished, state)
}

// TestTunnelFSM_TieBreakerEqual -- AC-11.
//
// VALIDATES: AC-11 -- bit-for-bit equal Tie Breaker values cause both
// sides to discard; no tunnel survives.
func TestTunnelFSM_TieBreakerEqual(t *testing.T) {
	ln, r, _, stop := buildLogReactor(t)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	tb := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
	client.Send(t, buildSCCRQWithTieBreaker(t, 321, "peer-a", tb))
	_ = readDatagram(t, client)
	require.Equal(t, 1, r.TunnelCount())

	// Second SCCRQ with the SAME tie-breaker; equal -> both discard.
	client.Send(t, buildSCCRQWithTieBreaker(t, 654, "peer-b", tb))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r.TunnelCount() == 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	require.Equal(t, 0, r.TunnelCount(), "equal tie-breaker must discard both tunnels")
}
