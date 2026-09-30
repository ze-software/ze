// Design: docs/architecture/wire/capabilities.md -- capabilities Ze advertises in its OPEN
// RFC: rfc/short/rfc4760.md -- Multiprotocol capability per exchanged <AFI, SAFI> (Section 8)
// RFC: rfc/short/rfc4684.md -- Route Target membership advertised as a Multiprotocol pair (Section 5)
// RFC: rfc/short/rfc7911.md -- one ADD-PATH capability instance for every AFI/SAFI (Section 4)
//
// These tests read the OPEN Ze itself builds from a peer's config
// (parsePeerFromTree, then Session.buildOpen, then the optional parameters
// parsed back), which is what reaches the wire. The unit tests in
// internal/core/bgp/capability prove the codecs; these prove Ze's own
// advertisement.
//
// VALIDATES: Ze advertises a Multiprotocol capability for every family it
// exchanges, and folds every ADD-PATH family into one capability instance.
// PREVENTS: a family exchanged without its advertisement, and one ADD-PATH
// capability per family.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

var rtcFamily = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIRTC}

// openCapabilitiesFromConfig builds the peer settings from a session tree
// holding families and, when addPath is non-nil, capability add-path, then
// returns the capabilities of the OPEN Ze would send. Each family entry gets a
// prefix maximum, which config requires.
func openCapabilitiesFromConfig(t *testing.T, families, addPath map[string]any) []capability.Capability {
	t.Helper()
	for _, entry := range families {
		if settings, ok := entry.(map[string]any); ok {
			settings["prefix"] = map[string]any{"maximum": "100000"} // Config refuses a family without one.
		}
	}
	session := map[string]any{
		"asn":    map[string]any{"remote": "65001"},
		"family": families,
	}
	if addPath != nil {
		session["capability"] = map[string]any{"add-path": addPath}
	}
	tree := map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}, "local": map[string]any{"ip": "auto"}},
		"session":    session,
	}
	settings, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)
	open, err := NewSession(settings).buildOpen(settings, settings.Capabilities)
	require.NoError(t, err)
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	require.NoError(t, err)
	return caps
}

// multiprotocolFamilies answers the <AFI, SAFI> pairs the OPEN's Multiprotocol
// capabilities advertise, in wire order.
func multiprotocolFamilies(caps []capability.Capability) []family.Family {
	var families []family.Family
	for _, c := range caps {
		if mp, ok := c.(*capability.Multiprotocol); ok {
			families = append(families, family.Family{AFI: mp.AFI, SAFI: mp.SAFI})
		}
	}
	return families
}

// peerOffering answers a peer OPEN's capabilities advertising every family.
func peerOffering(families ...family.Family) []capability.Capability {
	caps := make([]capability.Capability, 0, len(families))
	for _, f := range families {
		caps = append(caps, &capability.Multiprotocol{AFI: f.AFI, SAFI: f.SAFI})
	}
	return caps
}

var testPeerIdentity = capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001}

// TestRFC4760OpenAdvertisesEveryConfiguredFamily configures three families and
// reads Ze's OPEN: each carries its own Multiprotocol capability, and the
// negotiation with a peer that offers the same three makes all three usable.
//
// RFC requirement: RFC4760-8-1 positive -- for every family configured on a peer (ipv4/unicast, ipv6/unicast, l2vpn/evpn) the OPEN Ze builds carries exactly one Multiprotocol capability with that AFI/SAFI, and Negotiate against a peer advertising the same three marks each supported.
func TestRFC4760OpenAdvertisesEveryConfiguredFamily(t *testing.T) {
	t.Parallel()
	evpn := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	caps := openCapabilitiesFromConfig(t, map[string]any{
		"ipv4/unicast": map[string]any{},
		"ipv6/unicast": map[string]any{},
		"l2vpn/evpn":   map[string]any{},
	}, nil)
	want := []family.Family{family.IPv4Unicast, family.IPv6Unicast, evpn}
	require.ElementsMatch(t, want, multiprotocolFamilies(caps))

	neg := capability.Negotiate(caps, peerOffering(want...), testPeerIdentity)
	for _, f := range want {
		require.True(t, neg.SupportsFamily(f), "%s advertised by both OPENs", f)
	}
}

// TestRFC4760FamilyNotAdvertisedIsNotExchanged configures ipv6/unicast with
// mode disable beside ipv4/unicast: Ze's OPEN carries no Multiprotocol
// capability for it, and even a peer that offers ipv6/unicast does not get the
// family negotiated, so no ipv6/unicast route is exchanged.
//
// RFC requirement: RFC4760-8-1 negative -- a family configured with mode disable gets no Multiprotocol capability in Ze's OPEN (only ipv4/unicast is advertised), and Negotiate against a peer advertising both leaves the un-advertised ipv6/unicast unsupported while ipv4/unicast stays supported.
func TestRFC4760FamilyNotAdvertisedIsNotExchanged(t *testing.T) {
	t.Parallel()
	caps := openCapabilitiesFromConfig(t, map[string]any{
		"ipv4/unicast": map[string]any{},
		"ipv6/unicast": map[string]any{"mode": "disable"},
	}, nil)
	require.Equal(t, []family.Family{family.IPv4Unicast}, multiprotocolFamilies(caps))

	neg := capability.Negotiate(caps, peerOffering(family.IPv4Unicast, family.IPv6Unicast), testPeerIdentity)
	require.True(t, neg.SupportsFamily(family.IPv4Unicast))
	require.False(t, neg.SupportsFamily(family.IPv6Unicast), "Ze never advertised ipv6/unicast")
}

// TestRFC4684RTCAdvertisedAsMultiprotocolPair configures the Route Target
// membership family and reads Ze's OPEN for the (1, 132) Multiprotocol pair;
// with the family disabled the pair is absent and never negotiated.
//
// RFC requirement: RFC4684-5-2 positive -- a peer configured with ipv4/rtc gets a Multiprotocol capability AFI 1 SAFI 132 in the OPEN Ze builds, and Negotiate against a peer advertising (1, 132) marks the family supported.
// RFC requirement: RFC4684-5-2 negative -- with ipv4/rtc configured mode disable, Ze's OPEN carries no (1, 132) Multiprotocol capability and Negotiate against a peer advertising (1, 132) leaves Route Target membership unsupported.
func TestRFC4684RTCAdvertisedAsMultiprotocolPair(t *testing.T) {
	t.Parallel()
	caps := openCapabilitiesFromConfig(t, map[string]any{
		"ipv4/unicast": map[string]any{},
		"ipv4/rtc":     map[string]any{},
	}, nil)
	require.Contains(t, multiprotocolFamilies(caps), rtcFamily)
	neg := capability.Negotiate(caps, peerOffering(family.IPv4Unicast, rtcFamily), testPeerIdentity)
	require.True(t, neg.SupportsFamily(rtcFamily))

	caps = openCapabilitiesFromConfig(t, map[string]any{
		"ipv4/unicast": map[string]any{},
		"ipv4/rtc":     map[string]any{"mode": "disable"},
	}, nil)
	require.NotContains(t, multiprotocolFamilies(caps), rtcFamily)
	neg = capability.Negotiate(caps, peerOffering(family.IPv4Unicast, rtcFamily), testPeerIdentity)
	require.False(t, neg.SupportsFamily(rtcFamily))
}

// addPathInstances answers every ADD-PATH capability in the OPEN.
func addPathInstances(caps []capability.Capability) []*capability.AddPath {
	var instances []*capability.AddPath
	for _, c := range caps {
		if ap, ok := c.(*capability.AddPath); ok {
			instances = append(instances, ap)
		}
	}
	return instances
}

// addPathFamilyModes answers the per-family Send/Receive values of one instance.
func addPathFamilyModes(ap *capability.AddPath) map[family.Family]capability.AddPathMode {
	modes := make(map[family.Family]capability.AddPathMode, len(ap.Families))
	for _, f := range ap.Families {
		modes[family.Family{AFI: f.AFI, SAFI: f.SAFI}] = f.Mode
	}
	return modes
}

// TestRFC7911OpenCarriesOneAddPathInstance configures ADD-PATH with a default
// direction over two families and reads Ze's OPEN: exactly one ADD-PATH
// capability carries both.
//
// RFC requirement: RFC7911-4-1 positive -- with add-path direction send/receive over ipv4/unicast and ipv6/unicast, the OPEN Ze builds carries exactly one ADD-PATH capability (code 69) and that one instance lists both families with Send/Receive 3.
func TestRFC7911OpenCarriesOneAddPathInstance(t *testing.T) {
	t.Parallel()
	caps := openCapabilitiesFromConfig(t, map[string]any{
		"ipv4/unicast": map[string]any{},
		"ipv6/unicast": map[string]any{},
	}, map[string]any{"direction": "send/receive"})
	instances := addPathInstances(caps)
	require.Len(t, instances, 1, "one ADD-PATH capability for every family")
	require.Equal(t, map[family.Family]capability.AddPathMode{
		family.IPv4Unicast: capability.AddPathBoth,
		family.IPv6Unicast: capability.AddPathBoth,
	}, addPathFamilyModes(instances[0]))
}

// TestRFC7911PerFamilyAddPathStaysOneInstance drives the input most likely to
// produce one instance per family: no default direction, and a different
// per-family direction on each of three families. Ze's OPEN still carries one
// ADD-PATH capability holding all three.
//
// RFC requirement: RFC7911-4-1 negative -- with only per-family add-path overrides (ipv4/unicast send, ipv6/unicast receive, l2vpn/evpn send/receive) the OPEN Ze builds still carries exactly one ADD-PATH capability, which lists all three families with their own Send/Receive values (2, 1, 3).
func TestRFC7911PerFamilyAddPathStaysOneInstance(t *testing.T) {
	t.Parallel()
	evpn := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	caps := openCapabilitiesFromConfig(t, map[string]any{
		"ipv4/unicast": map[string]any{},
		"ipv6/unicast": map[string]any{},
		"l2vpn/evpn":   map[string]any{},
	}, map[string]any{"family": map[string]any{
		"ipv4/unicast": map[string]any{"direction": "send"},
		"ipv6/unicast": map[string]any{"direction": "receive"},
		"l2vpn/evpn":   map[string]any{"direction": "send/receive"},
	}})
	instances := addPathInstances(caps)
	require.Len(t, instances, 1, "per-family overrides must not split the capability")
	require.Equal(t, map[family.Family]capability.AddPathMode{
		family.IPv4Unicast: capability.AddPathSend,
		family.IPv6Unicast: capability.AddPathReceive,
		evpn:               capability.AddPathBoth,
	}, addPathFamilyModes(instances[0]))
}
