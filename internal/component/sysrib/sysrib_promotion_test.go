// VALIDATES: a promoted equal-cost member is programmed with the forwarding
// path it OWNS -- its device, its share, its MPLS stack and its SRv6 SID --
// because it is the path the packets take once the winner's gateway stops
// resolving. The prefix keeps what the PREFIX owns: the winning protocol, the
// forwarding action and the metric.
// PREVENTS: the MPLS misforward in
// plan/journal/helper-bypassed-by-an-open-coded-copy.md. The entry was built
// from the winner and the promotion then overrode the device and the share
// alone, so a cross-protocol member forwarded out its own interface imposing
// the WINNER's label stack, and encapsulated to the winner's SRv6 SID. A test
// that reads the device alone passes against that defect, because the device
// is one of the two fields the promotion did override.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// producerRoute is one protocol's route for the tests below: where it
// forwards, out of which device, and what it imposes on the traffic it
// carries. Every route here takes the same administrative distance, so the
// routes are equal-cost and the winner is decided by protocol name.
type producerRoute struct {
	protocol string
	nextHop  netip.Addr
	iface    string
	labels   []uint32
	srv6SID  netip.Addr
}

// addProducerRoute hands sysrib one producer's route for a prefix and
// publishes what the arbitration decided, which is the pair of steps every
// live path takes.
func addProducerRoute(s *sysRIB, prefix netip.Prefix, route producerRoute) {
	fam, changes := s.processEvent(makePayload(route.protocol, family.IPv4Unicast, []incomingChange{{
		Action:    routeaction.Add,
		Prefix:    prefix,
		NextHop:   route.nextHop,
		Interface: route.iface,
		Priority:  20,
		Labels:    route.labels,
		SRv6SID:   route.srv6SID,
	}}))
	if len(changes) > 0 {
		publishChanges(changes, fam)
	}
}

// promotionState is the state every test below drives: a member whose gateway
// the Loc-RIB covers, and a winner whose gateway it does not, so the resolver
// proves a path for the member alone and the member takes the prefix.
//
// The member is inserted FIRST and the winner second, because the winner is
// the protocol whose name sorts first and a prefix that changes hands is what
// the live path publishes.
type promotionState struct {
	sysrib *sysRIB
	bus    *testEventBus
	prefix netip.Prefix
	member producerRoute
	winner producerRoute
}

// TestPromotedMemberIsProgrammedWithItsOwnLabels is the MPLS misforward. The
// promoted member is another protocol's path, and its label stack is its own:
// imposing the winner's stack on it sends the packet into a label-switched
// path the next hop did not advertise.
func TestPromotedMemberIsProgrammedWithItsOwnLabels(t *testing.T) {
	state := newPromotionState(t, netip.MustParsePrefix("10.60.0.0/24"))
	state.member.labels = []uint32{200}
	state.winner.labels = []uint32{100}

	promoted := state.promote(t)

	if promoted.Interface != state.member.iface {
		t.Errorf("the entry forwards out %q, want the promoted member's %q",
			promoted.Interface, state.member.iface)
	}
	if !labelsEqual(promoted.Labels, state.member.labels) {
		t.Errorf("the entry forwards out %q imposing %v, want the promoted member's own stack %v: "+
			"the winner's stack over another protocol's path is an MPLS misforward",
			promoted.Interface, promoted.Labels, state.member.labels)
	}
}

// TestPromotedMemberIsProgrammedWithItsOwnSRv6SID is the same defect one field
// along. The winner's Service SID resolves, so nothing refuses the write, and
// the member carries no SID at all: encapsulating its path to the winner's SID
// sends the packet to a remote endpoint that answers for a service the member
// never advertised.
func TestPromotedMemberIsProgrammedWithItsOwnSRv6SID(t *testing.T) {
	state := newPromotionState(t, netip.MustParsePrefix("10.61.0.0/24"))
	connectedID := redistevents.RegisterProtocol("connected")
	getLocRIB().Insert(family.IPv6Unicast, srv6Locator, locrib.Path{Source: connectedID})
	state.winner.srv6SID = netip.MustParseAddr("2001:db8:cafe::7")

	promoted := state.promote(t)

	if promoted.SRv6SID.IsValid() {
		t.Errorf("the entry encapsulates to %s over the promoted member's path out %q, want no SID: "+
			"the member advertised none", promoted.SRv6SID, promoted.Interface)
	}
}

// TestPromotedMemberKeepsWhatThePrefixOwns is the other half of the split. The
// promotion moves the forwarding PATH onto the member and nothing else: the
// prefix still belongs to the protocol that won it, which is what a FIB writer
// attributes the entry to and what `show rib` reports.
func TestPromotedMemberKeepsWhatThePrefixOwns(t *testing.T) {
	state := newPromotionState(t, netip.MustParsePrefix("10.62.0.0/24"))

	promoted := state.promote(t)

	if promoted.Protocol != state.winner.protocol {
		t.Errorf("the entry is credited to %q, want the winning protocol %q: the promotion moves "+
			"the path, not the prefix", promoted.Protocol, state.winner.protocol)
	}
}

// newPromotionState wires a Loc-RIB that covers the member's gateway and not
// the winner's, and returns the two routes the test completes and drives.
func newPromotionState(t *testing.T, prefix netip.Prefix) *promotionState {
	t.Helper()
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("isis")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"),
		locrib.Path{Source: connectedID})

	bus := newTestEventBus()
	return &promotionState{
		sysrib: configuredSysRIB(t, bus),
		bus:    bus,
		prefix: prefix,
		member: producerRoute{
			protocol: "isis",
			nextHop:  netip.MustParseAddr("198.51.100.5"),
			iface:    "eth1",
		},
		winner: producerRoute{
			protocol: "bgp",
			nextHop:  netip.MustParseAddr("192.0.2.1"),
			iface:    "eth0",
		},
	}
}

// promote drives the state and answers with the entry sysrib published for the
// winner: the member is programmed first, then the winner takes the prefix and
// the resolver proves no path for its gateway.
func (p *promotionState) promote(t *testing.T) outgoingChange {
	t.Helper()
	addProducerRoute(p.sysrib, p.prefix, p.member)

	installed := publishedChanges(t, p.bus)
	if len(installed) != 1 || installed[0].Protocol != p.member.protocol {
		t.Fatalf("setup: published %+v, want one add for %s from %q",
			installed, p.prefix, p.member.protocol)
	}

	addProducerRoute(p.sysrib, p.prefix, p.winner)

	promoted := publishedChanges(t, p.bus)[1:]
	if len(promoted) != 1 {
		t.Fatalf("published %+v, want one change for %s: %q took the prefix and its gateway "+
			"resolves to nothing, so the equal-cost member carries it",
			promoted, p.prefix, p.winner.protocol)
	}
	if promoted[0].NextHop != installed[0].NextHop {
		t.Fatalf("the entry forwards to %s, want the promoted member's %s",
			promoted[0].NextHop, installed[0].NextHop)
	}
	return promoted[0]
}
