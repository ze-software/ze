package scenario

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The hand-written network every literal assertion below is checked against.
// It is typed here, never generated, so it cannot share a mistake with
// GenerateTopology.
//
//	transitA (AS 65101, 192.0.2.1) --- border1 (10.0.0.1) ---+
//	                                                         +--- reflector (10.0.0.3) --- client (10.0.0.4)
//	transitB (AS 65102, 192.0.2.2) --- border2 (10.0.0.2) ---+
//
// border1, border2 and reflector are fully meshed. client holds one iBGP
// session, with reflector. Both transits announce 198.51.100.0/24 from origin
// AS 65201 with paths of equal length, and transitB alone originates
// 203.0.113.0/24.
var (
	handBorder1   = netip.MustParseAddr("10.0.0.1")
	handBorder2   = netip.MustParseAddr("10.0.0.2")
	handReflector = netip.MustParseAddr("10.0.0.3")
	handClient    = netip.MustParseAddr("10.0.0.4")
	handTransitA  = netip.MustParseAddr("192.0.2.1")
	handTransitB  = netip.MustParseAddr("192.0.2.2")

	handTwoExits = netip.MustParsePrefix("198.51.100.0/24")
	handOneExit  = netip.MustParsePrefix("203.0.113.0/24")
	handInside   = netip.MustParsePrefix("10.20.0.0/24")
)

const handLocalAS = 65000

// handTopologySpec returns a fresh copy of the hand-written network, so a test
// may change it.
func handTopologySpec() TopologySpec {
	return TopologySpec{
		LocalAS: handLocalAS,
		Routers: []TopologyRouter{
			{Loopback: handBorder1},
			{Loopback: handBorder2},
			{Loopback: handReflector, Clients: []netip.Addr{handClient}},
			{Loopback: handClient},
		},
		Neighbors: []TopologyNeighbor{
			{
				Address: handTransitA, AS: 65101, Border: handBorder1, BorderNextHop: NextHopPolicySelf,
				Announces: []NeighborRoute{{Prefix: handTwoExits, ASPathBeyond: []uint32{65201}}},
			},
			{
				Address: handTransitB, AS: 65102, Border: handBorder2, BorderNextHop: NextHopPolicySelf,
				Announces: []NeighborRoute{
					{Prefix: handOneExit},
					{Prefix: handTwoExits, ASPathBeyond: []uint32{65201}},
				},
			},
		},
	}
}

// generatedTopologyParams returns the network of AC-16: five routers of one AS
// and six external neighbors.
func generatedTopologyParams(seed uint64) TopologyParams {
	return TopologyParams{
		Seed:             seed,
		LocalAS:          handLocalAS,
		Routers:          5,
		Borders:          2,
		ReflectorClients: 2,
		Neighbors:        6,
		Prefixes:         8,
		RouterFirst:      netip.MustParseAddr("10.0.0.1"),
		NeighborFirst:    netip.MustParseAddr("192.0.2.1"),
	}
}

// speakerAt returns the speaker of topology whose session address is address.
func speakerAt(t *testing.T, topology *Topology, address netip.Addr) Speaker {
	t.Helper()
	for _, speaker := range topology.Speakers() {
		if speaker.Address == address {
			return speaker
		}
	}
	require.FailNowf(t, "no speaker", "the topology holds no speaker at %s", address)
	return Speaker{}
}

// announcementOf returns what the speaker at address sends the collector for prefix.
func announcementOf(t *testing.T, topology *Topology, address netip.Addr, prefix netip.Prefix) Announcement {
	t.Helper()
	for _, announcement := range speakerAt(t, topology, address).Announcements {
		if announcement.Prefix == prefix {
			return announcement
		}
	}
	require.FailNowf(t, "no announcement", "speaker %s announces no route for %s", address, prefix)
	return Announcement{}
}

// TestTopologySameSeedSameNetwork verifies that the generator is deterministic.
//
// VALIDATES: AC-16 — one seed gives the same routers, neighbors, prefixes,
// per-speaker announcements and traces, and another seed gives another network.
// PREVENTS: a map order or an unseeded choice reaching the generated network.
func TestTopologySameSeedSameNetwork(t *testing.T) {
	first, err := GenerateTopology(generatedTopologyParams(7))
	require.NoError(t, err)
	second, err := GenerateTopology(generatedTopologyParams(7))
	require.NoError(t, err)

	assert.Equal(t, first.Speakers(), second.Speakers())
	assert.Equal(t, first.Prefixes(), second.Prefixes())

	var routers, neighbors int
	for _, speaker := range first.Speakers() {
		switch speaker.Session {
		case SessionKindInternal:
			routers++
			for _, prefix := range first.Prefixes() {
				firstTrace, err := first.Trace(speaker.Address, prefix)
				require.NoError(t, err)
				secondTrace, err := second.Trace(speaker.Address, prefix)
				require.NoError(t, err)
				assert.Equal(t, firstTrace, secondTrace, "trace of %s from %s", prefix, speaker.Address)
			}
		case SessionKindExternal:
			neighbors++
		case SessionKindUnspecified:
			assert.Failf(t, "unspecified session", "speaker %s has no session kind", speaker.Address)
		}
	}
	assert.Equal(t, 5, routers, "routers of the AS")
	assert.Equal(t, 6, neighbors, "external neighbors")
	// Eight prefixes from outside and the one that originates inside.
	assert.Len(t, first.Prefixes(), 9)

	other, err := GenerateTopology(generatedTopologyParams(8))
	require.NoError(t, err)
	assert.NotEqual(t, first.Speakers(), other.Speakers(), "another seed gives another network")
}

// TestBorderRouterRewritesNextHop verifies next-hop-self on a border router.
//
// VALIDATES: AC-17 — the route a border router learned outside carries its
// own loopback toward the collector and toward the other routers, and the
// neighbor's AS path unchanged.
// PREVENTS: the neighbor's address, or a path with an AS added, leaving the
// border router.
func TestBorderRouterRewritesNextHop(t *testing.T) {
	topology, err := NewTopology(handTopologySpec())
	require.NoError(t, err)

	// Toward the collector.
	assert.Equal(t,
		Announcement{Prefix: handTwoExits, NextHop: handBorder1, ASPath: []uint32{65101, 65201}},
		announcementOf(t, topology, handBorder1, handTwoExits))

	// Toward the other routers: border2 holds 203.0.113.0/24 alone, so what
	// border1 passes the collector unchanged is what border2 sent it inside.
	assert.Equal(t,
		Announcement{Prefix: handOneExit, NextHop: handBorder2, ASPath: []uint32{65102}},
		announcementOf(t, topology, handBorder1, handOneExit))
}

// TestReflectorRewritesReflectedNextHop verifies the reflector's rewrite.
//
// VALIDATES: AC-24 — the client receives the reflector as next hop and passes
// it to the collector unchanged, while the reflector passes the collector the
// border router's next hop.
// PREVENTS: a client whose next hop names the border router directly, which
// would leave no trace across the reflector.
func TestReflectorRewritesReflectedNextHop(t *testing.T) {
	topology, err := NewTopology(handTopologySpec())
	require.NoError(t, err)

	assert.Equal(t,
		Announcement{Prefix: handOneExit, NextHop: handReflector, ASPath: []uint32{65102}},
		announcementOf(t, topology, handClient, handOneExit))
	assert.Equal(t,
		Announcement{Prefix: handOneExit, NextHop: handBorder2, ASPath: []uint32{65102}},
		announcementOf(t, topology, handReflector, handOneExit))
}

// TestRouterAnnouncesItsBestExit verifies each router's choice between two exits.
//
// VALIDATES: AC-18 — for a prefix two neighbors announce on two border
// routers, each router announces one best path with the next hop the model
// computed, and the model states why, the tie included.
// PREVENTS: a router announcing both exits, or a tie broken without a reason.
func TestRouterAnnouncesItsBestExit(t *testing.T) {
	type choice struct {
		router  netip.Addr
		nextHop netip.Addr
		asPath  []uint32
		reason  ChoiceReason
	}
	viaA := []uint32{65101, 65201}
	viaB := []uint32{65102, 65201}
	longViaA := []uint32{65101, 65301, 65201}

	tests := []struct {
		name string
		// beyondA is the path transitA announces beyond its own AS.
		beyondA []uint32
		choices []choice
	}{
		{
			name:    "paths of equal length",
			beyondA: []uint32{65201},
			choices: []choice{
				{handBorder1, handBorder1, viaA, ChoiceReasonExternalOverInternal},
				{handBorder2, handBorder2, viaB, ChoiceReasonExternalOverInternal},
				// The two exits rank equal for the reflector: the lower address wins.
				{handReflector, handBorder1, viaA, ChoiceReasonLowerPeerAddress},
				{handClient, handReflector, viaA, ChoiceReasonOnlyRoute},
			},
		},
		{
			name:    "a longer path through transitA",
			beyondA: []uint32{65301, 65201},
			choices: []choice{
				{handBorder1, handBorder2, viaB, ChoiceReasonShorterASPath},
				// border1 prefers border2's route and so announces none inside:
				// border2 and the reflector each hold one route.
				{handBorder2, handBorder2, viaB, ChoiceReasonOnlyRoute},
				{handReflector, handBorder2, viaB, ChoiceReasonOnlyRoute},
				{handClient, handReflector, viaB, ChoiceReasonOnlyRoute},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := handTopologySpec()
			spec.Neighbors[0].Announces[0].ASPathBeyond = test.beyondA
			topology, err := NewTopology(spec)
			require.NoError(t, err)

			for _, want := range test.choices {
				assert.Equal(t,
					Announcement{Prefix: handTwoExits, NextHop: want.nextHop, ASPath: want.asPath},
					announcementOf(t, topology, want.router, handTwoExits), "router %s", want.router)
				trace, err := topology.Trace(want.router, handTwoExits)
				require.NoError(t, err)
				assert.Equal(t, want.reason, trace.Reason, "router %s", want.router)
			}
			// The longer path stays on transitA's own session, whatever the routers chose.
			if len(test.beyondA) == 2 {
				assert.Equal(t, longViaA, announcementOf(t, topology, handTransitA, handTwoExits).ASPath)
			}
		})
	}
}

// TestExternalNeighborKeepsItsNextHop verifies an external neighbor's own session.
//
// VALIDATES: AC-17 — on its eBGP session with the collector a neighbor
// announces its own address as next hop and its own AS first in the path.
// PREVENTS: the border router's rewrite applied to the neighbor's announcement.
func TestExternalNeighborKeepsItsNextHop(t *testing.T) {
	topology, err := NewTopology(handTopologySpec())
	require.NoError(t, err)

	assert.Equal(t, Speaker{
		Address: handTransitA,
		AS:      65101,
		Session: SessionKindExternal,
		Announcements: []Announcement{
			{Prefix: handTwoExits, NextHop: handTransitA, ASPath: []uint32{65101, 65201}},
		},
	}, speakerAt(t, topology, handTransitA))

	assert.Equal(t, Speaker{
		Address: handTransitB,
		AS:      65102,
		Session: SessionKindExternal,
		Announcements: []Announcement{
			{Prefix: handTwoExits, NextHop: handTransitB, ASPath: []uint32{65102, 65201}},
			{Prefix: handOneExit, NextHop: handTransitB, ASPath: []uint32{65102}},
		},
	}, speakerAt(t, topology, handTransitB))
}

// TestTopologyHasMultiHopTrace verifies that a generated network proves a walk
// through the core.
//
// VALIDATES: AC-24 — over several seeds every generated topology holds a
// router whose trace crosses an intermediate router, and the generator refuses
// a network that holds none.
// PREVENTS: a generated network of one-hop traces only passing as proof.
func TestTopologyHasMultiHopTrace(t *testing.T) {
	for seed := uint64(1); seed <= 25; seed++ {
		topology, err := GenerateTopology(generatedTopologyParams(seed))
		require.NoError(t, err, "seed %d", seed)

		routers := map[netip.Addr]bool{}
		for _, speaker := range topology.Speakers() {
			if speaker.Session == SessionKindInternal {
				routers[speaker.Address] = true
			}
		}
		crossing := 0
		for router := range routers {
			for _, prefix := range topology.Prefixes() {
				trace, err := topology.Trace(router, prefix)
				require.NoError(t, err, "seed %d", seed)
				require.NotEmpty(t, trace.Routers, "seed %d", seed)
				assert.Equal(t, router, trace.Routers[0], "seed %d: a trace starts at its router", seed)
				for _, crossed := range trace.Routers {
					assert.True(t, routers[crossed], "seed %d: %s is no router of the AS", seed, crossed)
				}
				if len(trace.Routers) >= 3 {
					crossing++
				}
			}
		}
		assert.Positive(t, crossing, "seed %d: no trace crosses an intermediate router", seed)
	}

	// With no reflector client every router reaches its exit in one hop.
	params := generatedTopologyParams(1)
	params.ReflectorClients = 0
	_, err := GenerateTopology(params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no router's trace crosses an intermediate router")
	assert.Contains(t, err.Error(), "set ReflectorClients to 1 or more")
}

// TestTraceOfHandCheckedPrefix asserts the traces of one prefix literally.
//
// VALIDATES: R-10 — the trace of 198.51.100.0/24 from every router of the
// hand-written network, worked out on paper: the client crosses the reflector
// and leaves at border1, a chain of three routers.
// PREVENTS: the model and the looking glass agreeing on a shared mistake.
func TestTraceOfHandCheckedPrefix(t *testing.T) {
	topology, err := NewTopology(handTopologySpec())
	require.NoError(t, err)

	want := map[netip.Addr]Trace{
		handClient: {
			Routers: []netip.Addr{handClient, handReflector, handBorder1},
			End:     TraceEndExitOwnAddress,
			NextHop: handBorder1,
			ASPath:  []uint32{65101, 65201},
			Reason:  ChoiceReasonOnlyRoute,
		},
		handReflector: {
			Routers: []netip.Addr{handReflector, handBorder1},
			End:     TraceEndExitOwnAddress,
			NextHop: handBorder1,
			ASPath:  []uint32{65101, 65201},
			Reason:  ChoiceReasonLowerPeerAddress,
		},
		handBorder1: {
			Routers: []netip.Addr{handBorder1},
			End:     TraceEndExitOwnAddress,
			NextHop: handBorder1,
			ASPath:  []uint32{65101, 65201},
			Reason:  ChoiceReasonExternalOverInternal,
		},
		handBorder2: {
			Routers: []netip.Addr{handBorder2},
			End:     TraceEndExitOwnAddress,
			NextHop: handBorder2,
			ASPath:  []uint32{65102, 65201},
			Reason:  ChoiceReasonExternalOverInternal,
		},
	}
	for router, trace := range want {
		got, err := topology.Trace(router, handTwoExits)
		require.NoError(t, err)
		assert.Equal(t, trace, got, "trace from %s", router)
	}

	announcers, err := topology.Announcers(handTwoExits)
	require.NoError(t, err)
	assert.Equal(t, []netip.Addr{handTransitA, handTransitB}, announcers)
	assert.Equal(t, []netip.Prefix{handTwoExits, handOneExit}, topology.Prefixes())
}

// TestTraceEndsWhereThePrefixOriginates verifies a prefix originated inside the AS.
//
// VALIDATES: AC-30 — the route has an empty AS path, its trace ends at the
// originating router and says so, and no external neighbor announces it.
// PREVENTS: an origin inside the AS read as an exit toward a neighbor.
func TestTraceEndsWhereThePrefixOriginates(t *testing.T) {
	spec := handTopologySpec()
	spec.Routers[3].Originates = []netip.Prefix{handInside}
	topology, err := NewTopology(spec)
	require.NoError(t, err)

	origin := announcementOf(t, topology, handClient, handInside)
	assert.Equal(t, handClient, origin.NextHop)
	assert.Empty(t, origin.ASPath)

	// The reflector rewrites what it reflects from its client, so border1
	// crosses the reflector to reach the origin.
	trace, err := topology.Trace(handBorder1, handInside)
	require.NoError(t, err)
	assert.Equal(t, []netip.Addr{handBorder1, handReflector, handClient}, trace.Routers)
	assert.Equal(t, TraceEndOriginInside, trace.End)
	assert.Equal(t, handClient, trace.NextHop)
	assert.Empty(t, trace.ASPath)

	announcers, err := topology.Announcers(handInside)
	require.NoError(t, err)
	assert.Empty(t, announcers)
}

// TestTraceLeavesTowardExternalAddress verifies a border router that keeps the
// neighbor's next hop.
//
// VALIDATES: the third end of a trace — a next hop that is no router's address
// leaves the AS at the router announcing it, toward that address.
// PREVENTS: an unchanged external next hop read as a broken chain.
func TestTraceLeavesTowardExternalAddress(t *testing.T) {
	spec := handTopologySpec()
	spec.Neighbors[1].BorderNextHop = NextHopPolicyUnchanged
	topology, err := NewTopology(spec)
	require.NoError(t, err)

	exit, err := topology.Trace(handBorder2, handOneExit)
	require.NoError(t, err)
	assert.Equal(t, Trace{
		Routers: []netip.Addr{handBorder2},
		End:     TraceEndExitExternalAddress,
		NextHop: handTransitB,
		ASPath:  []uint32{65102},
		Reason:  ChoiceReasonOnlyRoute,
	}, exit)

	// The reflector still rewrites what it reflects, and passes the collector
	// the external address it received.
	fromClient, err := topology.Trace(handClient, handOneExit)
	require.NoError(t, err)
	assert.Equal(t, Trace{
		Routers: []netip.Addr{handClient, handReflector},
		End:     TraceEndExitExternalAddress,
		NextHop: handTransitB,
		ASPath:  []uint32{65102},
		Reason:  ChoiceReasonOnlyRoute,
	}, fromClient)
}

// TestNewTopologyRefusesInvalidSpec verifies that a network the model cannot
// compute is refused with the reason.
//
// VALIDATES: every input the model would otherwise answer wrongly for is an
// error naming what is wrong.
// PREVENTS: a defaulted next-hop policy, a second reflection level or a
// duplicate address giving a plausible network.
func TestNewTopologyRefusesInvalidSpec(t *testing.T) {
	tests := []struct {
		name   string
		change func(spec *TopologySpec)
		want   string
	}{
		{"no local AS", func(spec *TopologySpec) { spec.LocalAS = 0 }, "the local AS is 0"},
		{"no router", func(spec *TopologySpec) { spec.Routers = nil }, "no router"},
		{
			"two routers with one loopback",
			func(spec *TopologySpec) { spec.Routers[1].Loopback = handBorder1 },
			"share loopback 10.0.0.1",
		},
		{
			"an IPv6 loopback",
			func(spec *TopologySpec) { spec.Routers[0].Loopback = netip.MustParseAddr("2001:db8::1") },
			"is no IPv4 address",
		},
		{
			"a client that is no router",
			func(spec *TopologySpec) { spec.Routers[2].Clients = []netip.Addr{handTransitA} },
			"which is no router's loopback",
		},
		{
			"a reflector that is its own client",
			func(spec *TopologySpec) { spec.Routers[2].Clients = []netip.Addr{handReflector} },
			"lists itself as a client",
		},
		{
			"a client of two reflectors",
			func(spec *TopologySpec) { spec.Routers[0].Clients = []netip.Addr{handClient} },
			"keep it in one client list",
		},
		{
			"a reflector that is a client",
			func(spec *TopologySpec) { spec.Routers[3].Clients = []netip.Addr{handBorder1} },
			"is itself a client",
		},
		{
			"no next-hop policy",
			func(spec *TopologySpec) { spec.Neighbors[0].BorderNextHop = NextHopPolicyUnspecified },
			"set BorderNextHop to NextHopPolicySelf or NextHopPolicyUnchanged",
		},
		{
			"an unknown next-hop policy",
			func(spec *TopologySpec) { spec.Neighbors[0].BorderNextHop = NextHopPolicy(9) },
			"is no known policy",
		},
		{
			"a neighbor in the local AS",
			func(spec *TopologySpec) { spec.Neighbors[0].AS = handLocalAS },
			"has the local AS 65000",
		},
		{
			"a neighbor attached to no router",
			func(spec *TopologySpec) { spec.Neighbors[0].Border = handTransitB },
			"which is no router's loopback",
		},
		{
			"a neighbor at a router's address",
			func(spec *TopologySpec) { spec.Neighbors[0].Address = handClient },
			"has the address of a router of the AS",
		},
		{
			"two neighbors at one address",
			func(spec *TopologySpec) { spec.Neighbors[1].Address = handTransitA },
			"share address 192.0.2.1",
		},
		{
			"the local AS in a path",
			func(spec *TopologySpec) { spec.Neighbors[0].Announces[0].ASPathBeyond = []uint32{handLocalAS} },
			"refuses as a loop",
		},
		{
			"a prefix with host bits",
			func(spec *TopologySpec) {
				spec.Neighbors[0].Announces[0].Prefix = netip.MustParsePrefix("198.51.100.1/24")
			},
			"write it as 198.51.100.0/24",
		},
		{
			"a prefix announced twice by one neighbor",
			func(spec *TopologySpec) {
				spec.Neighbors[0].Announces = append(spec.Neighbors[0].Announces, spec.Neighbors[0].Announces[0])
			},
			"twice",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := handTopologySpec()
			test.change(&spec)
			_, err := NewTopology(spec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

// TestTopologyRefusesUnknownQuestion verifies that a question about a router
// or a prefix the network does not hold is an error.
//
// VALIDATES: an unknown prefix or router is refused.
// PREVENTS: an empty trace or an empty announcer list read as an answer.
func TestTopologyRefusesUnknownQuestion(t *testing.T) {
	topology, err := NewTopology(handTopologySpec())
	require.NoError(t, err)

	_, err = topology.Announcers(handInside)
	require.Error(t, err)
	_, err = topology.Trace(handClient, handInside)
	require.Error(t, err)
	// An external neighbor is no router of the AS, so it has no trace.
	_, err = topology.Trace(handTransitA, handTwoExits)
	require.Error(t, err)
}
