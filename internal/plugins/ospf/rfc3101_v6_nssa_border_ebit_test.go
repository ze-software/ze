// Design: docs/architecture/ospf/ospfv3-5-nssa-redist.md -- OSPFv3 NSSA, the border router E-bit.
// Related: origination_v6.go -- v6OriginateSelf, the OSPFv3 self-origination this drives.
// Related: lsdb/origination.go -- NSSABorderEBit, the per-area decision it calls.
//
// VALIDATES: RFC 3101 sec 3.1 on the OSPFv3 path (RFC 5340 carries the NSSA rules to
// OSPFv3). An OSPFv3 border router attached to an NSSA sets the E-bit in the Router-LSA it
// originates in every directly attached non-stub area: the backbone, a normal area and the
// NSSA itself, and not in an attached stub area. The same router with no NSSA attached sets
// it in none. The E-bit is read as the literal 0x02 bit of the flags octet that follows the
// 20-octet OSPFv3 LSA header (RFC 5340 sec A.4.3), from the LSA v6OriginateSelf installs;
// nothing passes the decision in by hand.
// PREVENTS: an OSPFv3 origination that sets the E-bit only in the backbone, only in the
// NSSA, or for a router with no NSSA attached.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc3101V6FlagsOctet is the offset of the Router-LSA flags octet: the OSPFv3 LSA header is
// 20 octets and the flags octet opens the body (RFC 5340 sec A.4.3).
const rfc3101V6FlagsOctet = 20

// rfc3101V6EBit is the E-bit of the OSPFv3 Router-LSA flags octet (0 0 0 Nt x V E B).
const rfc3101V6EBit = 0x02

// rfc3101V6BorderEngine is an OSPFv3 engine attached to the backbone (eth0), area 0.0.0.1 of
// the given kind (eth1), the normal area 0.0.0.2 (eth2) and the stub area 0.0.0.3 (eth3).
// The interfaces are passive, so each area is active without an adjacency, and the router
// is an area border router.
func rfc3101V6BorderEngine(t *testing.T, area1 areaType) (*engine, types.RouterID) {
	t.Helper()
	e := newEngineWithCodecAF(transport.New(&fakeBackend{}), v6Codec{}, afIPv6Unicast)
	router := types.RouterID{1, 1, 1, 1}
	cfg := ospfConfig{RouterID: router, Areas: []areaConfig{
		{AreaID: types.BackboneArea, AreaType: types.AreaTypeNormal},
		{AreaID: types.AreaID{0, 0, 0, 1}, AreaType: area1},
		{AreaID: types.AreaID{0, 0, 0, 2}, AreaType: types.AreaTypeNormal},
		{AreaID: types.AreaID{0, 0, 0, 3}, AreaType: types.AreaTypeStub},
	}}
	e.mu.Lock()
	e.cfg = cfg
	for name, area := range map[string]types.AreaID{
		"eth0": types.BackboneArea,
		"eth1": {0, 0, 0, 1},
		"eth2": {0, 0, 0, 2},
		"eth3": {0, 0, 0, 3},
	} {
		e.running[name] = interfaceConfig{Name: name, AreaID: area, Enabled: true, Passive: true, NetworkType: types.NetworkPointToPoint}
	}
	e.mu.Unlock()
	e.lsdb.SetSelfRouterID(router)
	if n := e.v6OriginateSelf(router, false); n == 0 {
		t.Fatal("v6OriginateSelf originated nothing")
	}
	return e, router
}

// rfc3101V6EBitSet reads the E-bit of the Router-LSA the engine installed in area.
func rfc3101V6EBitSet(t *testing.T, e *engine, router types.RouterID, area types.AreaID) bool {
	t.Helper()
	lsa, ok := e.lsdb.LookupLSA(area, v6RouterKey(router))
	if !ok {
		t.Fatalf("area %s: no self Router-LSA installed", area)
	}
	if len(lsa.RawBytes) <= rfc3101V6FlagsOctet {
		t.Fatalf("area %s: Router-LSA is %d octets, no flags octet", area, len(lsa.RawBytes))
	}
	return lsa.RawBytes[rfc3101V6FlagsOctet]&rfc3101V6EBit != 0
}

// RFC requirement: RFC3101-3.1-1 positive -- an OSPFv3 area border router attached to the
// NSSA 0.0.0.1 sets the E-bit (literal 0x02 of the flags octet) in the Router-LSA that
// v6OriginateSelf installs in each directly attached non-stub area: the backbone, the normal
// area 0.0.0.2 and the NSSA 0.0.0.1, with no AS-external origination; the stub area 0.0.0.3
// gets none.
func TestRFC3101V6NSSABorderEBitInEveryNonStubArea(t *testing.T) {
	// Goal: the per-area E-bit on the OSPFv3 origination. Method: four passive areas, one
	// NSSA, v6OriginateSelf, read the flags octet of each installed Router-LSA.
	e, router := rfc3101V6BorderEngine(t, types.AreaTypeNSSA)
	for _, area := range []types.AreaID{types.BackboneArea, {0, 0, 0, 1}, {0, 0, 0, 2}} {
		if !rfc3101V6EBitSet(t, e, router, area) {
			t.Errorf("area %s: E-bit clear in the Router-LSA of an NSSA border router's non-stub area", area)
		}
	}
	if rfc3101V6EBitSet(t, e, router, types.AreaID{0, 0, 0, 3}) {
		t.Error("area 0.0.0.3 (stub): E-bit set in a stub area's Router-LSA")
	}
}

// RFC requirement: RFC3101-3.1-1 negative -- the same OSPFv3 area border router with area
// 0.0.0.1 normal (no NSSA attached, no AS-external origination) sets the E-bit in none of
// its Router-LSAs.
func TestRFC3101V6NoNSSAAttachedEBitClearEverywhere(t *testing.T) {
	// Goal: the E-bit is the NSSA border rule, not a property of every ABR. Method: the same
	// four areas with no NSSA, v6OriginateSelf, read each flags octet.
	e, router := rfc3101V6BorderEngine(t, types.AreaTypeNormal)
	for _, area := range []types.AreaID{types.BackboneArea, {0, 0, 0, 1}, {0, 0, 0, 2}, {0, 0, 0, 3}} {
		if rfc3101V6EBitSet(t, e, router, area) {
			t.Errorf("area %s: E-bit set with no NSSA attached", area)
		}
	}
}
