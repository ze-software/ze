// VALIDATES: spec-ospf-ext-3 AC-1/AC-2, A-1/A-2 -- OSPFv2 RI rides the ext-1 opaque carrier:
// the RI consumer registers Opaque type 4; enabling RI originates an opaque type-4 LSA whose
// Link State ID is 4<<24 | InstanceID (Instance 0 -> 4.0.0.0); area scope uses LS type 10 and
// AS scope LS type 11.
// PREVENTS: RI failing to register as an opaque consumer, a wrong Opaque type / Instance-ID
// byte order, or origination at the wrong flooding scope.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestRIOpaqueConsumerRegistered(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	eng := newEngine(transport.New(&fakeBackend{}))
	if err := registerRIConsumer(eng); err != nil {
		t.Fatalf("registerRIConsumer: %v", err)
	}
	c, ok := lookupOpaqueConsumer(packet.RIOpaqueType)
	if !ok {
		t.Fatalf("RI consumer not registered for Opaque type %d", packet.RIOpaqueType)
	}
	if c.scope != OpaqueScopeArea {
		t.Fatalf("RI default scope = %v, want area", c.scope)
	}
}

// riOpaqueByScope returns the originated RI opaque LSAs (Opaque type 4) at a given scope.
func riOpaqueByScope(eng *engine, scope OpaqueScope) []uint32 {
	var insts []uint32
	for _, v := range eng.lsdb.OpaqueLSAsByType(packet.RIOpaqueType) {
		if OpaqueScope(v.Scope) == scope {
			insts = append(insts, v.OpaqueID)
		}
	}
	return insts
}

func TestRIv2OriginateArea(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	eng, _ := newRedistEngine(t, riCfg(true, "area"))
	if err := registerRIConsumer(eng); err != nil {
		t.Fatalf("registerRIConsumer: %v", err)
	}
	eng.originateSelfLSAs()

	// One area-scope (Type 10) RI opaque LSA at Instance 0, and none at AS scope.
	area := riOpaqueByScope(eng, OpaqueScopeArea)
	if len(area) != 1 || area[0] != 0 {
		t.Fatalf("area-scope RI instances = %v, want [0]", area)
	}
	if as := riOpaqueByScope(eng, OpaqueScopeAS); len(as) != 0 {
		t.Fatalf("area-only config originated an AS-scope RI LSA: %v", as)
	}
}

func TestRIv2OriginateAS(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	eng, _ := newRedistEngine(t, riCfg(true, "as"))
	if err := registerRIConsumer(eng); err != nil {
		t.Fatalf("registerRIConsumer: %v", err)
	}
	eng.originateSelfLSAs()

	if as := riOpaqueByScope(eng, OpaqueScopeAS); len(as) != 1 || as[0] != 0 {
		t.Fatalf("AS-scope RI instances = %v, want [0] (Type 11)", as)
	}
}

// TestRIv2NoDoubleEmitIntoNSSA is the ext-3 review regression: with the default scope
// [area, as] and an attached NSSA area, the AS-scope branch's RFC 7770 sec 2.7 NSSA
// fallback must NOT re-emit an area-scoped RI into an NSSA the Area-scope branch already
// covered. Before the fix each attached NSSA got two identical (scope=area) originations,
// double-counting ze_ospf_ri_originations_total{scope=area}.
func TestRIv2NoDoubleEmitIntoNSSA(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	eng, router := newRedistEngine(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,`+
		`"router-information":{"enabled":true,"scope":["area","as"]},`+
		`"areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa"}}},`+
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0","network-type":"point-to-point"},`+
		`"eth1":{"area":"0.0.0.5","network-type":"point-to-point"}}}}}`)
	if err := registerRIConsumer(eng); err != nil {
		t.Fatalf("registerRIConsumer: %v", err)
	}
	seen := map[types.AreaID]int{}
	for _, o := range eng.riOriginate(router) {
		if o.Scope == OpaqueScopeArea && !o.Withdraw {
			seen[o.Area]++
		}
	}
	if len(seen) != 2 {
		t.Fatalf("area-scope RI covered %d areas, want 2 (backbone + NSSA)", len(seen))
	}
	for area, n := range seen {
		if n != 1 {
			t.Fatalf("area-scope RI for area %v emitted %d times, want 1 (NSSA double-emit regression)", area, n)
		}
	}
}

func TestRIv2InstanceIDIsOpaqueID(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	eng, _ := newRedistEngine(t, riCfg(true, "area"))
	if err := registerRIConsumer(eng); err != nil {
		t.Fatalf("registerRIConsumer: %v", err)
	}
	eng.originateSelfLSAs()

	views := eng.lsdb.OpaqueLSAsByType(packet.RIOpaqueType)
	if len(views) != 1 {
		t.Fatalf("RI opaque LSAs = %d, want 1", len(views))
	}
	v := views[0]
	if v.OpaqueType != packet.RIOpaqueType {
		t.Fatalf("Opaque type = %d, want 4", v.OpaqueType)
	}
	if v.OpaqueID != 0 {
		t.Fatalf("Instance 0 Opaque ID = %d, want 0", v.OpaqueID)
	}
	// A-2/R-1: Instance 0 -> Link State ID 0x04000000 (Opaque type 4 high byte, Instance ID 0).
	lsid := packet.OpaqueLinkStateID(packet.RIOpaqueType, v.OpaqueID)
	if want := (types.LinkStateID{0x04, 0x00, 0x00, 0x00}); lsid != want {
		t.Fatalf("RI Link State ID = %v, want 4.0.0.0", lsid)
	}
}
