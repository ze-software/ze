// Design: docs/architecture/ospf/ospf-ext-3-router-information.md -- RI informational capabilities per flooding scope.
// Related: ri.go -- deriveRICapabilities and riOriginate, the capability source and the per-area emission.
//
// VALIDATES: RFC 7770 Section 2.4, "the TLV MUST accurately reflect the OSPF router's
// capabilities in the scope advertised": an area-scope RI LSA flooded into an area where no
// interface runs traffic engineering does not claim the TE informational capability; a
// link-scope RI LSA follows its own interface; the AS-scope RI LSA claims TE only when an
// interface runs TE, never for a configured TE router address alone.
// PREVENTS: one router-wide capability word copied into every area's RI LSA, and a TE claim
// from a router that originates no TE LSA anywhere.
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

// riTEClaims originates the RI LSAs of eng at one scope and returns, per target (the area for
// an area-scope LSA, the interface for a link-scope one, "as" for the AS-scope one), whether
// instance 0's Informational Capabilities word sets the TE bit.
func riTEClaims(t *testing.T, eng *engine, router types.RouterID, scope OpaqueScope) map[string]bool {
	t.Helper()
	te := packet.RIInfoBitMask(packet.RIInfoBitTrafficEngineering)
	claims := map[string]bool{}
	for _, o := range eng.riOriginate(router) {
		if o.Scope != scope || o.Withdraw || o.OpaqueID != 0 {
			continue
		}
		// Instance 0 opens with the type-1 TLV: Type (2), Length (2), then the 32-bit word.
		if len(o.Body) < 8 || binary.BigEndian.Uint16(o.Body[0:2]) != packet.RITLVInformationalCapabilities {
			t.Fatalf("RI instance 0 does not open with the Informational Capabilities TLV")
		}
		target := "as"
		switch scope {
		case OpaqueScopeLink:
			target = o.Interface
		case OpaqueScopeArea:
			target = o.Area.String()
		case OpaqueScopeAS:
		default:
			panic("BUG: riTEClaims: invalid scope")
		}
		claims[target] = binary.BigEndian.Uint32(o.Body[4:8])&te != 0
	}
	return claims
}

// TestRFC7770LinkRICapabilitiesFollowTheInterface configures TE on eth0 only, with eth1 in the
// same area, and reads the TE bit of each link-scope (Type-9) RI LSA.
func TestRFC7770LinkRICapabilitiesFollowTheInterface(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)
	router := types.RouterID{1, 1, 1, 1}
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,`+
		`"router-information":{"enabled":true,"scope":["link"]},`+
		`"areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"}}},`+
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0","network-type":"point-to-point","traffic-engineering":{"enable":true}},`+
		`"eth1":{"area":"0.0.0.0","network-type":"point-to-point"}}}}}`)
	claims := riTEClaims(t, eng, router, OpaqueScopeLink)
	if len(claims) != 2 {
		t.Fatalf("link-scope RI originated on %d interfaces (%v), want 2", len(claims), claims)
	}
	// RFC requirement: RFC7770-2.4-2 positive -- the link-scope RI LSA of the interface that
	// runs TE sets the TE Informational Capability (§2.4, the scope is that link).
	if !claims["eth0"] {
		t.Fatal("the eth0 link-scope RI LSA does not claim TE although eth0 runs TE")
	}
	// RFC requirement: RFC7770-2.4-2 negative -- the link-scope RI LSA of an interface without
	// TE leaves the TE bit clear, although another interface of the same area runs TE (§2.4).
	if claims["eth1"] {
		t.Fatal("the eth1 link-scope RI LSA claims TE although eth1 runs no TE")
	}
}

// TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns reads the TE bit of the AS-scope RI LSA for
// each source of the TE answer alone: a configured TE router address with no TE interface,
// then a TE interface with no router address.
func TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)
	router := types.RouterID{1, 1, 1, 1}
	addressOnly, _ := newRedistEngine(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,"router-address":"9.9.9.9",`+
		`"router-information":{"enabled":true,"scope":["as"]},`+
		`"areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"}}},`+
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0","network-type":"point-to-point"}}}}}`)
	// The premise: with no TE interface the router originates no RFC 3630 TE LSA at all.
	if te := addressOnly.teOriginateType1(router); len(te) != 0 {
		t.Fatalf("a router address alone originated %d TE LSAs, want none", len(te))
	}
	// RFC requirement: RFC7770-2.4-2 negative -- a configured TE router address with no TE
	// interface leaves the AS-scope TE bit clear: the router runs no RFC 3630 TE anywhere,
	// so claiming it would not reflect its capabilities in that scope (§2.4).
	if claims := riTEClaims(t, addressOnly, router, OpaqueScopeAS); len(claims) != 1 || claims["as"] {
		t.Fatalf("AS-scope RI TE claims = %v, want one AS LSA with TE clear", claims)
	}

	resetRITLVs()
	interfaceOnly, _ := newRedistEngine(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,`+
		`"router-information":{"enabled":true,"scope":["as"]},`+
		`"areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"}}},`+
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0","network-type":"point-to-point","traffic-engineering":{"enable":true}}}}}}`)
	// RFC requirement: RFC7770-2.4-2 positive -- a TE interface alone, with no configured
	// router address, sets the AS-scope TE bit (§2.4).
	if claims := riTEClaims(t, interfaceOnly, router, OpaqueScopeAS); len(claims) != 1 || !claims["as"] {
		t.Fatalf("AS-scope RI TE claims = %v, want one AS LSA with TE set", claims)
	}
}
