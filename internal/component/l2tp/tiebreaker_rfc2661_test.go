// RFC 2661 Section 4.4.3: a tunnel collision (both peers sent an SCCRQ carrying a Tie
// Breaker) is resolved by the lower value; the loser silently discards its tunnel, and
// equal values discard both. The producer is resolveTieBreakerLocked, driven here over a
// reactor whose tunnel maps hold Ze's own dialed tunnel to the same peer.

package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// tieBreakerReactor returns an unstarted reactor holding one wait-ctl-reply tunnel to peer
// whose Tie Breaker is tb, and the tunnel itself.
func tieBreakerReactor(t *testing.T, peer netip.AddrPort, tb []byte) (*l2tpReactor, *L2TPTunnel) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(&lockedBuffer{}, nil))
	ln := newUDPListener(netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 1701), logger)
	r := newL2TPReactor(ln, logger, reactorParams{Defaults: TunnelDefaults{HostName: "ze-test"}})
	tun := newTunnel(100, 0, peer, ReliableConfig{}, logger, time.Now())
	tun.state = L2TPTunnelWaitCtlReply
	tun.tieBreaker = tb
	r.tunnelsByLocalID[tun.localTID] = tun
	r.tunnelsByPeer[peerKey{addr: peer, tid: 0}] = tun
	return r, tun
}

// RFC requirement: RFC2661-4.4.3-3 positive — when the peer's SCCRQ carries the lower Tie
// Breaker, Ze's own tunnel to that peer is the loser: it is discarded (closed and removed
// from the tunnel maps) and the peer's SCCRQ proceeds (resolveTieBreakerLocked).
func TestTieBreakerLoserDiscardsItsTunnel(t *testing.T) {
	peer := netip.MustParseAddrPort("192.0.2.9:1701")
	r, tun := tieBreakerReactor(t, peer, []byte{9, 9, 9, 9, 9, 9, 9, 9})
	proceed, _, _ := r.resolveTieBreakerLocked(peer, []byte{1, 1, 1, 1, 1, 1, 1, 1})
	require.NotNil(t, proceed, "the peer's lower Tie Breaker wins: its SCCRQ proceeds")
	require.Equal(t, L2TPTunnelClosed, tun.state, "Ze's losing tunnel is discarded")
	require.Nil(t, r.tunnelsByLocalID[tun.localTID], "the losing tunnel leaves the local-ID map")
	require.Nil(t, r.tunnelsByPeer[peerKey{addr: peer, tid: 0}], "the losing tunnel leaves the peer map")
}

// RFC requirement: RFC2661-4.4.3-3 negative — when Ze's own tunnel carries the lower Tie
// Breaker it is the winner: it is kept as it was and the peer's SCCRQ is the one dropped
// (resolveTieBreakerLocked returns nil and discards nothing).
func TestTieBreakerWinnerKeepsItsTunnel(t *testing.T) {
	peer := netip.MustParseAddrPort("192.0.2.9:1701")
	r, tun := tieBreakerReactor(t, peer, []byte{1, 1, 1, 1, 1, 1, 1, 1})
	proceed, teardowns, downs := r.resolveTieBreakerLocked(peer, []byte{9, 9, 9, 9, 9, 9, 9, 9})
	require.Nil(t, proceed, "the peer's higher Tie Breaker loses: its SCCRQ is dropped")
	require.Empty(t, teardowns)
	require.Empty(t, downs)
	require.Equal(t, L2TPTunnelWaitCtlReply, tun.state, "Ze's winning tunnel is untouched")
	require.Same(t, tun, r.tunnelsByLocalID[tun.localTID])
}

// RFC requirement: RFC2661-4.4.3-4 positive — equal Tie Breakers on both sides discard both
// tunnels: Ze's own tunnel is closed and removed, and the peer's SCCRQ is dropped
// (resolveTieBreakerLocked returns nil after discarding the existing tunnel).
func TestTieBreakerEqualDiscardsBoth(t *testing.T) {
	peer := netip.MustParseAddrPort("192.0.2.9:1701")
	tb := []byte{5, 5, 5, 5, 5, 5, 5, 5}
	r, tun := tieBreakerReactor(t, peer, tb)
	proceed, _, _ := r.resolveTieBreakerLocked(peer, append([]byte(nil), tb...))
	require.Nil(t, proceed, "equal Tie Breakers: the peer's SCCRQ is dropped too")
	require.Equal(t, L2TPTunnelClosed, tun.state, "equal Tie Breakers: Ze's tunnel is discarded")
	require.Nil(t, r.tunnelsByLocalID[tun.localTID])
}

// RFC requirement: RFC2661-4.4.3-4 negative — the both-discard outcome needs equal values:
// with a differing Tie Breaker exactly one tunnel survives, so a collision never leaves
// the peers with no tunnel unless the values match (resolveTieBreakerLocked).
func TestTieBreakerUnequalKeepsOneTunnel(t *testing.T) {
	peer := netip.MustParseAddrPort("192.0.2.9:1701")
	r, tun := tieBreakerReactor(t, peer, []byte{5, 5, 5, 5, 5, 5, 5, 6})
	proceed, _, _ := r.resolveTieBreakerLocked(peer, []byte{5, 5, 5, 5, 5, 5, 5, 5})
	require.NotNil(t, proceed, "the lower value proceeds")
	require.Equal(t, L2TPTunnelClosed, tun.state, "only the higher value is discarded")

	r, tun = tieBreakerReactor(t, peer, []byte{5, 5, 5, 5, 5, 5, 5, 5})
	proceed, _, _ = r.resolveTieBreakerLocked(peer, []byte{5, 5, 5, 5, 5, 5, 5, 6})
	require.Nil(t, proceed, "the higher value is dropped")
	require.Equal(t, L2TPTunnelWaitCtlReply, tun.state, "the lower value survives")
}
