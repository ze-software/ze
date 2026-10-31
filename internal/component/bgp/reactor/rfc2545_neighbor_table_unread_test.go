package reactor

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/network"
)

// unreadWarning is the text every warning for an unreadable neighbor table
// carries (warnNeighborTableUnread).
const unreadWarning = "the neighbor table cannot be read"

// TestRFC2545NoNeighbourReaderWarnsOncePerPeer proves the warning for a
// neighbor table that cannot be read is given once per peer, whatever the
// number of routes: the cause is the host's (no reader in this build, no
// interface backend), so it holds for every route until fixed, and a line per
// UPDATE would bury the log while one global line in a rate window would name
// only the first peer it met. The field still goes out as the global alone,
// 16 octets, and the line names the cause's fix. A scope rebuilt from a table
// that was read clears the flag, so a later failure warns again.
//
// Method: two peers on the shared subnet each send two third-party routes
// with no neighbor table reader, and the log is counted per peer.
//
// MUTATION: rate-limit the warning in one window for every peer, as before
// (the second peer is never named), or drop the reset in
// refreshLinkScopeWith (the failure after a good read stays silent).
func TestRFC2545NoNeighbourReaderWarnsOncePerPeer(t *testing.T) {
	sink := captureReactorLog(t)
	connected := []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
	unread := neighborView{err: network.ErrNeighborTableNotRegistered}
	nlriBytes := []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00}
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8:1::/64"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(thirdPartyGlobal)),
	}

	peers := make([]*Peer, 0, 2)
	for _, address := range []string{"2001:db8:1::2", "2001:db8:1::3"} {
		peer, conn := newAnnouncePeer(t, address)
		peer.settings.LocalAddress = netip.MustParseAddr("2001:db8:1::1")
		peer.refreshLinkScopeWith(connected, unread)
		for range 2 {
			require.NoError(t, peer.SendAnnounce(route, 65000))
		}
		assert.Contains(t, string(conn.written()), string(mpReachIPv6Attr(t, nlriBytes, thirdPartyGlobal)),
			"with no neighbor table the field is the global alone, 16 octets")
		peers = append(peers, peer)
	}

	lines := unreadLines(sink.String())
	require.Len(t, lines, 2, "one warning per peer, whatever the number of routes:\n%s", sink.String())
	assert.Contains(t, lines[0], "peer=2001:db8:1::2")
	assert.Contains(t, lines[1], "peer=2001:db8:1::3")
	for _, line := range lines {
		assert.Contains(t, line, "interface component", "the line names the fix for a build with no reader")
	}

	// The table becomes readable, then unreadable again: a new failure.
	peer := peers[0]
	peer.refreshLinkScopeWith(connected, thirdPartyNeighborView(network.NeighborBindingValid, network.NeighborBindingValid))
	peer.refreshLinkScopeWith(connected, neighborView{err: errors.New("no interface backend is loaded")})
	require.NoError(t, peer.SendAnnounce(route, 65000))
	lines = unreadLines(sink.String())
	require.Len(t, lines, 3, "a failure after a good read warns again:\n%s", sink.String())
	assert.Contains(t, lines[2], "peer=2001:db8:1::2")
	assert.Contains(t, lines[2], "netlink interface backend", "the line names the fix for a missing backend")
}

// unreadLines answers the log lines that warn of an unreadable neighbor table.
func unreadLines(log string) []string {
	var lines []string
	for line := range strings.SplitSeq(log, "\n") {
		if strings.Contains(line, unreadWarning) {
			lines = append(lines, line)
		}
	}
	return lines
}
