// Design: docs/architecture/core-design.md — the peer run loop owns the peer-closed cascade
// RFC: rfc/short/rfc4760.md — multiprotocol extensions, Section 7 error handling
// Related: rfc4760_section7_test.go — the same obligation pinned on a bare Session
//
// RFC 4760 Section 7: "If a BGP speaker receives from a neighbor an UPDATE message that
// contains the MP_REACH_NLRI or MP_UNREACH_NLRI attribute, and if the speaker determines
// that the attribute is incorrect, the speaker MUST delete all the BGP routes received
// from that neighbor whose AFI/SAFI is the same as the one carried in the incorrect
// MP_REACH_NLRI or MP_UNREACH_NLRI attribute."
//
// rfc4760_section7_test.go proves the Session answers the incorrect attribute with a
// session reset and reaches Idle. Idle deletes nothing by itself: the routes live in the
// RIB plugin, and the RIB drops a neighbor's Adj-RIB-In only when it hears the peer-down
// state event. That event has one producer, the peer run loop's `from ==
// fsm.StateEstablished` branch (peer_run.go), which calls Reactor.notifyPeerClosed. The
// tests here run a whole Peer against a scripted neighbor over TCP and assert that link:
// the incorrect attribute makes the run loop raise the peer-closed notification for that
// neighbor, and a correct one does not.

package reactor

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// peerDownRecorder is the peer lifecycle observer the RIB's state feed hangs off
// (apiStateObserver in production). It records each peer the reactor reports
// established or closed. The channels are buffered so the run loop never blocks on it.
type peerDownRecorder struct {
	established chan *Peer
	closed      chan *Peer
}

func (r *peerDownRecorder) OnPeerEstablished(peer *Peer) {
	select {
	case r.established <- peer:
	default:
	}
}

func (r *peerDownRecorder) OnPeerClosed(peer *Peer, _ string) {
	select {
	case r.closed <- peer:
	default:
	}
}

// mpLinkNeighbor is the scripted neighbor's side of an Established session with a
// running Peer, plus what the Peer reported about it.
type mpLinkNeighbor struct {
	peer     *Peer
	conn     net.Conn
	events   *peerDownRecorder
	updates  chan struct{} // one value per UPDATE the Peer dispatched to its consumers
	fromPeer chan []byte   // everything the Peer wrote after the handshake, sent at EOF
}

// startMPLinkNeighbor runs a dial-only eBGP Peer (AS 65000, ASN4 and IPv4 unicast)
// against a neighbor (AS 65001) that answers the OPEN and KEEPALIVE, and returns once the
// run loop has reported the peer established.
func startMPLinkNeighbor(t *testing.T) *mpLinkNeighbor {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // test cleanup
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address must be TCP")

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			close(accepted)
			return
		}
		accepted <- conn
	}()

	reactor := New(&Config{ListenAddr: "127.0.0.1:0"})
	link := &mpLinkNeighbor{
		events:   &peerDownRecorder{established: make(chan *Peer, 4), closed: make(chan *Peer, 4)},
		updates:  make(chan struct{}, 4),
		fromPeer: make(chan []byte, 1),
	}
	reactor.addPeerObserver(link.events)

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Port = uint16(addr.Port) //nolint:gosec // a listener port fits 16 bits
	settings.Connection = ConnectionActive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65000},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}
	link.peer = NewPeer(settings)
	link.peer.SetReactor(reactor)
	// A long reconnect delay keeps the run loop from redialing inside the test window.
	link.peer.setReconnectDelay(time.Minute, time.Minute)
	link.peer.messageCallback = func(_ netip.Addr, kind msgtype.MessageType, _ []byte,
		_ *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64,
	) bool {
		if kind == msgtype.TypeUPDATE && direction == rpc.DirectionReceived {
			link.updates <- struct{}{}
		}
		return false
	}
	stop := startAndStop(t, link.peer)
	t.Cleanup(stop)

	var conn net.Conn
	select {
	case conn = <-accepted:
		require.NotNil(t, conn, "listener closed before the peer dialed")
	case <-time.After(5 * time.Second):
		t.Fatal("the peer never dialed the neighbor")
	}
	link.conn = conn
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	buf := make([]byte, 4096)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = conn.Read(buf) // the Peer's OPEN
	require.NoError(t, err)

	open := &message.Open{
		Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x02020202,
		OptionalParams: []byte{
			2, 12,
			65, 4, 0, 0, 0xFD, 0xE9, // ASN4 65001
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4 unicast
		},
	}
	_, err = conn.Write(message.PackTo(open, nil))
	require.NoError(t, err)
	_, err = conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)

	select {
	case established := <-link.events.established:
		require.Same(t, link.peer, established)
	case <-time.After(5 * time.Second):
		t.Fatal("the run loop never reported the peer established")
	}

	// Drain the Peer's writes (KEEPALIVE, initial routes, End-of-RIB, a NOTIFICATION)
	// so it never blocks on the socket, and hand the bytes over at EOF.
	go func() {
		var all []byte
		chunk := make([]byte, 4096)
		for {
			if deadlineErr := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); deadlineErr != nil {
				break
			}
			n, readErr := conn.Read(chunk)
			all = append(all, chunk[:n]...)
			if readErr != nil {
				break
			}
		}
		link.fromPeer <- all
	}()
	return link
}

// mpReachUpdate wraps attributes, among them an MP_REACH_NLRI, in an UPDATE body with no
// withdrawn routes and no body NLRI.
func mpReachUpdate(attrs []byte) []byte {
	update := make([]byte, 0, 4+len(attrs))
	update = append(update, 0x00, 0x00, byte(len(attrs)>>8), byte(len(attrs)))
	return append(update, attrs...)
}

// hasUpdateErrorNotification reports whether stream holds a NOTIFICATION with error code 3.
func hasUpdateErrorNotification(stream []byte) bool {
	for len(stream) >= message.HeaderLen {
		size := int(stream[16])<<8 | int(stream[17])
		if size < message.HeaderLen || size > len(stream) {
			return false
		}
		if stream[18] == byte(msgtype.TypeNOTIFICATION) && size > message.HeaderLen &&
			stream[message.HeaderLen] == byte(message.NotifyUpdateMessage) {
			return true
		}
		stream = stream[size:]
	}
	return false
}

// TestRFC4760IncorrectMPReachRaisesThePeerDownThatClearsTheRIB closes the link the
// Session-level tests leave open.
//
// VALIDATES: a running Peer that receives an MP_REACH_NLRI whose NLRI overruns the
// attribute reports that neighbor closed to its lifecycle observers, the feed the RIB's
// peer-down deletion listens to, after sending NOTIFICATION code 3; the UPDATE is not
// dispatched to consumers.
// PREVENTS: a run loop that takes the session to Idle without notifying, which leaves
// every route the neighbor sent installed in the RIB while each Session test stays green.
//
// RFC requirement: RFC4760-7-1 positive -- the incorrect MP_REACH_NLRI makes the peer run
// loop (peer_run.go, the from-Established branch) call Reactor.notifyPeerClosed for that
// neighbor, which is the peer-down event on which the RIB deletes the neighbor's routes.
func TestRFC4760IncorrectMPReachRaisesThePeerDownThatClearsTheRIB(t *testing.T) {
	link := startMPLinkNeighbor(t)

	incorrect := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,                   // SAFI = 1 (Unicast)
		0x04,                   // Next-hop length = 4
		0xc0, 0x00, 0x02, 0x01, // Next hop = 192.0.2.1
		0x00,             // Reserved
		0x18, 0x0a, 0x00, // NLRI: /24 needs three octets, two follow
	}
	attrs := append(firstASAttrs(4, 65001), mpAttr(14, incorrect)...)
	_, err := link.conn.Write(buildUpdateMsg(mpReachUpdate(attrs)))
	require.NoError(t, err)

	select {
	case closed := <-link.events.closed:
		require.Same(t, link.peer, closed, "the peer-down must name the neighbor that sent the attribute")
	case <-time.After(5 * time.Second):
		t.Fatal("no peer-down raised: the RIB would keep the neighbor's routes")
	}

	select {
	case stream := <-link.fromPeer:
		require.True(t, hasUpdateErrorNotification(stream),
			"the reset is announced with NOTIFICATION code 3 (UPDATE Message Error)")
	case <-time.After(10 * time.Second):
		t.Fatal("the Peer never closed the connection")
	}
	require.Empty(t, link.updates, "the incorrect UPDATE must reach no consumer")
}

// TestRFC4760CorrectMPReachRaisesNoPeerDown is the conforming side of the same link.
//
// VALIDATES: a well-formed MP_REACH_NLRI for the same AFI/SAFI is dispatched to consumers,
// and at that moment the Peer is still Established and has reported no peer-down.
// PREVENTS: a run loop that raises the peer-down on every MP attribute, which the positive
// alone would accept.
//
// RFC requirement: RFC4760-7-1 negative -- a correct MP_REACH_NLRI is not an incorrect one:
// the run loop raises no peer-down, so the RIB keeps the neighbor's routes.
func TestRFC4760CorrectMPReachRaisesNoPeerDown(t *testing.T) {
	link := startMPLinkNeighbor(t)

	correct := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,                   // SAFI = 1 (Unicast)
		0x04,                   // Next-hop length = 4
		0xc0, 0x00, 0x02, 0x01, // Next hop = 192.0.2.1
		0x00,                   // Reserved
		0x18, 0x0a, 0x00, 0x00, // NLRI: 10.0.0.0/24, three octets present
	}
	attrs := append(firstASAttrs(4, 65001), mpAttr(14, correct)...)
	_, err := link.conn.Write(buildUpdateMsg(mpReachUpdate(attrs)))
	require.NoError(t, err)

	select {
	case <-link.updates:
	case <-time.After(5 * time.Second):
		t.Fatal("the correct UPDATE never reached the consumers")
	}
	require.Empty(t, link.events.closed, "a correct MP_REACH_NLRI must raise no peer-down")
	require.Equal(t, PeerStateEstablished, link.peer.State())

	select {
	case stream := <-link.fromPeer:
		require.False(t, hasUpdateErrorNotification(stream), "no UPDATE Message Error for a correct attribute")
	default:
	}
}
