package l2tp

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// buildZeroTIDReactor builds a reactor whose clock this test owns, so the
// StopCCN rate limiter can be driven across its interval without sleeping.
// testClock, its constructor and buildLogReactorWithClock all live in
// reactor_test.go; only the HELLO interval and the empty shared secret are
// chosen here, and neither reaches the path under test.
func buildZeroTIDReactor(t *testing.T, clk *testClock) (*UDPListener, *L2TPReactor, *lockedBuffer, func()) {
	t.Helper()
	ln, r, logs, stop := buildLogReactorWithClock(t, clk, 60*time.Second, "")
	require.NotZero(t, ln.Addr().Port(), "the listener is bound before the first send")
	require.Zero(t, r.TunnelCount(), "a fresh reactor holds no tunnel")
	return ln, r, logs, stop
}

// recvControl reads one datagram the reactor sent back to this client, or
// reports that none arrived within d.
func recvControl(t *testing.T, c *testClient, d time.Duration) ([]byte, bool) {
	t.Helper()
	require.NoError(t, c.conn.SetReadDeadline(time.Now().Add(d)))
	buf := make([]byte, 1500)
	n, _, err := c.conn.ReadFromUDP(buf)
	if err != nil {
		if ne, ok := asNetError(err); ok && ne.Timeout() {
			return nil, false
		}
		t.Fatalf("reading reply: %v", err)
	}
	require.NotZero(t, n, "a reply datagram carries bytes")
	return buf[:n], true
}

// asNetError is errors.As specialised to net.Error, kept local so the
// helper above reads as one line. A read deadline expiring is how this
// file spells "no datagram arrived"; any other error is a test bug.
func asNetError(err error) (net.Error, bool) {
	var ne net.Error
	ok := errors.As(err, &ne)
	if ok {
		return ne, true
	}
	return nil, false
}

// mustSilence asserts that no datagram reaches this client within
// silenceWait, and names the reply size when one arrives anyway.
func mustSilence(t *testing.T, c *testClient, why string) {
	t.Helper()
	reply, got := recvControl(t, c, silenceWait)
	require.Falsef(t, got, "%s: unexpected %d-byte reply", why, len(reply))
}

// The wait a test gives a reply that MUST arrive. Generous, because a
// loaded CI box schedules the reactor goroutine late; a real regression
// fails at the assertion, not at the deadline.
const replyWait = 2 * time.Second

// The wait a test gives a reply that MUST NOT arrive. Short, because the
// emission happens on the same goroutine turn as the parse: if it were
// going to come, it would already be here.
const silenceWait = 250 * time.Millisecond

// RFC requirement: RFC2661-24.10-1 negative -- an SCCRQ whose Assigned Tunnel ID
// AVP carries 0 is a protocol error. Ze answers it with a StopCCN carrying
// Result Code 2 and Error Code 3, and creates no tunnel. The two tags that
// existed before this test both drive the SCCRP half of the same requirement.
//
// VALIDATES: AC-2 -- the reply is emitted, and AC-6 -- nothing is allocated.
func TestSCCRQWithZeroAssignedTunnelIDIsAnswered(t *testing.T) {
	clk := newTestClock(time.Unix(1_700_000_000, 0))
	ln, r, _, stop := buildZeroTIDReactor(t, clk)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	client.Send(t, buildSCCRQ(t, 0, "zero-tid-peer"))

	reply, got := recvControl(t, client, replyWait)
	require.True(t, got, "a zero Assigned Tunnel ID SCCRQ must be answered, not dropped in silence")

	hdr, err := ParseMessageHeader(reply)
	require.NoError(t, err)
	require.True(t, hdr.IsControl)
	// RFC 2661 Section 4.4.3: with no Assigned Tunnel ID from the peer, the
	// header MUST carry Tunnel ID 0.
	require.EqualValues(t, 0, hdr.TunnelID)
	require.EqualValues(t, 0, hdr.Ns, "first control message ze sends on this connection")
	require.EqualValues(t, 1, hdr.Nr, "acknowledges the SCCRQ's Ns=0")

	info, err := parseStopCCN(reply[hdr.PayloadOff:hdr.Length])
	require.NoError(t, err)
	require.EqualValues(t, resultProtocolError, info.Result, "Result Code 2")
	require.EqualValues(t, errorValueOutOfRange, info.Error, "Error Code 3")
	require.NotZero(t, info.AssignedTunnelID, "RFC 2661 Section 4.4.3: the AVP is a non-zero integer")
	require.EqualValues(t, tidNoTunnel, info.AssignedTunnelID)

	require.Equal(t, 0, r.TunnelCount(), "the refusal allocates no tunnel")
	r.tunnelsMu.Lock()
	peerMapLen := len(r.tunnelsByPeer)
	r.tunnelsMu.Unlock()
	require.Equal(t, 0, peerMapLen)
}

// RFC requirement: RFC2661-24.10-1 positive -- an SCCRQ carrying a non-zero
// Assigned Tunnel ID is accepted on the same path: ze answers SCCRP, adopts the
// peer's tunnel ID, and sends no StopCCN.
//
// VALIDATES: the refusal is keyed on the zero value and nothing else, so the
// negative polarity above cannot pass by refusing every SCCRQ.
func TestSCCRQWithNonZeroAssignedTunnelIDEstablishes(t *testing.T) {
	clk := newTestClock(time.Unix(1_700_000_000, 0))
	ln, r, _, stop := buildZeroTIDReactor(t, clk)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	client.Send(t, buildSCCRQ(t, 4242, "good-peer"))

	reply, got := recvControl(t, client, replyWait)
	require.True(t, got, "a well-formed SCCRQ must be answered")

	hdr, err := ParseMessageHeader(reply)
	require.NoError(t, err)
	require.EqualValues(t, 4242, hdr.TunnelID, "the reply is addressed with the peer's Assigned Tunnel ID")
	require.EqualValues(t, MsgSCCRP, extractMsgType(reply[hdr.PayloadOff:hdr.Length]),
		"a non-zero Assigned Tunnel ID gets SCCRP, never StopCCN")

	require.Equal(t, 1, r.TunnelCount())
}

// VALIDATES: AC-6 -- a flood of zero-Assigned-Tunnel-ID SCCRQs from one source
// grows neither tunnel map, and the per-source rate limit holds the reply count
// to one per interval.
// PREVENTS: the reply path becoming the allocation path the parse-before-lock
// ordering exists to prevent, and ze becoming an unbounded reflector.
func TestZeroIDSCCRQFloodAllocatesNoTunnel(t *testing.T) {
	clk := newTestClock(time.Unix(1_700_000_000, 0))
	ln, r, _, stop := buildZeroTIDReactor(t, clk)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	const floodSize = 50
	for range floodSize {
		client.Send(t, buildSCCRQ(t, 0, "flood-peer"))
	}

	// The first is answered; the rest are refused in silence while the
	// clock stands still.
	_, got := recvControl(t, client, replyWait)
	require.True(t, got, "the first of the flood is answered")
	extra := 0
	for {
		if _, more := recvControl(t, client, silenceWait); !more {
			break
		}
		extra++
		require.Less(t, extra, floodSize, "the rate limit must stop the flood being reflected")
	}
	require.Zero(t, extra, "one StopCCN per source per interval")
	mustSilence(t, client, "the flood stays suppressed after the drain")

	require.Equal(t, 0, r.TunnelCount())
	r.tunnelsMu.Lock()
	peerMapLen := len(r.tunnelsByPeer)
	r.tunnelsMu.Unlock()
	require.Equal(t, 0, peerMapLen)
}

// VALIDATES: the per-source rate limit is a time window, not a one-shot latch:
// the source is answered again once stopCCNLimitInterval has passed.
func TestZeroIDSCCRQAnsweredAgainAfterInterval(t *testing.T) {
	clk := newTestClock(time.Unix(1_700_000_000, 0))
	ln, _, _, stop := buildZeroTIDReactor(t, clk)
	defer stop()

	client := newClient(t, ln)
	defer client.Close()

	client.Send(t, buildSCCRQ(t, 0, "peer"))
	_, got := recvControl(t, client, replyWait)
	require.True(t, got, "first refusal is answered")

	// One nanosecond below the interval: still refused.
	clk.add(stopCCNLimitInterval - time.Nanosecond)
	client.Send(t, buildSCCRQ(t, 0, "peer"))
	_, got = recvControl(t, client, silenceWait)
	require.False(t, got, "below the interval the reply is suppressed")

	// On the interval: answered again.
	clk.add(time.Nanosecond)
	client.Send(t, buildSCCRQ(t, 0, "peer"))
	_, got = recvControl(t, client, replyWait)
	require.True(t, got, "at the interval the source is answered again")
}

// VALIDATES: the bound carries no exception. A source that owns a live tunnel
// is answered once per stopCCNLimitInterval like every other source, so the
// ceiling holds for every address: one StopCCN per victim per interval.
// PREVENTS: an exemption keyed on "this address owns a tunnel". Nothing at this
// point in the exchange proves return-routability, so such an exemption answers
// an attacker who spoofs a current peer's address at his own packet rate, aimed
// at that peer.
func TestZeroIDSCCRQFromTunnelOwnerIsBounded(t *testing.T) {
	clk := newTestClock(time.Unix(1_700_000_000, 0))
	ln, r, logs, stop := buildZeroTIDReactor(t, clk)
	defer stop()

	// A first client establishes a tunnel from 127.0.0.1.
	established := newClient(t, ln)
	defer established.Close()
	established.Send(t, buildSCCRQ(t, 77, "tunnel-owner"))
	waitForLog(t, logs, "SCCRP sent; tunnel now wait-ctl-conn")
	require.Equal(t, 1, r.TunnelCount())

	// A second client on the SAME address -- what a spoofer forges -- sends
	// two zero-ID SCCRQs inside one interval. The first is answered, the
	// second is refused in silence.
	client := newClient(t, ln)
	defer client.Close()

	client.Send(t, buildSCCRQ(t, 0, "zero-tid"))
	_, got := recvControl(t, client, replyWait)
	require.True(t, got, "the first zero-ID SCCRQ of the interval is answered")

	client.Send(t, buildSCCRQ(t, 0, "zero-tid"))
	mustSilence(t, client, "owning a tunnel buys no second reply inside the interval")

	// The bound is a rate, not a ban: the next interval answers it again.
	clk.add(stopCCNLimitInterval)
	client.Send(t, buildSCCRQ(t, 0, "zero-tid"))
	_, got = recvControl(t, client, replyWait)
	require.True(t, got, "the next interval answers the same source again")

	require.Equal(t, 1, r.TunnelCount(), "the refusals allocate nothing")
}

// VALIDATES: the tunnel-free StopCCN reaches both capture rings, so `ze diag
// l2tp` shows the reply beside the SCCRQ that drew it.
// PREVENTS: an emission that leaves on the wire and is invisible to the
// diagnostics every other outbound control message appears in.
func TestUnassociatedStopCCNIsCaptured(t *testing.T) {
	clk := newTestClock(time.Unix(1_700_000_000, 0))
	logger := slog.New(slog.NewTextHandler(&lockedBuffer{}, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ln := newUDPListener(netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0), logger)
	require.NoError(t, ln.Start(context.Background()))
	defer func() { _ = ln.Stop() }()

	// The reactor is deliberately NOT started: this test calls the emission
	// directly, so the capture ring is written and read on one goroutine and
	// the assertions need no synchronization.
	r := newL2TPReactor(ln, logger, reactorParams{Clock: clk.now})
	r.EnableCapture()
	r.EnableRawCapture()

	client := newClient(t, ln)
	defer client.Close()
	to := netip.MustParseAddrPort(client.conn.LocalAddr().String())

	r.answerZeroTunnelIDSCCRQ(to, 0)

	wire, got := recvControl(t, client, replyWait)
	require.True(t, got, "the StopCCN is on the wire")

	entries := r.CaptureSnapshot(10, 0, "")
	require.Len(t, entries, 1, "one outbound control message is recorded")
	require.Equal(t, "out", entries[0].Direction)
	require.Equal(t, MsgStopCCN.String(), entries[0].MsgType)
	require.Equal(t, to.String(), entries[0].PeerAddr)
	require.Equal(t, len(wire), entries[0].ByteCount, "the recorded size is the size sent")

	raw := r.RawCaptureSnapshot(10)
	require.Len(t, raw, 1, "the pcap ring holds the same datagram")
	require.Equal(t, wire, raw[0].Data)
}

// VALIDATES: the Assigned Tunnel ID a tunnel-free StopCCN carries is never
// handed to a real tunnel, so a peer answering one cannot reach a stranger's
// tunnel.
// PREVENTS: allocateLocalTID returning tidNoTunnel after wrap-around.
func TestAllocateLocalTIDSkipsTidNoTunnel(t *testing.T) {
	r := &L2TPReactor{
		logger:           slog.Default(),
		tunnelsByLocalID: make(map[uint16]*L2TPTunnel),
		tunnelsByPeer:    make(map[peerKey]*L2TPTunnel),
	}

	// Two below the reserved value: the next allocation is the last usable
	// ID, and the one after it must step over tidNoTunnel.
	r.nextLocalTID = tidNoTunnel - 2
	last, err := r.allocateLocalTID()
	require.NoError(t, err)
	require.EqualValues(t, tidNoTunnel-1, last, "the ID just below the reserved one is usable")

	wrapped, err := r.allocateLocalTID()
	require.NoError(t, err)
	require.NotEqual(t, tidNoTunnel, wrapped, "the reserved ID is never handed out")
	require.NotZero(t, wrapped, "and neither is 0")
	require.EqualValues(t, 1, wrapped, "allocation wraps to 1 rather than returning the reserved ID")
}

// VALIDATES: stopCCNSlot lands inside the table for every address shape, so an
// attacker-chosen address cannot index out of it.
func TestStopCCNSlotInRange(t *testing.T) {
	for _, s := range []string{
		"0.0.0.0", "127.0.0.1", "255.255.255.255", "10.0.0.1",
		"::", "::1", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "2001:db8::1",
	} {
		slot := stopCCNSlot(netip.MustParseAddr(s))
		require.GreaterOrEqual(t, slot, 0, s)
		require.Less(t, slot, stopCCNLimitSlots, s)
	}
}
