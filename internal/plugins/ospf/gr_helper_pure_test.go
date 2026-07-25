// VALIDATES: pure graceful-restart helpers -- gr_restarter.go stringsToRouterIDs (parse a
// []string of dotted router IDs, dropping malformed entries) and gr_helper.go
// wouldFloodToHelper / sameHelperArea (RFC 3623 §3: whether a changed LSA of a given type
// would have flooded to the restarting neighbor X on the helper's segment; the AS-external
// stub/NSSA exception vs the area-scoped same-area rule).
// PREVENTS: a malformed router ID string aborting the whole parse; the flooding predicate
// treating an AS-external LSA as flooding into a stub/NSSA area, or an area-scoped LSA
// flooding to a neighbor on a different area's segment.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestStringsToRouterIDs(t *testing.T) {
	got := stringsToRouterIDs([]string{"1.1.1.1", "not-an-id", "2.2.2.2"})
	// The malformed entry is dropped; the two valid dotted quads parse in order.
	if len(got) != 2 {
		t.Fatalf("parsed %d router IDs, want 2 (malformed entry dropped): %v", len(got), got)
	}
	if got[0] != (types.RouterID{1, 1, 1, 1}) || got[1] != (types.RouterID{2, 2, 2, 2}) {
		t.Fatalf("parsed router IDs = %v, want [1.1.1.1 2.2.2.2]", got)
	}
	if out := stringsToRouterIDs(nil); len(out) != 0 {
		t.Fatalf("stringsToRouterIDs(nil) = %v, want empty", out)
	}
}

func TestWouldFloodToHelperASExternal(t *testing.T) {
	// An AS-external LSA floods to X in a normal area, but NOT in a stub or NSSA area
	// (RFC 3623 §3.2 exception). This branch never consults the engine, so e is nil.
	if !wouldFloodToHelper(types.LSTypeASExternal, areaTypeNormal, "eth0", types.AreaID{}, nil) {
		t.Fatalf("AS-external LSA must flood to a helper in a normal area")
	}
	if wouldFloodToHelper(types.LSTypeASExternal, areaTypeStub, "eth0", types.AreaID{}, nil) {
		t.Fatalf("AS-external LSA must NOT flood to a helper in a stub area")
	}
	if wouldFloodToHelper(types.LSTypeASExternal, areaTypeNSSA, "eth0", types.AreaID{}, nil) {
		t.Fatalf("AS-external LSA must NOT flood to a helper in an NSSA area")
	}
}

func TestWouldFloodToHelperAreaScoped(t *testing.T) {
	// An area-scoped LSA (Router-LSA) floods to X only on a segment in X's area.
	eng := newEngine(transport.New(&fakeBackend{}))
	area := types.AreaID{0, 0, 0, 1}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: area}

	if !wouldFloodToHelper(types.LSTypeRouter, areaTypeNormal, "eth0", area, eng) {
		t.Fatalf("a Router-LSA in eth0's own area must flood to the helper (sameHelperArea true)")
	}
	other := types.AreaID{0, 0, 0, 2}
	if wouldFloodToHelper(types.LSTypeRouter, areaTypeNormal, "eth0", other, eng) {
		t.Fatalf("a Router-LSA for a different area must NOT flood to eth0's helper")
	}
	// sameHelperArea returns false for an interface the engine is not running.
	if sameHelperArea(eng, "eth9", area) {
		t.Fatalf("sameHelperArea must be false for an unknown interface")
	}
}
