// Design: plan/immediate/spec-fib-depth.md -- ECMP grouping
// Related: sysrib.go -- recomputeBest calls into ecmpGroup after selecting best

package sysrib

import (
	"cmp"
	"net/netip"
	"slices"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/rib/nexthop"
)

// Intra-protocol equal-cost sibling next-hops (IS-IS ECMP, umbrella A-2) used
// to be recomputed here via a per-change Loc-RIB Lookup. They are now computed
// at Loc-RIB emit (locrib.siblingNextHops) and carried on locrib.Change.ECMP,
// reaching this package as protocolRoute.ecmpNextHops; ecmpCollect below folds
// them into the kernel multipath. No re-lookup happens on the best-change path.

// ecmpCollect gathers all routes for a prefix that are equal-cost to the
// winner (same effective priority and metric). It includes both INTER-protocol
// equal-cost routes (other protocols in protocols at the same priority/metric)
// and the winner's INTRA-protocol equal-cost siblings (winner.ecmpNextHops, the
// Loc-RIB path-group expansion for protocols like IS-IS that insert one Path per
// next-hop; isis-9, umbrella A-2). Returns nil when no sibling next-hop exists.
// The winner's own next-hop is NOT included (it stays in BestChangeEntry.NextHop).
func ecmpCollect(protocols map[string]*protocolRoute, winner *protocolRoute) []sysribevents.ECMPPath {
	var paths []sysribevents.ECMPPath
	for _, route := range protocols {
		if route == winner {
			continue
		}
		if route.priority != winner.priority {
			continue
		}
		if route.metric != winner.metric {
			continue
		}
		if !namesATarget(route.nextHop, route.nextHopInterface) {
			continue
		}
		paths = append(paths, sysribevents.ECMPPath{
			NextHop:   route.nextHop,
			Interface: route.nextHopInterface,
			Weight:    ecmpWeight(route.nextHopWeight),
			Labels:    route.labels,
		})
	}
	// Intra-protocol equal-cost siblings of the winner (same source, same
	// admin distance + metric, different next-hop), recovered from the Loc-RIB
	// path-group. These share the winner's labels (a labeled ECMP group imposes
	// the same stack on every member today).
	for _, nh := range winner.ecmpNextHops {
		if sameTarget(nh, winner) || !namesATarget(nh.Addr, nh.Interface) {
			continue
		}
		paths = append(paths, sysribevents.ECMPPath{
			NextHop:   nh.Addr,
			Interface: nh.Interface,
			Weight:    ecmpWeight(nh.Weight),
			Labels:    winner.labels,
		})
	}
	return finishECMP(paths)
}

// ecmpWeight renders a producer's declared share for the FIB. A producer that
// states no weight sends zero, and every member of the group then takes an equal
// share -- which the FIB spells as 1, the value every group carried before a
// producer could state one. Mapping it here rather than at each call site keeps
// an unweighted group byte-identical for the protocols this arbitration does not
// otherwise touch.
func ecmpWeight(declared uint8) uint8 {
	if declared == 0 {
		return 1
	}
	return declared
}

// namesATarget reports whether a next-hop names somewhere to forward to: a
// gateway address, an outgoing device, or both. A member that names neither is
// not a next-hop and never enters a multipath group.
func namesATarget(addr netip.Addr, iface string) bool {
	return addr.IsValid() || iface != ""
}

// sameTarget reports whether a sibling next-hop is the winner's own. The winner
// stays in BestChangeEntry.NextHop, so the group lists the others; a device-only
// next-hop is told apart by its device, because its address is invalid on both
// sides and would otherwise compare equal to every other device-only sibling.
func sameTarget(nh nexthop.NextHop, winner *protocolRoute) bool {
	return nh.Addr == winner.nextHop && nh.Interface == winner.nextHopInterface
}

// finishECMP sorts, dedups and bounds a collected group. Both collectors end
// this way, so the ordering the FIB sees does not depend on which one ran.
func finishECMP(paths []sysribevents.ECMPPath) []sysribevents.ECMPPath {
	if len(paths) == 0 {
		return nil
	}
	slices.SortFunc(paths, ecmpPathCompare)
	paths = dedupECMP(paths)
	if len(paths) > sysribevents.MaxECMPPaths-1 {
		paths = paths[:sysribevents.MaxECMPPaths-1]
	}
	return paths
}

// backupPaths returns the winner's fast-reroute backup as a DEDICATED next-hop
// list (never merged into ecmpCollect). A backup is used only on primary
// failure; ECMP siblings load-share. Nil when the route has no backup.
func backupPaths(route *protocolRoute) []sysribevents.ECMPPath {
	if route == nil || !route.backupNextHop.IsValid() {
		return nil
	}
	return []sysribevents.ECMPPath{{NextHop: route.backupNextHop, Weight: 1, Labels: route.backupLabels}}
}

// dedupKey is the identity of one member of a multipath group: the gateway and
// the outgoing device together. The device is part of it because a route may
// name several device-only next-hops, whose addresses are all invalid.
type dedupKey struct {
	addr  netip.Addr
	iface string
}

// dedupECMP removes duplicate next-hops from a sorted ECMP slice, keeping the
// first occurrence. A next-hop can appear both as an inter-protocol route and an
// intra-protocol sibling; the kernel multipath must list it once. Two members
// are the same next-hop only when their gateway AND their device agree.
func dedupECMP(paths []sysribevents.ECMPPath) []sysribevents.ECMPPath {
	if len(paths) <= 1 {
		return paths
	}
	key := func(p sysribevents.ECMPPath) dedupKey {
		return dedupKey{addr: p.NextHop, iface: p.Interface}
	}
	out := paths[:1]
	for _, p := range paths[1:] {
		if key(p) != key(out[len(out)-1]) {
			out = append(out, p)
		}
	}
	return out
}

// ecmpChanged reports whether two ECMP path sets differ. It compares the FULL
// path (next-hop, weight, and label stack), not the next-hop alone: a relabel
// of the same multipath (identical next-hops, new MPLS label stack) MUST be
// treated as a change so the kernel's stale label stack is replaced. Comparing
// next-hops only silently dropped relabels (isis-9 ECMP). The two slices are
// already next-hop-sorted by ecmpCollect / ecmpCollectResolved, but ecmpChanged
// sorts defensive copies so it does not depend on caller ordering.
func ecmpChanged(a, b []sysribevents.ECMPPath) bool {
	if len(a) != len(b) {
		return true
	}
	if len(a) == 0 {
		return false
	}
	sa := slices.Clone(a)
	sb := slices.Clone(b)
	slices.SortFunc(sa, ecmpPathCompare)
	slices.SortFunc(sb, ecmpPathCompare)
	for i := range sa {
		if !ecmpPathEqual(sa[i], sb[i]) {
			return true
		}
	}
	return false
}

// ecmpPathCompare orders ECMP paths by next-hop, then outgoing device, then
// weight, then label stack, giving a stable total order so two equal sets sort
// identically. The device is compared before the weight so that device-only
// members, whose addresses are all invalid, still sort apart from each other.
func ecmpPathCompare(a, b sysribevents.ECMPPath) int {
	if c := a.NextHop.Compare(b.NextHop); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Interface, b.Interface); c != 0 {
		return c
	}
	if a.Weight != b.Weight {
		if a.Weight < b.Weight {
			return -1
		}
		return 1
	}
	return slices.Compare(a.Labels, b.Labels)
}

// ecmpPathEqual reports whether two ECMP paths are identical in next-hop,
// outgoing device, weight, and label stack.
func ecmpPathEqual(a, b sysribevents.ECMPPath) bool {
	return a.NextHop == b.NextHop && a.Interface == b.Interface &&
		a.Weight == b.Weight && slices.Equal(a.Labels, b.Labels)
}
