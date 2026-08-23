// VALIDATES: spec-ospf-ext-3 end-to-end user stories through the live engine -- the RI
// opaque consumer registers and is discovered; enabling RI originates an Opaque type-4 LSA
// (OSPFv2) / a function-code-12 LSA (OSPFv3) that `show ospf database router-information`
// decodes into capability bits; a registered TLV builder's TLV appears after the type-1 TLV;
// a received RI LSA is stored and rendered; and disabling RI withdraws it.
// PREVENTS: the RI consumer compiling but not wired to origination, the TLV hook, reception,
// the CLI view, or the withdraw path.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// riFnRegister builds a v4 engine from cfg with the RI opaque consumer registered.
func riFnRegister(t *testing.T, cfgJSON string) *engine {
	t.Helper()
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	eng, _ := newRedistEngine(t, cfgJSON)
	if err := registerRIConsumer(eng); err != nil {
		t.Fatalf("registerRIConsumer: %v", err)
	}
	return eng
}

func TestOSPFRIRegisterFunctional(t *testing.T) {
	eng := riFnRegister(t, riCfg(true, "area"))
	if !eng.cfg.Opaque {
		t.Fatalf("opaque leaf not parsed true (OSPFv2 RI requires opaque capability)")
	}
	if _, ok := lookupOpaqueConsumer(packet.RIOpaqueType); !ok {
		t.Fatalf("RI consumer (Opaque type 4) not discoverable")
	}
}

func TestOSPFRIOriginateFunctional(t *testing.T) {
	eng := riFnRegister(t, riCfg(true, "area", "as"))
	// The engine discovers the RI consumer and installs its opaque LSAs on the self-LSA pass.
	eng.originateSelfLSAs()

	view := riShowView(t, riDatabaseSnapshot(eng, nil))
	var area, as bool
	for _, e := range view.RouterInformation {
		if e.AF != "v2" {
			continue
		}
		switch e.Scope {
		case OpaqueScopeArea.String():
			area = true
		case OpaqueScopeAS.String():
			as = true
		}
		if len(e.Instances) == 0 || len(e.Instances[0].TLVs) == 0 {
			t.Fatalf("v2 RI entry %+v has no decoded TLVs", e)
		}
	}
	if !area || !as {
		t.Fatalf("RI default scope did not originate area + AS opaque LSAs: area=%v as=%v", area, as)
	}
}

func TestOSPFRI6OriginateFunctional(t *testing.T) {
	eng := newV6RIEngine(t)
	eng.setConfig(v6RIConfig(t, "area", "as"))
	router := types.RouterID{1, 1, 1, 1}
	keep := map[ospflsdb.SelfLSARef]struct{}{}
	eng.v6OriginateRIScope(router, OpaqueScopeArea, types.BackboneArea, keep)
	eng.v6OriginateRIScope(router, OpaqueScopeAS, types.BackboneArea, keep)

	view := riShowView(t, riDatabaseSnapshot(nil, eng))
	var area, as bool
	for _, e := range view.RouterInformation {
		if e.AF != "v3" {
			continue
		}
		if e.Scope == OpaqueScopeArea.String() {
			area = true
		}
		if e.Scope == OpaqueScopeAS.String() {
			as = true
		}
	}
	if !area || !as {
		t.Fatalf("OSPFv3 RI did not originate area + AS LSAs: area=%v as=%v", area, as)
	}
}

func TestOSPFRIRegisterTLVFunctional(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)
	// A test-stub downstream consumer (standing in for Segment Routing / ext-5) registers a TLV.
	if err := registerRITLV(8, OpaqueScopeArea, func(types.RouterID) []packet.RITLV {
		return []packet.RITLV{{Type: 8, Value: []byte{0xAB, 0xCD, 0xEF, 0x00}}}
	}); err != nil {
		t.Fatalf("registerRITLV: %v", err)
	}
	eng := riFnRegister(t, riCfg(true, "area"))
	eng.originateSelfLSAs()

	view := riShowView(t, riDatabaseSnapshot(eng, nil))
	var sawType8 bool
	for _, e := range view.RouterInformation {
		for _, inst := range e.Instances {
			// The type-1 TLV must be first, then the registered type-8 TLV (RFC 7770 sec 2.4).
			if len(inst.TLVs) == 0 || inst.TLVs[0].Type != packet.RITLVInformationalCapabilities {
				t.Fatalf("first TLV not type-1 in %+v", inst.TLVs)
			}
			for _, tlv := range inst.TLVs {
				if tlv.Type == 8 {
					sawType8 = true
				}
			}
		}
	}
	if !sawType8 {
		t.Fatalf("registered TLV (type 8) did not appear in the RI LSA")
	}
}

func TestOSPFRIReceiveFunctional(t *testing.T) {
	eng := riFnRegister(t, riCfg(true, "area"))
	a0 := types.BackboneArea
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{{
			Name: "eth0", AreaID: a0, AreaType: ospflsdb.AreaTypeNormal,
			NetworkType: ospflsdb.NetworkPointToPoint, State: ospflsdb.InterfaceStateDR,
			RouterID: mustRouterID(t, "1.1.1.1"), TransmitDelay: 1,
			Neighbors: []ospflsdb.NeighborInfo{{RouterID: mustRouterID(t, "2.2.2.2"), Address: naddrForTest("10.0.0.2"), State: ospflsdb.NeighborStateFull, OpaqueCapable: true}},
		}}
	})
	// A peer's RI opaque LSA (Opaque type 4, area scope) carrying the stub-router bit.
	riBody := packet.EncodeRITLVs([]packet.RITLV{{Type: packet.RITLVInformationalCapabilities, Value: packet.RICapabilitiesValue(packet.RIInfoBitMask(packet.RIInfoBitStubRouter))}})
	lsa := opaqueLSAForTest(t, types.LSTypeOpaqueArea, packet.RIOpaqueType, 0, mustRouterID(t, "2.2.2.2"), types.InitialSequenceNumber, riBody)
	reason := eng.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: mustRouterID(t, "2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	// The received RI LSA is stored and rendered with its decoded capability bits.
	view := riShowView(t, riDatabaseSnapshot(eng, nil))
	var found bool
	for _, e := range view.RouterInformation {
		if e.AdvertisingRouter == "2.2.2.2" && containsStr(e.Capabilities, "stub-router") {
			found = true
		}
	}
	if !found {
		t.Fatalf("received RI LSA from 2.2.2.2 not stored/rendered with stub-router: %+v", view.RouterInformation)
	}
}

func TestOSPFRIShowFunctional(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)
	// v2 with stub-router advertised.
	v4 := riFnRegister(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,`+
		`"max-metric":{"router-lsa":{"always":"true"}},`+
		`"router-information":{"enabled":true,"scope":"area"},`+
		`"areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0"}}}}}`)
	v4.originateSelfLSAs()
	// v3.
	v6 := newV6RIEngine(t)
	v6.setConfig(v6RIConfig(t, "area"))
	v6.v6OriginateRIScope(types.RouterID{1, 1, 1, 1}, OpaqueScopeArea, types.BackboneArea, map[ospflsdb.SelfLSARef]struct{}{})

	view := riShowView(t, riDatabaseSnapshot(v4, v6))
	var v2Caps, v3 bool
	for _, e := range view.RouterInformation {
		if e.AF == "v2" && containsStr(e.Capabilities, "stub-router") {
			v2Caps = true
		}
		if e.AF == "v3" {
			v3 = true
		}
	}
	if !v2Caps || !v3 {
		t.Fatalf("show did not render both AFs with bits: v2Caps=%v v3=%v", v2Caps, v3)
	}
}

func TestOSPFRIWithdrawFunctional(t *testing.T) {
	eng := riFnRegister(t, riCfg(true, "area", "as"))
	eng.originateSelfLSAs()
	if before := riDatabaseSnapshot(eng, nil); len(riShowView(t, before).RouterInformation) == 0 {
		t.Fatalf("no RI LSAs originated before withdraw")
	}
	// Disable RI and re-run the self-LSA pass: the opaque withdraw MaxAge-flushes the LSAs.
	offCfg, err := parseOSPFConfig(ospfSec(riCfg(false)), nil)
	if err != nil {
		t.Fatalf("parse off cfg: %v", err)
	}
	eng.setConfig(offCfg)
	eng.originateSelfLSAs()

	// The RI opaque LSAs are MaxAge in the store (purged), so the live population reads zero.
	for _, v := range eng.lsdb.OpaqueLSAsByType(packet.RIOpaqueType) {
		if v.Age != types.MaxAge {
			t.Fatalf("RI LSA (instance %d) not MaxAge-flushed after disable (age %d)", v.OpaqueID, v.Age)
		}
	}
}
