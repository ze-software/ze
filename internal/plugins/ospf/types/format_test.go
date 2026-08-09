// Design: docs/architecture/ospf/ospf-1-types.md -- formatting allocation tests

package types

import "testing"

// VALIDATES: AC-1, AC-3, AC-12 - AppendTo for dotted-quad identifiers allocates zero.
// PREVENTS: hot CLI/database listing paths from allocating per identifier component.
func TestStringNoAlloc(t *testing.T) {
	router := RouterID{10, 0, 0, 1}
	area := AreaID{0, 0, 0, 1}
	lsid := LinkStateID{192, 0, 2, 7}
	if allocs := testing.AllocsPerRun(1000, func() {
		var buf [dottedQuadLen]byte
		_ = router.AppendTo(buf[:0])
	}); allocs != 0 {
		t.Fatalf("RouterID.AppendTo allocated %.2f times, want 0", allocs)
	}
	if allocs := testing.AllocsPerRun(1000, func() {
		var buf [dottedQuadLen]byte
		_ = area.AppendTo(buf[:0])
	}); allocs != 0 {
		t.Fatalf("AreaID.AppendTo allocated %.2f times, want 0", allocs)
	}
	if allocs := testing.AllocsPerRun(1000, func() {
		var buf [dottedQuadLen]byte
		_ = lsid.AppendTo(buf[:0])
	}); allocs != 0 {
		t.Fatalf("LinkStateID.AppendTo allocated %.2f times, want 0", allocs)
	}
	if allocs := testing.AllocsPerRun(1000, func() { _ = router.String() }); allocs > 1 {
		t.Fatalf("RouterID.String allocated %.2f times, want <= 1", allocs)
	}
}
