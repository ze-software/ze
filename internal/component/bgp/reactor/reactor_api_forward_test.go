package reactor

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// recordingConn is a non-blocking net.Conn whose Write appends to an in-memory
// buffer, so a test can directly observe whether (and what) a Session flushed to
// the wire. Unlike net.Pipe it needs no concurrent reader, keeping the test
// fully synchronous and deterministic.
type recordingConn struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *recordingConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(b)
}

func (c *recordingConn) written() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.buf.Bytes()...)
}

func (c *recordingConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (c *recordingConn) Close() error                     { return nil }
func (c *recordingConn) LocalAddr() net.Addr              { return testConnAddr{} }
func (c *recordingConn) RemoteAddr() net.Addr             { return testConnAddr{} }
func (c *recordingConn) SetDeadline(time.Time) error      { return nil }
func (c *recordingConn) SetReadDeadline(time.Time) error  { return nil }
func (c *recordingConn) SetWriteDeadline(time.Time) error { return nil }

type testConnAddr struct{}

func (testConnAddr) Network() string { return "test" }
func (testConnAddr) String() string  { return "10.0.0.2:179" }

// newEORGuardPeer builds an Established peer wired to an Established Session that
// is backed by a recordingConn. The AnnounceEOR ordering-guard tests then assert
// directly on the bytes flushed to the wire (an End-of-RIB or nothing), rather
// than inferring intent from an error.
func newEORGuardPeer(t *testing.T) (*reactorAPIAdapter, *Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
	})

	// Drive a Session to Established, then back it with the recording conn.
	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	adapter := &reactorAPIAdapter{r: &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}}
	return adapter, peer, conn
}

// eorWire returns the on-wire encoding of the End-of-RIB for a family.
func eorWire(fam family.Family) []byte {
	var buf [64]byte
	n := message.BuildEOR(fam).WriteTo(buf[:], 0, nil)
	return buf[:n]
}

// TestAnnounceEOR_SkipsPeerInInitialRouteSync pins the EoR-ordering guard
// (commit 99c943404): an external AnnounceEOR (plugin / route-server) must NOT
// write an End-of-RIB to a peer that is still draining its initial-route
// opQueue. sendInitialRoutes owns the per-family EoR for such a peer; sending
// here would race ahead of the still-queued route NLRI (RFC 4724 ordering).
//
// VALIDATES: while ShouldQueue()==true, AnnounceEOR succeeds but flushes nothing
// to the wire (the peer is skipped, its EoR deferred to sendInitialRoutes).
// PREVENTS: regression of the guard -- without it the EoR is flushed here,
// landing on the wire ahead of the queued routes.
func TestAnnounceEOR_SkipsPeerInInitialRouteSync(t *testing.T) {
	adapter, peer, conn := newEORGuardPeer(t)

	// Simulate "initial route sync in progress" -> ShouldQueue() == true.
	peer.sendingInitialRoutes.Store(1)
	require.True(t, peer.ShouldQueue(), "precondition: peer must be in initial-sync state")

	err := adapter.AnnounceEOR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast))
	require.NoError(t, err)
	require.Empty(t, conn.written(),
		"no End-of-RIB may reach the wire while the peer drains its initial-route queue")
}

// TestAnnounceEOR_SendsWhenNotInInitialSync confirms the skip is specific to the
// initial-sync window, not unconditional. A normally-established peer (no
// opQueue, not syncing) is sent the EoR, which lands on the wire.
//
// VALIDATES: while ShouldQueue()==false, AnnounceEOR flushes the exact End-of-RIB.
// PREVENTS: an over-broad guard that silently drops every external EoR.
func TestAnnounceEOR_SendsWhenNotInInitialSync(t *testing.T) {
	adapter, peer, conn := newEORGuardPeer(t)

	require.False(t, peer.ShouldQueue(), "precondition: peer must NOT be in initial-sync state")

	err := adapter.AnnounceEOR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast))
	require.NoError(t, err)
	require.Equal(t, eorWire(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}), conn.written(),
		"a not-syncing peer must receive the End-of-RIB on the wire")
}
