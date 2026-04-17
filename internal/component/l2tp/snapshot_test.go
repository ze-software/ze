package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newReactorForSnapshot builds a reactor with no listener, suitable for
// driving Snapshot / teardown tests without UDP. The listener pointer is
// left nil; snapshot code guards against it.
func newReactorForSnapshot(t *testing.T) *L2TPReactor {
	t.Helper()
	return &L2TPReactor{
		logger: slog.Default(),
		params: ReactorParams{
			Clock: func() time.Time { return time.Unix(1000, 0).UTC() },
			Defaults: TunnelDefaults{
				RecvWindow: 4,
			},
		},
		tunnelsByLocalID: map[uint16]*L2TPTunnel{},
		tunnelsByPeer:    map[peerKey]*L2TPTunnel{},
	}
}

// insertEstablishedTunnel wires a tunnel in the Established state with
// optional sessions, so snapshot tests can assert against a known shape.
func insertEstablishedTunnel(t *testing.T, r *L2TPReactor, localTID, remoteTID uint16, peer netip.AddrPort, sessions ...*L2TPSession) {
	t.Helper()
	tun := newTunnel(localTID, remoteTID, peer,
		ReliableConfig{MaxRetransmit: 3, RTimeout: time.Second, RTimeoutCap: 4 * time.Second, RecvWindow: 4},
		r.logger, r.params.Clock())
	tun.state = L2TPTunnelEstablished
	tun.peerHostName = "peer.example.net"
	tun.peerFraming = 0x00000003
	tun.peerBearer = 0x00000001
	tun.peerRecvWindow = 8
	for _, s := range sessions {
		tun.addSession(s)
	}
	r.tunnelsMu.Lock()
	r.tunnelsByLocalID[localTID] = tun
	r.tunnelsByPeer[peerKey{addr: peer, tid: remoteTID}] = tun
	r.tunnelsMu.Unlock()
}

// VALIDATES: AC-6, AC-7 -- Snapshot returns tunnel and session summaries
// with the fields required by `show l2tp tunnels` / `show l2tp sessions`.
// PREVENTS: CLI handlers reaching into reactor private fields.
func TestSnapshotReturnsTunnelsAndSessions(t *testing.T) {
	r := newReactorForSnapshot(t)
	peer := netip.MustParseAddrPort("10.0.0.1:1701")
	sess := &L2TPSession{
		localSID:     42,
		remoteSID:    43,
		state:        L2TPSessionEstablished,
		createdAt:    r.params.Clock(),
		username:     "alice",
		assignedAddr: netip.MustParseAddr("192.0.2.7"),
	}
	insertEstablishedTunnel(t, r, 100, 200, peer, sess)

	snap := r.Snapshot()

	require.Equal(t, 1, snap.TunnelCount)
	require.Equal(t, 1, snap.SessionCount)
	require.Len(t, snap.Tunnels, 1)
	ts := snap.Tunnels[0]
	require.Equal(t, uint16(100), ts.LocalTID)
	require.Equal(t, uint16(200), ts.RemoteTID)
	require.Equal(t, peer, ts.PeerAddr)
	require.Equal(t, "peer.example.net", ts.PeerHostName)
	require.Equal(t, "established", ts.State)
	require.Len(t, ts.Sessions, 1)
	ss := ts.Sessions[0]
	require.Equal(t, uint16(42), ss.LocalSID)
	require.Equal(t, uint16(43), ss.RemoteSID)
	require.Equal(t, "alice", ss.Username)
	require.Equal(t, "ipv4", ss.Family)
	require.Equal(t, "192.0.2.7", ss.AssignedAddr.String())
}

// VALIDATES: Snapshot output is stable (sorted by LocalTID then LocalSID).
// PREVENTS: Map-iteration order leaking into CLI output.
func TestSnapshotOrderIsDeterministic(t *testing.T) {
	r := newReactorForSnapshot(t)
	insertEstablishedTunnel(t, r, 300, 400, netip.MustParseAddrPort("10.0.0.3:1701"))
	insertEstablishedTunnel(t, r, 100, 200, netip.MustParseAddrPort("10.0.0.1:1701"))
	insertEstablishedTunnel(t, r, 200, 300, netip.MustParseAddrPort("10.0.0.2:1701"))

	snap := r.Snapshot()

	require.Len(t, snap.Tunnels, 3)
	require.Equal(t, uint16(100), snap.Tunnels[0].LocalTID)
	require.Equal(t, uint16(200), snap.Tunnels[1].LocalTID)
	require.Equal(t, uint16(300), snap.Tunnels[2].LocalTID)
}

// VALIDATES: LookupTunnel / LookupSession return the same shape as
// Snapshot entries. AC-8, AC-10.
// PREVENTS: Detail views drifting from summary.
func TestLookupTunnelAndSession(t *testing.T) {
	r := newReactorForSnapshot(t)
	peer := netip.MustParseAddrPort("10.0.0.5:1701")
	sess := &L2TPSession{
		localSID: 7,
		state:    L2TPSessionEstablished,
	}
	insertEstablishedTunnel(t, r, 11, 22, peer, sess)

	ts, ok := r.LookupTunnel(11)
	require.True(t, ok)
	require.Equal(t, uint16(11), ts.LocalTID)
	require.Len(t, ts.Sessions, 1)

	_, ok = r.LookupTunnel(99)
	require.False(t, ok, "missing tunnel must report false")

	ss, ok := r.LookupSession(7)
	require.True(t, ok)
	require.Equal(t, uint16(7), ss.LocalSID)
	require.Equal(t, uint16(11), ss.TunnelLocalTID)

	_, ok = r.LookupSession(999)
	require.False(t, ok, "missing session must report false")
}

// VALIDATES: AC-18 -- teardown of unknown IDs returns a typed error
// naming the missing ID.
func TestTeardownUnknownIDReturnsError(t *testing.T) {
	r := newReactorForSnapshot(t)

	err := r.TeardownTunnelByID(999)
	require.ErrorIs(t, err, ErrTunnelNotFound)
	require.Contains(t, err.Error(), "999")

	err = r.TeardownSessionByID(999)
	require.ErrorIs(t, err, ErrSessionNotFound)
	require.Contains(t, err.Error(), "999")
}

// VALIDATES: Boundary -- teardown rejects ID 0 (reserved by RFC 2661).
func TestTeardownRejectsZeroID(t *testing.T) {
	r := newReactorForSnapshot(t)

	require.ErrorIs(t, r.TeardownTunnelByID(0), ErrInvalidID)
	require.ErrorIs(t, r.TeardownSessionByID(0), ErrInvalidID)
}

// VALIDATES: FormatFraming renders RFC 2661 bitmap values the CLI
// expects. Async-only, sync-only, both, zero, and unknown bits.
func TestFormatFraming(t *testing.T) {
	require.Equal(t, "-", FormatFraming(0))
	require.Equal(t, "async", FormatFraming(0x00000001))
	require.Equal(t, "sync", FormatFraming(0x00000002))
	require.Equal(t, "async+sync", FormatFraming(0x00000003))
	require.Equal(t, "0x00000080", FormatFraming(0x00000080))
}
