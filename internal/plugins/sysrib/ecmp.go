// Design: plan/spec-fib-depth.md -- ECMP grouping
// Related: sysrib.go -- recomputeBest calls into ecmpGroup after selecting best

package sysrib

import (
	"net/netip"
	"slices"

	sysribevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/events"
)

// ecmpCollect gathers all routes for a prefix that are equal-cost to the
// winner (same effective priority and metric). Returns nil if only one path
// exists (no ECMP). The winner itself is NOT included in the returned slice;
// it remains in BestChangeEntry.NextHop.
func ecmpCollect(protocols map[string]*protocolRoute, winner *protocolRoute) []sysribevents.ECMPPath {
	if len(protocols) <= 1 {
		return nil
	}

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
		if !route.nextHop.IsValid() {
			continue
		}
		paths = append(paths, sysribevents.ECMPPath{
			NextHop: route.nextHop,
			Weight:  1,
			Labels:  route.labels,
		})
	}
	if len(paths) == 0 {
		return nil
	}
	slices.SortFunc(paths, func(a, b sysribevents.ECMPPath) int {
		return a.NextHop.Compare(b.NextHop)
	})
	if len(paths) > sysribevents.MaxECMPPaths-1 {
		paths = paths[:sysribevents.MaxECMPPaths-1]
	}
	return paths
}

// ecmpChanged reports whether two ECMP path sets differ.
func ecmpChanged(a, b []sysribevents.ECMPPath) bool {
	if len(a) != len(b) {
		return true
	}
	if len(a) == 0 {
		return false
	}
	aSet := make(map[netip.Addr]struct{}, len(a))
	for _, p := range a {
		aSet[p.NextHop] = struct{}{}
	}
	for _, p := range b {
		if _, ok := aSet[p.NextHop]; !ok {
			return true
		}
	}
	return false
}
