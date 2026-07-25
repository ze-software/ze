// VALIDATES: spec-ospf-ext-14 AC-10/AC-11 -- the engine neighbor deep-dump returns each
// neighbor's full state with the Options field decoded per address family (OSPFv2 O-bit vs
// OSPFv3 R/V6/E/N/AF).
// PREVENTS: a detail view that shows the wrong option bits for the address family.
package ospf

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestNeighborDetailOptionBits(t *testing.T) {
	v2 := optionsV2Bits(uint32(types.OptionE | types.OptionO))
	if !hasOptBit(v2, "E") || !hasOptBit(v2, "O") {
		t.Fatalf("v2 option bits = %v, want E + O", v2)
	}
	v6 := optionsV6Bits(uint32(ospfv3types.OptV6 | ospfv3types.OptR | ospfv3types.OptAF))
	if !hasOptBit(v6, "V6") || !hasOptBit(v6, "R") || !hasOptBit(v6, "AF") {
		t.Fatalf("v6 option bits = %v, want V6 + R + AF", v6)
	}
}

func TestNeighborDetailSnapshotEngine(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	cfg := ospfneighbor.InterfaceConfig{
		Name: "eth0", AreaID: types.BackboneArea, RouterID: mustRouterID(t, "10.0.0.1"),
		NetworkType: "broadcast", DeadInterval: 40,
	}
	eng.neighbors.ConfigureInterface(cfg)
	peer := mustRouterID(t, "10.0.0.2")
	eng.neighbors.Hello(ospfneighbor.HelloInput{
		InterfaceName: "eth0", AreaID: cfg.AreaID, LocalRouterID: cfg.RouterID, NeighborID: peer,
		Address: netip.AddrFrom4([4]byte(peer)), Priority: 1, TwoWay: true,
		NetworkType: "broadcast", DeadInterval: 40, Now: time.Now(),
	})

	rows := eng.neighborDetailSnapshot()
	if len(rows) != 1 {
		t.Fatalf("neighbor detail rows = %d, want 1", len(rows))
	}
	v, ok := rows[0].(neighborDetailView)
	if !ok {
		t.Fatalf("row type = %T", rows[0])
	}
	if v.RouterID != "10.0.0.2" || v.State == "" || v.LastEvent == "" {
		t.Fatalf("neighbor detail = %+v", v)
	}
}

func hasOptBit(xs []string, want string) bool {
	return slices.Contains(xs, want)
}
