// Design: docs/architecture/chaos-web-dashboard.md -- a hand-written network whose border router keeps the neighbour's next hop, run against a real ze
// Related: lg_simulated_network.go -- the run this network goes through (lgSimulatedRunNetworkPlan)
// Related: lg_simulated_trace.go -- the comparison of the trace graph with the model's traces

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"

	chaosscenario "github.com/ze-software/ze/internal/chaos/scenario"
)

// The hand-written network: three routers of one AS in a full mesh, and two
// external neighbours. The first border router keeps the next hop its
// neighbour sent, the second rewrites it to itself, so the one network holds
// the case under test and the case beside it.
//
// The generated network cannot hold the first case: its border routers always
// rewrite the next hop (generateTopologySpec,
// internal/chaos/scenario/topology_generate.go).
var (
	lgNextHopUnchangedBorderKeeps   = netip.MustParseAddr("198.51.100.1")
	lgNextHopUnchangedBorderSelf    = netip.MustParseAddr("198.51.100.2")
	lgNextHopUnchangedRouterInner   = netip.MustParseAddr("198.51.100.3")
	lgNextHopUnchangedNeighbourKept = netip.MustParseAddr("203.0.113.1")
	lgNextHopUnchangedNeighbourSelf = netip.MustParseAddr("203.0.113.2")
	lgNextHopUnchangedPrefixKept    = netip.MustParsePrefix("198.18.0.0/24")
	lgNextHopUnchangedPrefixSelf    = netip.MustParsePrefix("198.18.1.0/24")
)

const (
	lgNextHopUnchangedLocalAS         = 65000
	lgNextHopUnchangedNeighbourKeptAS = 65101
	lgNextHopUnchangedNeighbourSelfAS = 65102
	lgNextHopUnchangedOriginAS        = 65201
)

// lgNextHopUnchangedSpec answers the hand-written network.
func lgNextHopUnchangedSpec() chaosscenario.TopologySpec {
	return chaosscenario.TopologySpec{
		LocalAS: lgNextHopUnchangedLocalAS,
		Routers: []chaosscenario.TopologyRouter{
			{Loopback: lgNextHopUnchangedBorderKeeps},
			{Loopback: lgNextHopUnchangedBorderSelf},
			{Loopback: lgNextHopUnchangedRouterInner},
		},
		Neighbours: []chaosscenario.TopologyNeighbour{
			{
				Address: lgNextHopUnchangedNeighbourKept, AS: lgNextHopUnchangedNeighbourKeptAS,
				Border: lgNextHopUnchangedBorderKeeps, BorderNextHop: chaosscenario.NextHopPolicyUnchanged,
				Announces: []chaosscenario.NeighbourRoute{
					{Prefix: lgNextHopUnchangedPrefixKept, ASPathBeyond: []uint32{lgNextHopUnchangedOriginAS}},
				},
			},
			{
				Address: lgNextHopUnchangedNeighbourSelf, AS: lgNextHopUnchangedNeighbourSelfAS,
				Border: lgNextHopUnchangedBorderSelf, BorderNextHop: chaosscenario.NextHopPolicySelf,
				Announces: []chaosscenario.NeighbourRoute{{Prefix: lgNextHopUnchangedPrefixSelf}},
			},
		},
	}
}

// lgNextHopUnchangedHandChecked is what the trace graph must state for the network
// above, written out by hand from BGP's rules and never asked of the model, so
// the model and the looking glass cannot agree on a shared mistake.
//
// The kept prefix: the first border router passes the neighbour's address on,
// inside the AS and to the collector, so every router's next hop is that
// address, which is no router's. Each walk therefore leaves the AS at the
// router it started from, toward the neighbour.
//
// The rewritten prefix: the second border router writes itself, so the two
// other routers' walks cross to it and end there on its own address.
func lgNextHopUnchangedHandChecked() map[netip.Prefix]map[string]lgSimulatedTraceLine {
	keeps, self, inner := lgNextHopUnchangedBorderKeeps.String(), lgNextHopUnchangedBorderSelf.String(), lgNextHopUnchangedRouterInner.String()
	kept := func(router string) lgSimulatedTraceLine {
		return lgSimulatedTraceLine{
			routers: []string{router}, end: "exit-external-address",
			nextHop: lgNextHopUnchangedNeighbourKept.String(), asPath: []string{"65101", "65201"},
		}
	}
	rewritten := func(routers ...string) lgSimulatedTraceLine {
		return lgSimulatedTraceLine{routers: routers, end: "exit-own-address", nextHop: self, asPath: []string{"65102"}}
	}
	return map[netip.Prefix]map[string]lgSimulatedTraceLine{
		lgNextHopUnchangedPrefixKept: {keeps: kept(keeps), self: kept(self), inner: kept(inner)},
		lgNextHopUnchangedPrefixSelf: {keeps: rewritten(keeps, self), self: rewritten(self), inner: rewritten(inner, self)},
	}
}

// lgNextHopUnchangedTraceProblem compares the trace graph's text of one prefix with
// the hand-written lines, and answers what differs, "" when nothing does.
func lgNextHopUnchangedTraceProblem(text string, prefix netip.Prefix, want map[string]lgSimulatedTraceLine) string {
	read, err := lgSimulatedReadTrace(text, prefix)
	if err != nil {
		return err.Error()
	}
	if len(read.traces) != len(want) {
		return fmt.Sprintf("the trace graph of %s holds %d traces, and the network has %d routers:\n%s",
			prefix, len(read.traces), len(want), text)
	}
	for router, line := range want {
		got, drawn := read.traces[router]
		if !drawn {
			return fmt.Sprintf("the trace graph of %s holds no trace from %s:\n%s", prefix, router, text)
		}
		if problem := lgSimulatedTraceProblem(got, line); problem != "" {
			return fmt.Sprintf("the trace graph of %s: the trace from %s %s", prefix, router, problem)
		}
	}
	return ""
}

// lgNextHopUnchangedTraceGraph is the trace graph question of the hand-written
// network (A-10, AC-27): every walk is the model's, at least one of them
// leaves the AS toward an external address, and the two prefixes read as the
// lines written by hand above.
func lgNextHopUnchangedTraceGraph(ctx context.Context, base string, topology *chaosscenario.Topology) error {
	tally, err := lgSimulatedTraceCompare(ctx, base, topology)
	if err != nil {
		return err
	}
	if tally.externalExit == 0 {
		return errors.New("no walk of the network leaves the AS toward an external address, so the unchanged next hop is unproven")
	}

	for prefix, want := range lgNextHopUnchangedHandChecked() {
		text, err := lgSimulatedGraphAnswer(ctx, base, "trace", prefix)
		if err != nil {
			return err
		}
		if problem := lgNextHopUnchangedTraceProblem(text, prefix, want); problem != "" {
			return errors.New(problem)
		}
	}
	fmt.Fprintf(os.Stderr,
		"OK: the trace graph leaves the AS toward the external address a border router did not rewrite: "+
			"%d traces, %d ending at an external address\n",
		tally.traces, tally.externalExit)
	return nil
}

// lgNextHopUnchangedChecks are the questions asked of the hand-written network. The
// search page is not among them: its comparison is proven on the generated
// network, and its hand check states that network's shape.
var lgNextHopUnchangedChecks = []lgSimulatedCheck{
	{name: "peers page", ask: lgSimulatedPeersPage},
	{name: "next-hop graph", ask: lgSimulatedNextHopGraph},
	{name: "trace graph", ask: lgNextHopUnchangedTraceGraph},
}

// lgSimulatedNextHopUnchangedFixture runs the hand-written network against a real ze
// and asks the looking glass where each walk leaves the AS.
//
// Arguments: as lgSimulatedNetworkFixture.
func lgSimulatedNextHopUnchangedFixture(ctx context.Context, args []string) error {
	options, err := lgSimulatedOptions("plugin/lg-simulated-unchanged-next-hop", args)
	if err != nil {
		return err
	}
	topology, err := chaosscenario.NewTopology(lgNextHopUnchangedSpec())
	if err != nil {
		return err
	}
	return lgSimulatedRunNetworkPlan(ctx, options, lgSimulatedNetworkPlan{topology: topology, checks: lgNextHopUnchangedChecks}, nil)
}
