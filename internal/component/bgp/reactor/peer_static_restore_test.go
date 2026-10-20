package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// TestExitWithdrawRestoresConfiguredRoute pins ExaBGP's withdraw_owner on the
// reactor: a withdrawal asked to restore configured routes gives back the
// configured route of a prefix the peer's static config holds, and withdraws
// the rest.
//
// VALIDATES: spec A-8. A script's route and a configured route of the same
// prefix are one entry on the wire, so the exit withdrawal of the script's
// route would leave the prefix unannounced; ExaBGP "gives back a configured
// route one of them had replaced" (rib/outgoing.py).
// PREVENTS: the exit of an ExaBGP script taking a configured route off the
// wire until the session restarts.
func TestExitWithdrawRestoresConfiguredRoute(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.settings.StaticRoutes = []StaticRoute{
		staticRouteAt("10.0.0.0/24", "10.9.9.9"),
		staticRouteAt("10.0.1.0/24", "10.9.9.9"),
	}
	peer.sendInitialRoutes()
	require.NotEmpty(t, conn.written(), "the fixture must put the configured set on the wire")

	// The script's route replaces the configured one on the wire: the same
	// prefix from another local writer, with another next hop.
	replaced := peer.sendStaticRoutes(peer.currentSession(), []StaticRoute{staticRouteAt("10.0.0.0/24", "10.8.8.8")}, false, 4096, false)
	require.Len(t, replaced, 1, "the fixture must replace the configured route on the wire")
	initial := len(conn.written())

	configured := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)
	scriptOnly := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.5.0/24"), 0)
	longer := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/25"), 0)

	rest := peer.restoreConfigured([]nlri.NLRI{configured, scriptOnly, longer})

	require.Len(t, rest, 2, "only the configured prefix leaves the withdrawal")
	assert.Equal(t, scriptOnly.String(), rest[0].String(), "a prefix the config does not hold is still withdrawn")
	assert.Equal(t, longer.String(), rest[1].String(), "a longer prefix is another route")

	frames := bgpFrames(t, conn.written()[initial:])
	require.Len(t, frames, 1, "the configured route is announced once, in place of its withdrawal")
	update := hex.EncodeToString(frames[0])
	assert.Contains(t, update, "180a0000", "the configured 10.0.0.0/24 must reach the wire")
	assert.Contains(t, update, "0a090909", "with the configured next hop 10.9.9.9")
	assert.NotContains(t, update, "180a0001", "a configured route nobody withdrew is not sent again")
}

// TestRestoreConfiguredWithoutConfigWithdrawsEverything pins the other side:
// with no static route, the withdrawal is answered whole and nothing is sent.
func TestRestoreConfiguredWithoutConfigWithdrawsEverything(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	withdrawn := []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)}

	rest := peer.restoreConfigured(withdrawn)

	assert.Len(t, rest, 1, "the withdrawal is untouched")
	assert.Empty(t, conn.written(), "nothing is given back")
}
