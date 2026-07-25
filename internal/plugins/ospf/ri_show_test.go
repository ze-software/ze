// VALIDATES: spec-ospf-ext-3 AC-12/AC-13/AC-14 -- `show ospf database router-information`
// decodes the stored RI LSA bodies for both address families into named capability bits and a
// TLV list, and a truncated/malformed body renders what it can without crashing.
// PREVENTS: a show that shows only one address family, hides the capability bits, or panics on
// a hostile received RI LSA.
package ospf

import (
	"slices"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func riShowView(t *testing.T, rows []any) riDatabaseView {
	t.Helper()
	if len(rows) != 1 {
		t.Fatalf("RI snapshot wrapping = %d, want 1", len(rows))
	}
	view, ok := rows[0].(riDatabaseView)
	if !ok {
		t.Fatalf("RI snapshot is not a riDatabaseView: %T", rows[0])
	}
	return view
}

func TestRIShowRender(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	resetRITLVs()
	t.Cleanup(resetRITLVs)

	// v2 engine: RI enabled with max-metric so the stub-router bit is advertised.
	v4, _ := newRedistEngine(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,`+
		`"max-metric":{"router-lsa":{"always":"true"}},`+
		`"router-information":{"enabled":true,"scope":["area"]},`+
		`"areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0"}}}}}`)
	if err := registerRIConsumer(v4); err != nil {
		t.Fatalf("registerRIConsumer: %v", err)
	}
	v4.originateSelfLSAs()

	// v3 engine: RI enabled at area scope.
	v6 := newV6RIEngine(t)
	v6.setConfig(v6RIConfig(t, "area"))
	v6.v6OriginateRIScope(types.RouterID{1, 1, 1, 1}, OpaqueScopeArea, types.BackboneArea, map[ospflsdb.SelfLSARef]struct{}{})

	view := riShowView(t, riDatabaseSnapshot(v4, v6))
	var haveV2, haveV3 bool
	for _, e := range view.RouterInformation {
		switch e.AF {
		case "v2":
			haveV2 = true
			if !containsStr(e.Capabilities, "stub-router") {
				t.Errorf("v2 RI capabilities = %v, want stub-router", e.Capabilities)
			}
			if len(e.Instances) == 0 || len(e.Instances[0].TLVs) == 0 {
				t.Errorf("v2 RI entry has no decoded TLVs: %+v", e)
			}
		case "v3":
			haveV3 = true
		}
	}
	if !haveV2 || !haveV3 {
		t.Fatalf("RI view missing an address family: v2=%v v3=%v (%+v)", haveV2, haveV3, view.RouterInformation)
	}
}

func TestRIShowMalformedTLV(t *testing.T) {
	// A valid type-1 TLV followed by a truncated second TLV: the renderer decodes the first
	// and flags the body malformed, never panicking (AC-14, bound-checked iterator).
	good := packet.EncodeRITLVs([]packet.RITLV{{Type: packet.RITLVInformationalCapabilities, Value: packet.RICapabilitiesValue(packet.RIInfoBitMask(packet.RIInfoBitStubRouter))}})
	body := append([]byte(nil), good...)
	body = append(body, 0x00, 0x08, 0xFF, 0xFF, 0x01) // a TLV header claiming a length past the end
	caps, tlvs, malformed := decodeRIBody(body)
	if !malformed {
		t.Fatalf("truncated RI body not flagged malformed")
	}
	if len(tlvs) != 1 || tlvs[0].Type != packet.RITLVInformationalCapabilities {
		t.Fatalf("partial decode = %+v, want the leading type-1 TLV", tlvs)
	}
	if caps&packet.RIInfoBitMask(packet.RIInfoBitStubRouter) == 0 {
		t.Fatalf("leading type-1 capabilities not decoded: %#08x", caps)
	}
}

func containsStr(s []string, want string) bool { return slices.Contains(s, want) }
