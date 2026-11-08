// Design: docs/architecture/policyroute/policy-routing.md -- netlink ip rule and route management

package iprule

import "testing"

// VALIDATES: A-6 of spec-ipsec-spd-selection: no two Ze producers allocate from
// overlapping routing table ranges (the VRF reservation, 1000 to 1999, is one of
// them), and none overlaps the kernel's own tables (253 to 255). Each range is non-empty. The boundary values of the IPsec guard range sit
// inside it, and one below and one above sit outside.
// Method: compare every declared range pairwise and against the fixed reservations.
// PREVENTS: an IPsec source-guard table landing on a policy routing auto table or a
// VRF table, which would steer one producer's traffic with the other's routes.
func TestTableRangesAreDisjoint(t *testing.T) {
	ranges := ZeTableRanges()
	reserved := []TableRange{{First: 253, Last: 255}}
	for i, r := range ranges {
		if r.First > r.Last {
			t.Errorf("range %d is empty: %d to %d", i, r.First, r.Last)
		}
		for j := i + 1; j < len(ranges); j++ {
			if r.Overlaps(ranges[j]) {
				t.Errorf("ranges %v and %v overlap", r, ranges[j])
			}
		}
		for _, other := range reserved {
			if r.Overlaps(other) {
				t.Errorf("range %v overlaps the reserved range %v", r, other)
			}
		}
	}
	guard := IPsecSourceGuardTables
	if !guard.Contains(guard.First) || !guard.Contains(guard.Last) {
		t.Errorf("guard range %v does not contain its own ends", guard)
	}
	if guard.Contains(guard.First-1) || guard.Contains(guard.Last+1) {
		t.Errorf("guard range %v contains a value beyond its ends", guard)
	}
}

// VALIDATES: Reserved answers the Ze range for every table inside one, at both ends,
// and nothing for the table just outside each end.
// PREVENTS: an operator table ID accepted inside a Ze range, which the producer of
// that range would then overwrite or flush.
func TestReservedNamesTheRangeAtItsEnds(t *testing.T) {
	for _, r := range ZeTableRanges() {
		for _, table := range []uint32{r.First, r.Last} {
			got, ok := Reserved(table)
			if !ok || got != r {
				t.Errorf("Reserved(%d) = %v, %v; want %v, true", table, got, ok, r)
			}
		}
		for _, table := range []uint32{r.First - 1, r.Last + 1} {
			if got, ok := Reserved(table); ok && got == r {
				t.Errorf("Reserved(%d) = %v; it is outside %v", table, got, r)
			}
		}
	}
	if _, ok := Reserved(4000); ok {
		t.Error("Reserved(4000) answered a range; no Ze range holds it")
	}
}
