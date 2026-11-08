// Design: docs/architecture/policyroute/policy-routing.md -- netlink ip rule and route management

package iprule

import "testing"

// VALIDATES: A-6 of spec-ipsec-spd-selection: no two Ze producers allocate from
// overlapping routing table ranges, and none overlaps the operator side the policy
// routing page reserves (VRF tables 1000 to 1999) or the kernel's own tables (253 to
// 255). Each range is non-empty. The boundary values of the IPsec guard range sit
// inside it, and one below and one above sit outside.
// Method: compare every declared range pairwise and against the fixed reservations.
// PREVENTS: an IPsec source-guard table landing on a policy routing auto table or a
// VRF table, which would steer one producer's traffic with the other's routes.
func TestTableRangesAreDisjoint(t *testing.T) {
	ranges := ZeTableRanges()
	reserved := []TableRange{{First: 1000, Last: 1999}, {First: 253, Last: 255}}
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
