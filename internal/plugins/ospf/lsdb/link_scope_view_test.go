// VALIDATES: the per-interface link-store helpers in link_scope.go: LinkLSAs returns
// every link-local-scope LSA stored for an interface in stable key order; and the
// self-Link-LSA stale sweep FlushStaleLinkSelfLSAs -> deleteLinkLSA fully removes a
// self Link-LSA absent from the regenerated keep set.
// PREVENTS: a link view that omits stored LSAs or returns them unordered, and a stale
// sweep that leaves a withdrawn self Link-LSA in the per-interface store.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestLinkLSAsReturnsSortedForInterface(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	lsaA := opaqueLSA(t, types.LSTypeOpaqueLink, 1, 0x10, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})
	lsaB := opaqueLSA(t, types.LSTypeOpaqueLink, 1, 0x20, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{5, 6, 7, 8})
	if _, ok := db.installLink("eth0", a0, lsaA, false, true); !ok {
		t.Fatalf("installLink A rejected")
	}
	if _, ok := db.installLink("eth0", a0, lsaB, false, true); !ok {
		t.Fatalf("installLink B rejected")
	}

	got := db.LinkLSAs("eth0")
	if len(got) != 2 {
		t.Fatalf("LinkLSAs(eth0) len = %d, want 2", len(got))
	}
	if !got[0].Header.Key().Less(got[1].Header.Key()) {
		t.Fatalf("LinkLSAs not in ascending key order: %v then %v", got[0].Header.Key(), got[1].Header.Key())
	}
	keys := map[types.LSAKey]bool{got[0].Header.Key(): true, got[1].Header.Key(): true}
	if !keys[lsaA.Header.Key()] || !keys[lsaB.Header.Key()] {
		t.Fatalf("LinkLSAs missing an installed key: %+v", keys)
	}
	// An interface with no link store returns nil.
	if got := db.LinkLSAs("eth9"); got != nil {
		t.Fatalf("LinkLSAs(unknown) = %+v, want nil", got)
	}
}

func TestFlushStaleLinkSelfLSAsRemovesDropped(t *testing.T) {
	db, _, _ := opaqueOriginateDB(t)
	a0 := area("0.0.0.0")
	h, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 1, OpaqueID: 0x03, Scope: types.LSTypeOpaqueLink,
		Interface: "eth0", Area: a0, Options: types.OptionO, Body: []byte{1, 2, 3, 4},
	})
	if !ok {
		t.Fatalf("originate self link opaque failed")
	}
	if _, ok := db.LookupLinkLSA("eth0", h.Key()); !ok {
		t.Fatalf("self link opaque not installed")
	}

	// Regenerate with an empty keep set: the self Link-LSA is stale and removed outright.
	if n := db.FlushStaleLinkSelfLSAs(rid("1.1.1.1"), map[LinkLSARef]struct{}{}); n != 1 {
		t.Fatalf("FlushStaleLinkSelfLSAs removed %d, want 1", n)
	}
	if _, ok := db.LookupLinkLSA("eth0", h.Key()); ok {
		t.Fatalf("stale self Link-LSA still present after flush")
	}
	if got := db.LinkLSAs("eth0"); len(got) != 0 {
		t.Fatalf("eth0 link store not empty after flush: %+v", got)
	}
	// deleteLinkLSA on an unknown interface reports nothing removed.
	if db.deleteLinkLSA("eth9", h.Key()) {
		t.Fatalf("deleteLinkLSA(unknown) reported a removal")
	}
}
