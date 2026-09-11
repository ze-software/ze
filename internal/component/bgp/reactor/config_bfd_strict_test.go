package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// bfdStrictTree builds the smallest bgp config tree carrying one peer whose
// connection block holds the bfd container in bfd.
func bfdStrictTree(bfd map[string]any) map[string]any {
	connection := map[string]any{
		"remote": map[string]any{"ip": "192.0.2.1"},
		"local":  map[string]any{"ip": "auto"},
	}
	if bfd != nil {
		connection["bfd"] = bfd
	}
	return map[string]any{
		"router-id": "10.0.0.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"peer": map[string]any{
			"peer1": map[string]any{
				"connection": connection,
				"session":    map[string]any{"asn": map[string]any{"remote": "65001"}},
			},
		},
	}
}

func peerAdvertisesBFDStrict(t *testing.T, settings *PeerSettings) bool {
	t.Helper()
	for _, c := range settings.Capabilities {
		if c.Code() == capability.CodeBFDStrictMode {
			return true
		}
	}
	return false
}

// TestBFDSettingsStrictParse is the config end of
// draft-ietf-idr-bgp-bfd-strict-mode Section 6: "A BGP speaker which supports
// capabilities advertisement and has BFD strict-mode enabled MUST include the
// BFD Strict-Mode Capability in its OPEN message."
//
// VALIDATES: `strict true` reaches BFDSettings.Strict AND puts capability 74 in
// the peer's advertised set; the hold-time leaf reaches BFDSettings.HoldTime.
//
// PREVENTS: The two halves diverging. A Strict flag with no capability makes ze
// wait for a peer it never told, and a capability with no flag makes ze promise
// a wait it does not perform.
//
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1 positive -- a peer
// with BFD strict-mode enabled carries the BFD Strict-Mode Capability in the set
// ze puts in its OPEN: parsePeerFromTree appends capability.BFDStrictMode to
// PeerSettings.Capabilities, which is what sendOpen encodes
// (internal/component/bgp/reactor/config.go, parsePeerFromTree).
func TestBFDSettingsStrictParse(t *testing.T) {
	peers, err := PeersFromTree(bfdStrictTree(map[string]any{
		"strict":    "true",
		"hold-time": "45",
	}))
	require.NoError(t, err)
	require.Len(t, peers, 1)

	require.NotNil(t, peers[0].BFD)
	require.True(t, peers[0].BFD.Strict)
	require.Equal(t, uint16(45), peers[0].BFD.HoldTime)
	require.True(t, peerAdvertisesBFDStrict(t, peers[0]),
		"Section 6 makes the advertisement a MUST for a speaker with strict-mode enabled")
}

// TestBFDSettingsStrictAbsentAdvertisesNothing is the negative half of Section
// 6, and it is what keeps every existing BFD peer on the behavior it has.
//
// VALIDATES: A bfd block without `strict`, and a peer with no bfd block at all,
// leave Strict false and advertise no capability 74.
//
// PREVENTS: Every BFD peer in the field starting to advertise a capability, and
// so starting to wait, because one leaf gained a default.
func TestBFDSettingsStrictAbsentAdvertisesNothing(t *testing.T) {
	withBFD, err := PeersFromTree(bfdStrictTree(map[string]any{"mode": "multi-hop", "min-ttl": "250"}))
	require.NoError(t, err)
	require.NotNil(t, withBFD[0].BFD)
	require.False(t, withBFD[0].BFD.Strict)
	require.False(t, peerAdvertisesBFDStrict(t, withBFD[0]))

	withoutBFD, err := PeersFromTree(bfdStrictTree(nil))
	require.NoError(t, err)
	require.Nil(t, withoutBFD[0].BFD)
	require.False(t, peerAdvertisesBFDStrict(t, withoutBFD[0]))
}

// TestBFDSettingsStrictDisabledAdvertisesNothing reads Section 3, attribute 17
// (BfdStrictEnabled): "If BfdEnabled is not TRUE for this BGP session, this
// attribute has no impact."
//
// VALIDATES: `strict true` under `enabled false` advertises nothing.
//
// PREVENTS: A peer suspended for maintenance still promising the peer that it
// runs the strict-mode procedures, which would hold the far end down against a
// speaker that has switched BFD off.
//
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1 negative -- the MUST
// binds a speaker that "has BFD strict-mode enabled", and this peer does not:
// its bfd block is disabled, which Section 3 attribute 17 says leaves
// BfdStrictEnabled with no impact. No capability is advertised, so the
// obligation's condition is what gates the advertisement rather than the strict
// leaf alone (internal/component/bgp/reactor/config.go, parsePeerFromTree).
func TestBFDSettingsStrictDisabledAdvertisesNothing(t *testing.T) {
	peers, err := PeersFromTree(bfdStrictTree(map[string]any{
		"enabled": "false",
		"strict":  "true",
	}))
	require.NoError(t, err)
	require.True(t, peers[0].BFD.Strict, "the configured value is preserved")
	require.False(t, peers[0].BFD.Enabled)
	require.False(t, peerAdvertisesBFDStrict(t, peers[0]))
}

// TestBFDSettingsHoldTimeBounds is the boundary case for the numeric leaf.
//
// VALIDATES: 1 and 65535 are accepted, 0 and 65536 are refused with the peer
// named, and an absent leaf leaves the field zero so the timer takes the
// draft's own 30-second default (fsm.DefaultBfdHoldTime).
//
// PREVENTS: A hold-time of zero, which would arm a timer that fires
// immediately, and a value the uint16 field would silently truncate.
func TestBFDSettingsHoldTimeBounds(t *testing.T) {
	for _, value := range []string{"1", "65535"} {
		peers, err := PeersFromTree(bfdStrictTree(map[string]any{"strict": "true", "hold-time": value}))
		require.NoError(t, err, "hold-time %s is inside the range", value)
		require.NotZero(t, peers[0].BFD.HoldTime)
	}

	for _, value := range []string{"0", "65536"} {
		_, err := PeersFromTree(bfdStrictTree(map[string]any{"strict": "true", "hold-time": value}))
		require.Error(t, err, "hold-time %s is outside the range", value)
		require.Contains(t, err.Error(), "hold-time")
	}

	peers, err := PeersFromTree(bfdStrictTree(map[string]any{"strict": "true"}))
	require.NoError(t, err)
	require.Equal(t, uint16(0), peers[0].BFD.HoldTime,
		"an absent leaf leaves the timer to its own draft default")
}

// TestBFDSettingsHoldDownParse is the config end of the BFD hold-down interval
// of draft-ietf-idr-bgp-bfd-strict-mode Section 10.
//
// VALIDATES: `hold-down` reaches BFDSettings.HoldDown in milliseconds, and its
// absence leaves the field zero.
//
// PREVENTS: A unit slip. The leaf beside it, `hold-time`, is in SECONDS, so a
// parser that read both the same way would make a 300 ms hold-down into a 300
// second one and hold every strict peer down for five minutes.
func TestBFDSettingsHoldDownParse(t *testing.T) {
	peers, err := PeersFromTree(bfdStrictTree(map[string]any{
		"strict":    "true",
		"hold-down": "300",
	}))
	require.NoError(t, err)
	require.Equal(t, uint32(300), peers[0].BFD.HoldDown)
	require.Equal(t, uint16(0), peers[0].BFD.HoldTime, "the two leaves are independent")

	absent, err := PeersFromTree(bfdStrictTree(map[string]any{"strict": "true"}))
	require.NoError(t, err)
	require.Equal(t, uint32(0), absent[0].BFD.HoldDown,
		"an absent leaf establishes on the first BFD Up, which is what a peer did before the interval existed")
}

// TestBFDStrictConfigChangedCountsHoldDown covers the reload predicate over the
// leaf added last.
//
// VALIDATES: A change to hold-down alone counts as a strict-mode configuration
// change, so the reload path raises draft Event 35.
//
// PREVENTS: A reload that changes only the damping interval restarting the peer
// with no NOTIFICATION saying why, which is what the predicate exists to stop
// for every other strict-mode leaf.
func TestBFDStrictConfigChangedCountsHoldDown(t *testing.T) {
	base := func() *PeerSettings {
		s := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 1)
		s.BFD = &BFDSettings{Enabled: true, Strict: true, HoldTime: 30, HoldDown: 300}
		return s
	}
	changed := base()
	changed.BFD.HoldDown = 500
	require.True(t, bfdStrictConfigChanged(base(), changed))
	require.False(t, bfdStrictConfigChanged(base(), base()))
}

// TestStrictPeerRequestReachesTheSharedKey is the BGP half of RFC 5882
// Section 4.4. The other two halves drive their own real builders against the
// same key: TestOSPFNeighborRequestReachesTheSharedKey
// (internal/plugins/ospf) drives bfdRequestForNeighbor, and
// TestPinnedSessionReachesTheSharedKey (internal/component/bfd) drives the
// pinned-session parser. No single test can drive all three, because each
// builder is unexported in its own package.
//
// RFC requirement: RFC5882-4.4-1 positive -- "If multiple control protocols
// wish to establish BFD sessions with the same remote system for the same data
// protocol, all MUST share a single BFD session" (RFC 5882 sec 4.4). A strict
// peer whose operator wrote neither the optional `bfd interface` leaf nor a
// local address still reaches the key OSPF builds for the same neighbor,
// because api.SessionRequest.Canonical derives both from the link the peer is
// on. Before that derivation existed, bfdRequestFor produced Interface "" and
// VRF "" and the two protocols opened two sessions on one link.
func TestStrictPeerRequestReachesTheSharedKey(t *testing.T) {
	links := []api.Link{{
		Name: "eth0",
		VRF:  api.DefaultVRF,
		Addrs: []api.LinkAddress{{
			Addr:   netip.MustParseAddr("172.30.0.2"),
			Prefix: netip.MustParsePrefix("172.30.0.0/24"),
		}},
	}}
	want := api.Key{
		Peer:      netip.MustParseAddr("172.30.0.10"),
		Local:     netip.MustParseAddr("172.30.0.2"),
		Interface: "eth0",
		VRF:       api.DefaultVRF,
		Mode:      api.SingleHop,
	}

	bare := NewPeerSettings(netip.MustParseAddr("172.30.0.10"), 65001, 65003, 1)
	bare.BFD = &BFDSettings{Enabled: true, Strict: true}
	require.Equal(t, want, bfdRequestFor(bare).Canonical(api.Topology{Links: links}).Key(),
		"a strict peer naming no interface and no local address must land on the shared session")

	// The same peer with the local address configured: the derivation has
	// less to do and MUST reach the same key, or an operator who writes the
	// optional leaf gets a second session for writing it.
	named := NewPeerSettings(netip.MustParseAddr("172.30.0.10"), 65001, 65003, 1)
	named.LocalAddress = netip.MustParseAddr("172.30.0.2")
	named.BFD = &BFDSettings{Enabled: true, Strict: true, Interface: "eth0"}
	require.Equal(t, want, bfdRequestFor(named).Canonical(api.Topology{Links: links}).Key(),
		"naming the interface and the local address must not build a second key")
}
