// Design: docs/architecture/static-routes.md -- diff engine for config reload

package static

import (
	"cmp"
	"slices"
)

func routesEqual(a, b staticRoute) bool {
	if a.Table != b.Table || a.Action != b.Action || a.Metric != b.Metric || a.Tag != b.Tag || a.Description != b.Description {
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
		if c := a.Address.Compare(b.Address); c != 0 {
			return c
		}
		return cmp.Compare(a.Interface, b.Interface)
	})
	return sorted
}
