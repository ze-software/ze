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
