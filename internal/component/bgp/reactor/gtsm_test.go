// Design: rfc/short/rfc5082.md -- GTSM, the TTL 255 rule for related ICMP messages
// Overview: gtsm.go -- gtsmPeers and publishGTSMKernelState, the code under test

package reactor

import (
	"net/netip"
	"reflect"
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

	var published []gtsm.Peer
	previous := setGTSMPeers
	setGTSMPeers = func(peers []gtsm.Peer) error {
		published = peers
		return nil
	}
	t.Cleanup(func() { setGTSMPeers = previous })
	return &published
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

	tables := gtsm.FilterTables(*published)
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
	host := netip.PrefixFrom(peer.Addr, peer.Addr.BitLen())
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
// family at the peer's floor: firewall.MatchIPv4TTLBelow for IPv4, and the
// IPv6 hop-limit match (spec-bcp194-3-session D-3, MatchIPv6HopLimitBelow) for
// IPv6. The IPv6 type does not exist until Phase 2 adds it, so it is read by
// name and its Floor field here; Phase 2 replaces this with a type switch.
func floorMatchFor(match firewall.Match, peer gtsm.Peer) bool {
	if peer.Addr.Is4() {
		m, ok := match.(firewall.MatchIPv4TTLBelow)
		return ok && m.Floor == peer.Floor
	}
	value := reflect.ValueOf(match)
	if value.Kind() != reflect.Struct || value.Type().Name() != "MatchIPv6HopLimitBelow" {
		return false
	}
	floor := value.FieldByName("Floor")
	return floor.IsValid() && floor.Kind() == reflect.Uint8 && floor.Uint() == uint64(peer.Floor)
}
