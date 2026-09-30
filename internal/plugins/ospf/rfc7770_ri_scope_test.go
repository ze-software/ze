// Design: docs/architecture/ospf/ospf-ext-3-router-information.md -- RI informational capabilities per flooding scope.
// Related: ri.go -- deriveRICapabilities and riOriginate, the capability source and the per-area emission.
//
// VALIDATES: RFC 7770 Section 2.4, "the TLV MUST accurately reflect the OSPF router's
// capabilities in the scope advertised": an area-scope RI LSA flooded into an area where no
// interface runs traffic engineering does not claim the TE informational capability.
// PREVENTS: one router-wide capability word copied into every area's RI LSA.
package ospf

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestRFC7770AreaRICapabilitiesFollowTheArea configures TE on eth0 (area 0) only, with eth1 in
// area 5, and reads the Informational Capabilities word of each area-scope RI LSA the engine
// originates.
func TestRFC7770AreaRICapabilitiesFollowTheArea(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)
	router := types.RouterID{1, 1, 1, 1}
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,`+
		`"router-information":{"enabled":true,"scope":["area"]},`+
		`"areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},"0.0.0.5":{"area-id":"0.0.0.5"}}},`+
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0","network-type":"point-to-point","traffic-engineering":{"enable":true}},`+
		`"eth1":{"area":"0.0.0.5","network-type":"point-to-point"}}}}}`)
	te := packet.RIInfoBitMask(packet.RIInfoBitTrafficEngineering)
	claims := map[types.AreaID]bool{}
	for _, o := range eng.riOriginate(router) {
		if o.Scope != OpaqueScopeArea || o.Withdraw || o.OpaqueID != 0 {
			continue
		}
		// Instance 0 opens with the type-1 TLV: Type (2), Length (2), then the 32-bit word.
		if len(o.Body) < 8 || binary.BigEndian.Uint16(o.Body[0:2]) != packet.RITLVInformationalCapabilities {
			t.Fatalf("area %s RI instance 0 does not open with the Informational Capabilities TLV", o.Area)
		}
		claims[o.Area] = binary.BigEndian.Uint32(o.Body[4:8])&te != 0
	}
	backbone, other := types.BackboneArea, types.AreaID{0, 0, 0, 5}
	if len(claims) != 2 {
		t.Fatalf("area-scope RI originated into %d areas, want 2", len(claims))
	}
	// RFC requirement: RFC7770-2.4-2 positive -- the area-scope RI LSA flooded into the area
	// whose interface runs TE sets the TE Informational Capability: the capability the router
	// holds in that scope is reflected (§2.4).
	if !claims[backbone] {
		t.Fatal("the backbone RI LSA does not claim TE although eth0 in the backbone runs TE")
	}
	// RFC requirement: RFC7770-2.4-2 negative -- the area-scope RI LSA flooded into an area with
	// no TE interface leaves the TE Informational Capability clear, although the router runs TE
	// in another area: the word is not the router-wide one (§2.4).
	if claims[other] {
		t.Fatal("the area 0.0.0.5 RI LSA claims TE although no interface in area 0.0.0.5 runs TE")
	}
}
