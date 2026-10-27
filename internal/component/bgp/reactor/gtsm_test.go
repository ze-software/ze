// Design: rfc/short/rfc5082.md -- GTSM, the TTL 255 rule for related ICMP messages
// Overview: gtsm.go -- gtsmPeers and publishGTSMKernelState, the code under test

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/gtsm"
)

// captureGTSMPeers redirects the gtsm component's reconcile entry point into
// the returned slice pointer, so a test reads the peer set this reactor
// derives without a kernel to publish it to.
func captureGTSMPeers(t *testing.T) *[]gtsm.Peer {
	t.Helper()
	return &captureGTSMSet(t).peers
}

// gtsmSet is one publication the reactor handed the gtsm component.
type gtsmSet struct {
	peers  []gtsm.Peer
	ranges []gtsm.Range
}

// captureGTSMSet redirects the reconcile entry point and keeps both halves of
// the last publication.
func captureGTSMSet(t *testing.T) *gtsmSet {
	t.Helper()

	published := &gtsmSet{}
	previous := setGTSMPeers
	setGTSMPeers = func(peers []gtsm.Peer, ranges []gtsm.Range) error {
		published.peers = peers
		published.ranges = ranges
		return nil
	}
	t.Cleanup(func() { setGTSMPeers = previous })
	return published
}

// TestListenTermsDynamicRange is AC-16 at the reactor: a dynamic group whose
// template carries `ttl max 1` publishes its range with the template's floor
// and listen port, the gtsm tables then drop a below-floor SYN from inside
// the range, and a group without GTSM is published with no floor and owes no
// term while no GTSM range contains it.
func TestListenTermsDynamicRange(t *testing.T) {
	published := captureGTSMSet(t)
	r := New(&Config{Port: 179})

	template := NewPeerSettings(netip.Addr{}, 65000, 0, 0x01010101)
	template.OutTTL = 255
	template.MinTTL = 255
	plain := NewPeerSettings(netip.Addr{}, 65000, 0, 0x01010101)
	gtsmRange := netip.MustParsePrefix("10.1.0.0/16")
	r.SetDynamicGroups([]*DynamicGroupConfig{
		{GroupName: "ixp", Ranges: []netip.Prefix{gtsmRange}, Settings: template},
		{GroupName: "plain", Ranges: []netip.Prefix{netip.MustParsePrefix("10.2.0.0/16")}, Settings: plain},
	})

	r.publishGTSMKernelState()

	require.Equal(t, []gtsm.Range{
		{Prefix: gtsmRange, Port: 179, Floor: 255},
		{Prefix: netip.MustParsePrefix("10.2.0.0/16"), Port: 179},
	}, published.ranges, "the GTSM range with its floor, the plain one with none")
	tables := gtsm.FilterTables(published.peers, published.ranges)
	require.Len(t, tables, 1)
	terms := tables[0].Chains[0].Terms
	require.Len(t, terms, 1, "the plain range lies in no GTSM range, so it owes no term")
	member := gtsm.Peer{Addr: netip.MustParseAddr("10.1.2.3"), Port: 179, Floor: 255}
	assert.True(t, hasListenPortDropFrom(terms, gtsmRange, member), "no term drops a below-floor SYN from the range")
}

// TestGTSMRangeSparesAPlainStaticPeerInside proves the reactor publishes a
// static peer without GTSM, and that the gtsm component then claims its
// address ahead of the GTSM range that contains it. The listener gives that
// peer's SYN to the peer's own session, which is not GTSM-enabled, so RFC 5082
// Section 3 forbids the range's rule from dropping it.
func TestGTSMRangeSparesAPlainStaticPeerInside(t *testing.T) {
	published := captureGTSMSet(t)
	r := New(&Config{Port: 179})

	template := NewPeerSettings(netip.Addr{}, 65000, 0, 0x01010101)
	template.OutTTL = 255
	template.MinTTL = 255
	gtsmRange := netip.MustParsePrefix("10.1.0.0/16")
	r.SetDynamicGroups([]*DynamicGroupConfig{{GroupName: "ixp", Ranges: []netip.Prefix{gtsmRange}, Settings: template}})
	require.NoError(t, r.AddPeer(NewPeerSettings(mustParseAddr("10.1.9.9"), 65000, 65009, 0x01010101)))

	r.publishGTSMKernelState()

	host := netip.MustParsePrefix("10.1.9.9/32")
	require.Contains(t, published.ranges, gtsm.Range{Prefix: host, Port: 179}, "the plain static peer is published with no floor")
	tables := gtsm.FilterTables(published.peers, published.ranges)
	require.Len(t, tables, 1)
	terms := tables[0].Chains[0].Terms
	require.Len(t, terms, 2)
	assert.Equal(t, []firewall.Match{
		firewall.MatchSourceAddress{Prefix: host},
		firewall.MatchProtocol{Protocol: "tcp"},
		firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 179, Hi: 179}}},
	}, terms[0].Matches, "the claim names the peer and its listen port")
	assert.Equal(t, []firewall.Action{firewall.Accept{}}, terms[0].Actions, "the claim ends this chain for the peer's SYN")
	member := gtsm.Peer{Addr: netip.MustParseAddr("10.1.2.3"), Port: 179, Floor: 255}
	assert.True(t, hasListenPortDropFrom(terms[1:], gtsmRange, member), "the range's drop comes after the claim")
}

// TestGTSMRangeEqualLengthFollowsTheListener proves that a prefix held by two
// sources is published only for the one the listener gives its SYNs to: a
// static peer before a group's host prefix at its address, and the first of
// two groups whose ranges are the same prefix (findDynamicGroup). The source
// the listener never picks would otherwise drop, by its own floor, the SYNs of
// a session with no GTSM, which RFC 5082 Section 3 forbids.
func TestGTSMRangeEqualLengthFollowsTheListener(t *testing.T) {
	published := captureGTSMSet(t)
	r := New(&Config{Port: 179})

	gtsmTemplate := NewPeerSettings(netip.Addr{}, 65000, 0, 0x01010101)
	gtsmTemplate.OutTTL = 255
	gtsmTemplate.MinTTL = 255
	plain := NewPeerSettings(netip.Addr{}, 65000, 0, 0x01010101)
	shared := netip.MustParsePrefix("10.3.0.0/24")
	host := netip.MustParsePrefix("10.4.0.9/32")
	r.SetDynamicGroups([]*DynamicGroupConfig{
		{GroupName: "first", Ranges: []netip.Prefix{shared}, Settings: plain},
		{GroupName: "second", Ranges: []netip.Prefix{shared, host}, Settings: gtsmTemplate},
	})
	require.NoError(t, r.AddPeer(NewPeerSettings(host.Addr(), 65000, 65009, 0x01010101)))

	r.publishGTSMKernelState()

	assert.ElementsMatch(t, []gtsm.Range{
		{Prefix: host, Port: 179},
		{Prefix: shared, Port: 179},
	}, published.ranges, "each prefix is published once, for the session the listener picks")
	assert.Nil(t, gtsm.FilterTables(published.peers, published.ranges), "no source the listener picks asked for GTSM, so no term drops")
}

// noopJournal is the rollback journal the config apply hands the reconcile.
// This test needs the reconcile to run, not to roll back, so every recorded
// step is applied and nothing is kept.
type noopJournal struct{}

func (noopJournal) Record(apply, _ func() error) error { return apply() }
func (noopJournal) Rollback() []error                  { return nil }
func (noopJournal) Discard()                           {}

// TestReactorPublishesGTSMPeersFromConfig is the wiring proof: a peer whose
// configuration derived the GTSM TTL values reaches the gtsm component through
// the same peer reconcile a config apply runs, carrying the address, the BGP
// port and both TTL values.
//
// VALIDATES: reconcilePeersJournaled publishes the GTSM peer set it produced.
// PREVENTS: a GTSM peer whose kernel state is installed by nothing, because
// the only publisher ran at start and never again.
func TestReactorPublishesGTSMPeersFromConfig(t *testing.T) {
	published := captureGTSMPeers(t)
	r := New(&Config{Port: 179})

	gtsmPeer := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0x01010101)
	// The two values parseTTLSettings derives from `connection ttl max 1`.
	gtsmPeer.OutTTL = 255
	gtsmPeer.MinTTL = 255
	plain := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65002, 0x01010101)

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{gtsmPeer, plain}, Globals{}, dynamicGroupsNotRead, "test", noopJournal{}))

	require.Len(t, *published, 1, "only the peer that enabled GTSM owes kernel state")
	got := (*published)[0]
	assert.Equal(t, gtsmPeer.Address, got.Addr)
	assert.Equal(t, uint16(179), got.Port, "the port that associates a quoted TCP header with this session")
	assert.Equal(t, uint8(255), got.HopLimit)
	assert.Equal(t, uint8(255), got.Floor)
}

// TestReactorPublishesAPeersOwnListenPort proves the port handed to the filter
// is the peer's own, not the default: a term built with 179 would claim no
// session of a peer that runs BGP somewhere else, and the error it was meant
// to refuse would be delivered.
func TestReactorPublishesAPeersOwnListenPort(t *testing.T) {
	published := captureGTSMPeers(t)
	r := New(&Config{Port: 179})

	settings := NewPeerSettings(mustParseAddr("10.0.0.4"), 65000, 65004, 0x01010101)
	settings.LocalPort = 1179
	settings.OutTTL = 255
	settings.MinTTL = 254
	require.NoError(t, r.AddPeer(settings))

	r.publishGTSMKernelState()

	require.Len(t, *published, 1)
	assert.Equal(t, uint16(1179), (*published)[0].Port)
	assert.Equal(t, uint8(254), (*published)[0].Floor, "the floor is the peer's own, so a multi-hop session keeps working")
}

// TestReactorPublishesNoPeerWithoutGTSM keeps the feature off a deployment
// that did not ask for it: no peer in the set means the gtsm component
// installs no route and never loads the firewall backend.
func TestReactorPublishesNoPeerWithoutGTSM(t *testing.T) {
	published := captureGTSMPeers(t)
	r := New(&Config{Port: 179})

	require.NoError(t, r.AddPeer(NewPeerSettings(mustParseAddr("10.0.0.3"), 65000, 65003, 0x01010101)))

	r.publishGTSMKernelState()

	assert.Empty(t, *published)
}

// TestGTSMPeersFromResolvedTreeReadsTheConfigAlone is the `ze doctor` half of
// the wiring: with no reactor running, the same derivation reads the resolved
// bgp{} tree and answers the set a config apply would publish.
//
// VALIDATES: a peer with `connection ttl max 1` is in the set with hop limit
// 255, floor 255 and the default BGP port; a peer without a ttl block is not;
// and the seam the gtsm doctor check reads is this function.
// PREVENTS: a doctor check that reads the configuration through a second,
// drifting parse of the ttl block.
func TestGTSMPeersFromResolvedTreeReadsTheConfigAlone(t *testing.T) {
	tree := map[string]any{
		"router-id": "10.0.0.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"peer": map[string]any{
			"gtsm": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{"ip": "192.0.2.1"},
					"local":  map[string]any{"ip": "auto"},
					"ttl":    map[string]any{"max": "1"},
				},
				"session": map[string]any{"asn": map[string]any{"remote": "65001"}},
			},
			"plain": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{"ip": "192.0.2.2"},
					"local":  map[string]any{"ip": "auto"},
				},
				"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
			},
		},
	}

	peers, err := gtsmPeersFromResolvedTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1, "only the peer with a ttl block owes kernel state")
	assert.Equal(t, mustParseAddr("192.0.2.1"), peers[0].Addr)
	assert.Equal(t, uint16(DefaultBGPPort), peers[0].Port)
	assert.Equal(t, uint8(255), peers[0].HopLimit)
	assert.Equal(t, uint8(255), peers[0].Floor)

	// A peer section the parse cannot read answers the refusal, never an empty
	// set: an empty set reads as "no GTSM peer" and would keep the doctor check
	// silent about a configuration nothing could derive.
	_, err = gtsmPeersFromResolvedTree(map[string]any{"peer": "not a map"})
	assert.Error(t, err, "a tree the peer parse refuses answers the refusal, never an empty set")
}

// TestListenerMinTTLWiring is the wiring proof for the GTSM listen-port
// floor: a configuration with one IPv4 and one IPv6 GTSM peer, parsed and
// reconciled the way a config apply does it, publishes through setGTSMPeers
// a peer set whose gtsm tables drop each peer's TCP to its Ze listen port
// below the peer's floor, before the TCP stack sees the SYN.
//
// VALIDATES: PeersFromTree -> reconcilePeersJournaled -> setGTSMPeers ->
// gtsm.FilterTables carries a listen-port term per GTSM peer and family.
// PREVENTS: a below-floor SYN from a GTSM peer's address costing a request
// socket and a SYN-ACK because only the accepted connection carries a floor
// (RFC 5082 Section 2.2: "For maximal protection, ingress filtering should be
// applied before the packet goes through the scarce resource.").
func TestListenerMinTTLWiring(t *testing.T) {
	published := captureGTSMPeers(t)
	r := New(&Config{Port: 179})

	gtsmPeerTree := func(ip, asn string) map[string]any {
		return map[string]any{
			"connection": map[string]any{
				"remote": map[string]any{"ip": ip},
				"local":  map[string]any{"ip": "auto"},
				"ttl":    map[string]any{"max": "1"},
			},
			"session": map[string]any{"asn": map[string]any{"remote": asn}},
		}
	}
	settings, err := PeersFromTree(map[string]any{
		"router-id": "10.0.0.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"peer": map[string]any{
			"v4": gtsmPeerTree("192.0.2.1", "65001"),
			"v6": gtsmPeerTree("2001:db8::1", "65002"),
		},
	})
	require.NoError(t, err)

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.reconcilePeersJournaled(settings, Globals{}, dynamicGroupsNotRead, "test", noopJournal{}))
	require.Len(t, *published, 2, "both GTSM peers reach the gtsm component")

	tables := gtsm.FilterTables(*published, nil)
	require.Len(t, tables, 1, "one ze_gtsm table carries every peer's terms")
	require.Len(t, tables[0].Chains, 1)
	terms := tables[0].Chains[0].Terms

	for _, peer := range *published {
		assert.Truef(t, hasListenPortDrop(terms, peer), "no term drops TCP from %s to listen port %d below floor %d", peer.Addr, peer.Port, peer.Floor)
	}
}

// hasListenPortDrop reports whether one term drops TCP whose source is the
// peer's address, whose destination port is the peer's Ze listen port, and
// whose TTL (IPv4) or hop limit (IPv6) is below the peer's floor.
func hasListenPortDrop(terms []firewall.Term, peer gtsm.Peer) bool {
	return hasListenPortDropFrom(terms, netip.PrefixFrom(peer.Addr, peer.Addr.BitLen()), peer)
}

// hasListenPortDropFrom is hasListenPortDrop with the term's source named:
// the peer's host prefix, or the dynamic group range the peer falls in.
func hasListenPortDropFrom(terms []firewall.Term, host netip.Prefix, peer gtsm.Peer) bool {
	for _, term := range terms {
		source, tcp, toPort, belowFloor := false, false, false, false
		for _, match := range term.Matches {
			switch m := match.(type) {
			case firewall.MatchSourceAddress:
				source = m.Prefix == host
			case firewall.MatchProtocol:
				tcp = m.Protocol == "tcp"
			case firewall.MatchDestinationPort:
				for _, r := range m.Ranges {
					if r.Lo <= peer.Port && peer.Port <= r.Hi {
						toPort = true
					}
				}
			default:
				belowFloor = belowFloor || floorMatchFor(match, peer)
			}
		}
		if !source || !tcp || !toPort || !belowFloor {
			continue
		}
		if len(term.Actions) == 1 {
			if _, drop := term.Actions[0].(firewall.Drop); drop {
				return true
			}
		}
	}
	return false
}

// floorMatchFor reports whether match is the below-floor match for the peer's
// family at the peer's floor: firewall.MatchIPv4TTLBelow for IPv4, and
// firewall.MatchIPv6HopLimitBelow for IPv6 (spec-bcp194-3-session D-3). A
// match of the other family's type answers false, so an IPv6 peer guarded by
// an IPv4 TTL read is a red, not a pass.
func floorMatchFor(match firewall.Match, peer gtsm.Peer) bool {
	switch m := match.(type) {
	case firewall.MatchIPv4TTLBelow:
		return peer.Addr.Is4() && m.Floor == peer.Floor
	case firewall.MatchIPv6HopLimitBelow:
		return peer.Addr.Is6() && m.Floor == peer.Floor
	}
	return false
}
