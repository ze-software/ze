// Design: docs/architecture/bgp/interface-event-reactions.md -- scoped session fixtures for subnet refresh.
// Related: link_scope_scoped_test.go -- scoped membership and refresh assertions.
// Related: next_hop_auto_relay_test.go -- shared recording session fixtures.
package reactor

import (
	"bufio"
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/network"
)

type ls71PairMode struct {
	name                           string
	mode                           uint8
	internal, rsClient, preserving bool
}

func ls71PairModes() []ls71PairMode {
	return []ls71PairMode{
		{name: "auto", mode: NextHopAuto},
		{name: "self", mode: NextHopSelf},
		{name: "explicit", mode: NextHopExplicit},
		{name: "unchanged", mode: NextHopUnchanged, preserving: true},
		{name: "ibgp-auto", mode: NextHopAuto, internal: true, preserving: true},
		{name: "rs-auto", mode: NextHopAuto, rsClient: true, preserving: true},
	}
}

type ls71PairTopology struct {
	name                                string
	self, explicit, received, recipient netip.Addr
	connected                           []netip.Prefix
	joint, ownLL, neighborLL            bool
}

func ls71PairTopologies() []ls71PairTopology {
	base := ls71PairTopology{
		name: "joint-subnet", self: netip.MustParseAddr("2001:db8:1::1"),
		explicit: netip.MustParseAddr("2001:db8:1::a"), received: netip.MustParseAddr("2001:db8:1::9"),
		recipient: netip.MustParseAddr("2001:db8:1::2"),
		connected: []netip.Prefix{netip.MustParsePrefix("2001:db8:1::1/64")},
		joint:     true, ownLL: true, neighborLL: true,
	}
	off := base
	off.name, off.recipient, off.joint = "recipient-off-link", netip.MustParseAddr("2001:db8:ffff::2"), false
	split := base
	split.name, split.recipient, split.joint = "speaker-and-peer-share-other-prefix", netip.MustParseAddr("2001:db8:2::2"), false
	split.connected = []netip.Prefix{netip.MustParsePrefix("2001:db8:1::1/64"), netip.MustParsePrefix("2001:db8:2::1/64")}
	moved := split
	moved.name, moved.joint = "effective-entity-moved-onto-shared-prefix", true
	moved.self, moved.explicit, moved.received = netip.MustParseAddr("2001:db8:2::1"), netip.MustParseAddr("2001:db8:2::a"), netip.MustParseAddr("2001:db8:2::9")
	noPrefix := base
	noPrefix.name, noPrefix.joint = "common-prefix-removed", false
	noPrefix.connected = []netip.Prefix{netip.MustParsePrefix("2001:db8:1::1/128")}
	noLL := base
	noLL.name, noLL.ownLL, noLL.neighborLL = "entity-link-local-unknown", false, false
	return []ls71PairTopology{base, off, split, moved, noPrefix, noLL}
}

// The fixture uses the existing ASN4/multiprotocol negotiation and connected
// snapshot seams. Its two local globals are different speaker-owned addresses;
// the third-party neighbor binds only its own global and LL to one MAC/device.
func ls71PairFixture(t *testing.T, mode ls71PairMode, topology ls71PairTopology) *aigpReplayFixture {
	t.Helper()
	f := newAIGPReplayFixture(t, nil)
	rfc2545ForwardScope(f, true, true)
	ls71PairRecipient(f, topology.recipient)
	f.destination.settings.LocalAddress = topology.explicit
	f.destination.settings.LinkLocal = netip.Addr{}
	if topology.ownLL {
		f.destination.settings.LinkLocal = netip.MustParseAddr("fe80::1")
	}
	f.destination.settings.NextHopMode = mode.mode
	f.destination.settings.NextHopAddress = topology.explicit
	f.destination.settings.RSClient = mode.rsClient
	f.destination.settings.PeerAS = 65002
	if mode.internal {
		f.destination.settings.PeerAS = f.destination.settings.LocalAS
	}
	f.destination.session.transport.Store(&sessionTransport{local: topology.self, localString: topology.self.String()})
	if topology.recipient.Zone() != "" {
		// Retain the recording sink while modeling the actual scoped remote
		// endpoint. The zone belongs to transport, never to wire NH bytes.
		conn := &ls71PairScopedConn{
			recordingConn: f.conn,
			local:         &net.TCPAddr{IP: topology.self.AsSlice(), Port: 179},
			remote:        &net.TCPAddr{IP: topology.recipient.AsSlice(), Port: 179, Zone: topology.recipient.Zone()},
		}
		f.destination.session.mu.Lock()
		f.destination.session.conn = conn
		f.destination.session.bufWriter = bufio.NewWriterSize(conn, 4096)
		f.destination.session.mu.Unlock()
	}
	for _, peer := range []*Peer{f.source, f.destination} {
		negotiateForwardTestFamilies(peer, lsFam, family.IPv4Unicast)
	}
	var neighbors []network.Neighbor
	if topology.neighborLL {
		neighbors = []network.Neighbor{
			{Address: topology.received, LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
			{Address: netip.MustParseAddr("fe80::9"), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
		}
	}
	f.destination.refreshLinkScopeWith(topology.connected, neighborView{links: network.NewNeighborLinkLocals(neighbors)})
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	return f
}

func ls71PairRecipient(f *aigpReplayFixture, recipient netip.Addr) {
	delete(f.r.peers, f.destination.Settings().PeerKey())
	f.destination.settings.Address = recipient
	f.r.peers[f.destination.Settings().PeerKey()] = f.destination
	f.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: f.destination.Settings().PeerKey()}, 4096)
}

// ls71PairScopedConn adds endpoint metadata to the existing synchronized sink;
// its inherited Write/written/reset retain the fixture's ownership and fencing.
type ls71PairScopedConn struct {
	*recordingConn
	local, remote *net.TCPAddr
}

func (c *ls71PairScopedConn) LocalAddr() net.Addr  { return c.local }
func (c *ls71PairScopedConn) RemoteAddr() net.Addr { return c.remote }

func ls71PairScopedTopology(peerPrefix bool) ls71PairTopology {
	topology := ls71PairTopologies()[0]
	topology.recipient = netip.MustParseAddr("fe80::2%if0")
	topology.joint = false
	if peerPrefix {
		topology.connected = append(topology.connected, netip.MustParsePrefix("fe80::1/64"))
	}
	return topology
}
