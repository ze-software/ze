// Design: plan/spec-static-routes.md -- diff engine for config reload

package static

import "slices"

func routesEqual(a, b staticRoute) bool {
	if a.Action != b.Action || a.Metric != b.Metric || a.Tag != b.Tag || a.Description != b.Description {
		return false
	}
	if len(a.NextHops) != len(b.NextHops) {
		return false
	}
	aSorted := sortedNextHops(a.NextHops)
	bSorted := sortedNextHops(b.NextHops)
	for i := range aSorted {
		if aSorted[i] != bSorted[i] {
			return false
		}
	}
	return true
}

func sortedNextHops(nhs []nextHop) []nextHop {
	sorted := make([]nextHop, len(nhs))
	copy(sorted, nhs)
	slices.SortFunc(sorted, func(a, b nextHop) int {
		return a.Address.Compare(b.Address)
	})
	return sorted
}
