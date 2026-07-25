// VALIDATES: the OSPFv3 Grace-LSA (RFC 5187 sec 2.1, internal AF-neutral sentinel
// LSTypeGraceV6, function code 11) routes through the link-local store predicate, while the
// OSPFv2 Opaque-AS type (numerically 0x000B == 11) does NOT (A-4, AC-2), so the two never
// collide despite sharing the wire value 0x000B across address families.
// PREVENTS: a link-scoped Grace-LSA being misrouted into the AS-wide opaque store, or a
// Type-11 Opaque-AS LSA being misrouted into the per-interface link store.
package lsdb

import (
	"testing"

	types "github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestGraceLSALinkScopeRouting(t *testing.T) {
	if !isLinkLSAType(types.LSTypeGraceV6) {
		t.Fatalf("LSTypeGraceV6 must be link-scoped so the OSPFv3 Grace-LSA routes through the link store")
	}
	if isLinkLSAType(types.LSTypeOpaqueAS) {
		t.Fatalf("Opaque-AS (Type 11) must NOT be link-scoped -- it is AS-wide")
	}
	// The two link-scoped OSPFv3 types (Link-LSA + Grace-LSA) and the Type-9 opaque LSA are
	// the only link-scoped types; area/AS types are not.
	if !isLinkLSAType(types.LSTypeLink) || !isLinkLSAType(types.LSTypeOpaqueLink) {
		t.Fatalf("Link-LSA and Type-9 opaque must remain link-scoped")
	}
	if isLinkLSAType(types.LSTypeASExternal) || isLinkLSAType(types.LSTypeOpaqueArea) {
		t.Fatalf("area/AS types must not be link-scoped")
	}
}
