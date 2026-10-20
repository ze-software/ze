package reactor

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
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

// TestExitWithdrawRestoresConfiguredVPNAndLabeledRoutes pins A-8 for a static
// route that carries a route distinguisher or a label stack: the withdrawal
// names it by family, RD and prefix, and its label field is not part of the
// route's name.
//
// Method: configure a VPN-IPv4 route (RD 65000:1, label 100) and a labeled
// IPv4 route (label 200), then ask restoreConfigured to withdraw each with the
// one Compatibility field the update parser writes, beside a VPN route of
// another RD and a unicast route of the VPN route's prefix.
//
// VALIDATES: RFC 4364 Section 4.3.2 makes the RD part of the VPN route's NLRI,
// so a different RD is another route; RFC 8277 Section 2.4 has the receiver
// ignore the Compatibility field, so the withdrawal names the labeled route
// whatever value it holds. Both configured routes are given back.
// PREVENTS: an exiting ExaBGP script withdrawing a configured VPN or labeled
// route it had replaced.
func TestExitWithdrawRestoresConfiguredVPNAndLabeledRoutes(t *testing.T) {
	vpn := family.Family{AFI: family.AFIIPv4, SAFI: 128}
	labeled := family.Family{AFI: family.AFIIPv4, SAFI: 4}
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, vpn, labeled)

	configuredVPN := staticRouteAt("10.0.0.0/24", "10.9.9.9")
	configuredVPN.RD = "65000:1"
	configuredVPN.RDBytes = [8]byte{0x00, 0x00, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01}
	configuredVPN.Labels = []uint32{100}
	configuredLabeled := staticRouteAt("10.0.2.0/24", "10.9.9.9")
	configuredLabeled.Labels = []uint32{200}
	peer.settings.StaticRoutes = []StaticRoute{configuredVPN, configuredLabeled}
	peer.sendInitialRoutes()
	require.NotEmpty(t, conn.written(), "the fixture must put the configured set on the wire")

	// The script's routes replace the configured ones on the wire: the same
	// routes from another local writer, with another next hop. A route the wire
	// already carries unchanged is not sent again, so without the replacement
	// the give-back would put nothing on the wire.
	scriptVPN, scriptLabeled := configuredVPN, configuredLabeled
	scriptVPN.NextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.8.8.8"))
	scriptLabeled.NextHop = scriptVPN.NextHop
	replaced := peer.sendStaticRoutes(peer.currentSession(), []StaticRoute{scriptVPN, scriptLabeled}, false, 4096, false)
	require.Len(t, replaced, 2, "the fixture must replace the configured routes on the wire")
	initial := len(conn.written())

	// Length in bits, the Compatibility field, then the RD for a VPN route,
	// then the prefix octets: the withdrawn form the update parser writes.
	wire := func(fam family.Family, hexNLRI string) nlri.NLRI {
		t.Helper()
		raw, err := hex.DecodeString(hexNLRI)
		require.NoError(t, err)
		n, err := nlri.NewWireNLRI(fam, raw, false)
		require.NoError(t, err)
		return n
	}
	sameVPN := wire(vpn, "70"+"800000"+"0000fde800000001"+"0a0000")
	otherRD := wire(vpn, "70"+"800000"+"0000fde800000002"+"0a0000")
	sameLabeled := wire(labeled, "30"+"000641"+"0a0002")
	unicast := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)

	rest := peer.restoreConfigured([]nlri.NLRI{sameVPN, otherRD, sameLabeled, unicast})

	require.Len(t, rest, 2, "only the two configured routes leave the withdrawal")
	assert.Equal(t, otherRD.String(), rest[0].String(), "another RD names another route")
	assert.Equal(t, unicast.String(), rest[1].String(), "the unicast route of the VPN prefix is another family")

	given := hex.EncodeToString(bytes.Join(bgpFrames(t, conn.written()[initial:]), nil))
	assert.Contains(t, given, "0000fde800000001"+"0a0000", "the configured VPN route must reach the wire with its RD")
	assert.Contains(t, given, "000c81"+"0a0002", "the configured labeled route must reach the wire with its label 200")
	assert.Contains(t, given, "0a090909", "with the configured next hop 10.9.9.9")
}
