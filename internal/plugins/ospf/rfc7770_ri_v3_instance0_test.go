// Design: docs/architecture/ospf/ospf-ext-3-router-information.md -- OSPFv3 Router Information LSA origination.
// Related: origination_v6_ri.go -- v6OriginateRIScope, the OSPFv3 RI LSA producer.
// Related: ri.go -- buildRIInstances, the per-instance body builder.
//
// VALIDATES: RFC 7770 Section 2.2, "The first Instance ID, i.e., 0, SHOULD always contain the
// Router Informational Capabilities TLV and, if advertised, the Router Functional
// Capabilities TLV", over the OSPFv3 RI LSA the engine installs in its own LSDB.
// PREVENTS: an OSPFv3 RI LSA whose Instance ID 0 lacks the capabilities TLVs, or carries
// them in another order, while the OSPFv2 twin (Section 2.1) is correct.
package ospf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// TestRFC7770V3InstanceZeroCarriesCapabilities checks the OSPFv3 RI LSA at Instance ID 0.
// Goal: the LSA this router floods with Link State ID 0 opens with the type-1 Router
// Informational Capabilities TLV and then the type-2 Router Functional Capabilities TLV.
// Method: originate the area-scope RI LSA through v6OriginateRIScope, read the installed
// LSA with Link State ID 0 back out of the LSDB, and decode the TLVs of its body.
func TestRFC7770V3InstanceZeroCarriesCapabilities(t *testing.T) {
	eng := newV6RIEngine(t)
	eng.setConfig(v6RIConfig(t, "area"))
	router := types.RouterID{1, 1, 1, 1}
	keep := map[ospflsdb.SelfLSARef]struct{}{}

	if n := eng.v6OriginateRIScope(router, OpaqueScopeArea, types.BackboneArea, keep); n == 0 {
		t.Fatal("v6OriginateRIScope(area) originated nothing")
	}
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, v6RIKey(router, ospfv3types.LSTypeRouterInformationArea, 0))
	if !ok {
		t.Fatal("no OSPFv3 RI LSA with Instance ID 0 is installed")
	}
	if len(lsa.RawBytes) < ospfv3packet.LSAHeaderLen {
		t.Fatalf("installed RI LSA is %d octets, shorter than its %d-octet header", len(lsa.RawBytes), ospfv3packet.LSAHeaderLen)
	}

	// RFC requirement: RFC7770-2.2-2 positive -- the OSPFv3 RI LSA this router originates with
	// Instance ID 0 carries the type-1 Router Informational Capabilities TLV first and the
	// type-2 Router Functional Capabilities TLV second.
	got := riTLVTypes(t, lsa.RawBytes[ospfv3packet.LSAHeaderLen:])
	if len(got) < 2 {
		t.Fatalf("Instance ID 0 carries TLV types %v, want type 1 then type 2 first", got)
	}
	if got[0] != packet.RITLVInformationalCapabilities {
		t.Fatalf("Instance ID 0 opens with TLV type %d, want %d (Informational Capabilities): %v", got[0], packet.RITLVInformationalCapabilities, got)
	}
	if got[1] != packet.RITLVFunctionalCapabilities {
		t.Fatalf("Instance ID 0 second TLV is type %d, want %d (Functional Capabilities): %v", got[1], packet.RITLVFunctionalCapabilities, got)
	}
}
