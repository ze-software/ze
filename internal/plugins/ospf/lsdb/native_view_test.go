// VALIDATES: the native-LSA view builders (native_view.go) and the Entry identity
// accessor (entry.go). AllLSAViews surfaces every stored LSA across the per-area,
// AS-wide, and per-interface stores with a copy of its body; LSAViewsByType filters
// by 16-bit LS Type; Entry.Key returns the LSDB identity tuple of its header.
// PREVENTS: a view builder that drops a store (leaking a scope from a debug database
// dump) or mislabels an LSA's area/interface, and an Entry.Key that diverges from the
// header it was built from.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestEntryKeyRoundTrips(t *testing.T) {
	h := packet.LSAHeader{
		Type:              types.LSTypeRouter,
		LinkStateID:       lsid("7.7.7.7"),
		AdvertisingRouter: rid("8.8.8.8"),
		Sequence:          types.InitialSequenceNumber,
	}
	e := newEntry(h, nil, time.Unix(0, 0), false)
	want := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: lsid("7.7.7.7"), AdvertisingRouter: rid("8.8.8.8")}
	if got := e.Key(); got != want {
		t.Fatalf("Entry.Key() = %+v, want %+v", got, want)
	}
	if e.Key() != h.Key() {
		t.Fatalf("Entry.Key() = %+v diverged from header.Key() = %+v", e.Key(), h.Key())
	}
}

func TestAllLSAViewsSurfacesEveryStore(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")

	if !db.Install(a0, routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10)) {
		t.Fatalf("install router LSA rejected")
	}
	if !db.Install(a0, externalLSA(t, rid("3.3.3.3"), types.InitialSequenceNumber)) {
		t.Fatalf("install external LSA rejected")
	}
	// A link-scope OSPFv3 Type-9 opaque LSA lands in the per-interface link store.
	lsa9 := opaqueLSA(t, types.LSTypeOpaqueLink, 1, 0x30, rid("4.4.4.4"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})
	if _, ok := db.installLink("eth0", a0, lsa9, false, true); !ok {
		t.Fatalf("installLink rejected Type 9 opaque LSA")
	}

	views := db.AllLSAViews()
	if len(views) != 3 {
		t.Fatalf("AllLSAViews len = %d, want 3: %+v", len(views), views)
	}
	byType := map[types.LSType]NativeLSAView{}
	for _, v := range views {
		byType[v.Type] = v
	}

	rv, ok := byType[types.LSTypeRouter]
	if !ok {
		t.Fatalf("router LSA missing from AllLSAViews: %+v", views)
	}
	if rv.Area != a0 || rv.Interface != "" || rv.AdvertisingRouter != rid("2.2.2.2") {
		t.Fatalf("router view mislabelled: %+v", rv)
	}
	if len(rv.Body) == 0 || len(rv.RawBytes) < types.LSAHeaderLen {
		t.Fatalf("router view body/raw not populated: body=%d raw=%d", len(rv.Body), len(rv.RawBytes))
	}

	ev, ok := byType[types.LSTypeASExternal]
	if !ok {
		t.Fatalf("external LSA missing from AllLSAViews: %+v", views)
	}
	// AS-External is collected under the backbone area with no interface.
	if ev.Area != types.BackboneArea || ev.Interface != "" {
		t.Fatalf("external view mislabelled: %+v", ev)
	}

	lv, ok := byType[types.LSTypeOpaqueLink]
	if !ok {
		t.Fatalf("link LSA missing from AllLSAViews: %+v", views)
	}
	if lv.Interface != "eth0" || lv.Area != a0 {
		t.Fatalf("link view mislabelled: %+v", lv)
	}
	if len(lv.Body) != 4 {
		t.Fatalf("link view body copy = % x, want the 4 originated bytes", lv.Body)
	}
}

func TestLSAViewsByTypeFilters(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	db.Install(a0, routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10))
	db.Install(a0, externalLSA(t, rid("3.3.3.3"), types.InitialSequenceNumber))

	routers := db.LSAViewsByType(types.LSTypeRouter)
	if len(routers) != 1 || routers[0].AdvertisingRouter != rid("2.2.2.2") || routers[0].Area != a0 {
		t.Fatalf("LSAViewsByType(router) = %+v", routers)
	}
	externals := db.LSAViewsByType(types.LSTypeASExternal)
	if len(externals) != 1 || externals[0].Area != types.BackboneArea {
		t.Fatalf("LSAViewsByType(external) = %+v", externals)
	}
	// A type with no stored LSA yields an empty result, not the other types.
	if got := db.LSAViewsByType(types.LSTypeNetwork); len(got) != 0 {
		t.Fatalf("LSAViewsByType(network) = %+v, want empty", got)
	}
}
