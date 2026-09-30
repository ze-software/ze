// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- RFC 5250 Type-11 originator advertises itself an ASBR.
// Related: origination.go -- selfIsASBRLocked, the Router-LSA E-bit test.
// Related: opaque_as.go -- OriginateOpaque, the Type-11 LSA header Options.
//
// VALIDATES: RFC 5250 Section 5 (1): "An OSPF router that is configured to originate AS-scope
// opaque LSAs will advertise itself as an ASBR and MUST follow the requirements related to
// setting of the Options field E-bit in OSPF LSA headers as specified in [OSPF]." RFC 2328
// Section 12.1.2 sets the E-bit "in all AS-external-LSAs", the other AS-scope LSA.
// PREVENTS: a Type-11 originator whose Router-LSA carries no E-bit. RFC 5250 Section 5 (2)
// receivers, Ze among them, find no ASBR routing table entry for it and ignore every Type-11
// LSA it floods.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestRFC5250Type11OriginatorAdvertisesASBR drives one router through four states: nothing
// opaque originated, a Type-10 opaque LSA only, a Type-11 opaque LSA as well, and the Type-11
// withdrawn. After each it re-originates the Router-LSA past MinLSInterval and reads its E-bit.
func TestRFC5250Type11OriginatorAdvertisesASBR(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(originTopology)
	router := rid("1.1.1.1")
	body := []byte{0, 1, 0, 4, 0, 0, 0, 0}
	originate := func(scope types.LSType, withdraw bool) packet.LSAHeader {
		t.Helper()
		h, ok := db.OriginateOpaque(OpaqueOriginateInput{Router: router, OpaqueType: 4, Scope: scope,
			Area: area("0.0.0.0"), Options: types.OptionO, Body: body, Withdraw: withdraw})
		if !ok {
			t.Fatalf("originate opaque %s (withdraw %v) refused", scope, withdraw)
		}
		return h
	}
	reoriginate := func() {
		clock.now = clock.now.Add(6 * time.Second) // Past MinLSInterval (5s).
		db.OriginateFromTopology(router, false)
	}

	// RFC requirement: RFC5250-5-3 negative -- a router that originates no AS-scope opaque LSA
	// does not advertise itself an ASBR: its Router-LSA E-bit stays clear with nothing
	// originated, with a Type-10 (area-scope) opaque LSA originated, and again once its only
	// Type-11 LSA is withdrawn.
	db.OriginateFromTopology(router, false)
	if routerLSAEBit(t, db, router) {
		t.Fatal("Router-LSA E-bit set with no external or opaque LSA originated")
	}
	originate(types.LSTypeOpaqueArea, false)
	reoriginate()
	if routerLSAEBit(t, db, router) {
		t.Fatal("Router-LSA E-bit set by a Type-10 opaque LSA, which is not AS-scope")
	}

	// RFC requirement: RFC5250-5-3 positive -- once the router originates a Type-11 opaque LSA
	// it advertises itself an ASBR (Router-LSA E-bit set), and the Type-11 LSA header carries
	// the Options E-bit that RFC 2328 Section 12.1.2 sets in every AS-scope LSA.
	h := originate(types.LSTypeOpaqueAS, false)
	if !h.Options.Has(types.OptionE) {
		t.Fatalf("Type-11 LSA header Options %#02x lack the E-bit", uint8(h.Options))
	}
	reoriginate()
	if !routerLSAEBit(t, db, router) {
		t.Fatal("Router-LSA E-bit clear while the router originates a Type-11 opaque LSA")
	}

	clock.now = clock.now.Add(6 * time.Second)
	originate(types.LSTypeOpaqueAS, true)
	reoriginate()
	if routerLSAEBit(t, db, router) {
		t.Fatal("Router-LSA E-bit still set after the only Type-11 opaque LSA was withdrawn")
	}
}
