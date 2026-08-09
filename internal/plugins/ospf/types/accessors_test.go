// Design: docs/architecture/ospf/ospf-1-types.md -- fixed identifier accessor tests mirroring ISIS types

package types

import "testing"

// VALIDATES: AC-13 - fixed identifiers expose copied byte slices and remain comparable values.
// PREVENTS: callers mutating a returned slice and accidentally changing the map key value.
func TestFixedIdentifierBytesReturnCopies(t *testing.T) {
	router := RouterID{10, 0, 0, 1}
	routerBytes := router.Bytes()
	routerBytes[0] = 192
	if router[0] != 10 {
		t.Fatalf("RouterID.Bytes returned aliased storage")
	}

	area := AreaID{0, 0, 0, 1}
	areaBytes := area.Bytes()
	areaBytes[3] = 2
	if area[3] != 1 {
		t.Fatalf("AreaID.Bytes returned aliased storage")
	}

	lsid := LinkStateID{192, 0, 2, 7}
	lsidBytes := lsid.Bytes()
	lsidBytes[3] = 8
	if lsid[3] != 7 {
		t.Fatalf("LinkStateID.Bytes returned aliased storage")
	}
}
