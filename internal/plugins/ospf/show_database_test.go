// VALIDATES: spec-ospf-13 AC-4 + spec-ospf-ext-1 -- the database subview filter keeps
// only LSAs of the requested LS Type, and the subview map covers the six base types
// (router/network/summary/asbr-summary/external/nssa-external) plus the three RFC 5250
// opaque subviews (opaque-link/opaque-area/opaque-as).
// PREVENTS: a subview that leaks other LS types or a missing subview mapping.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestDatabaseSubviewFilter(t *testing.T) {
	lsas := []ospflsdb.LSASnapshot{{Type: "router"}, {Type: "network"}, {Type: "router"}, {Type: "as-external"}}
	got := filterLSAsByType(lsas, "router")
	assert.Len(t, got, 2, "only the two router LSAs are kept")
	for i := range got {
		assert.Equal(t, "router", got[i].Type)
	}
	assert.Empty(t, filterLSAsByType(lsas, "nssa"), "no nssa LSAs -> empty")
}

func TestDatabaseSubviewMapCovers6Types(t *testing.T) {
	want := map[string]string{
		"show ospf database router":        "router",
		"show ospf database network":       "network",
		"show ospf database summary":       "summary-network",
		"show ospf database asbr-summary":  "summary-asbr",
		"show ospf database external":      "as-external",
		"show ospf database nssa-external": "nssa",
		"show ospf database opaque-link":   "opaque-link", // Type 9 (RFC 5250)
		"show ospf database opaque-area":   "opaque-area", // Type 10 (RFC 5250)
		"show ospf database opaque-as":     "opaque-as",   // Type 11 (RFC 5250)
	}
	assert.Equal(t, want, dbSubviewType)
}

func TestDatabaseSnapshotByTypeNilLSDB(t *testing.T) {
	e := &engine{}
	assert.Nil(t, e.databaseSnapshotByType("router"))
}

func TestDatabaseSnapshotIncludesLinkLSAs(t *testing.T) {
	e := newV6OriginEngine()
	self := types.RouterID{172, 30, 0, 2}
	if _, ok := e.v6OriginateLinkLSA(self, v6BroadcastInterface(types.BackboneArea, self)); !ok {
		t.Fatal("v6OriginateLinkLSA did not originate")
	}
	rows := e.databaseSnapshot()
	if len(rows) != 1 {
		t.Fatalf("database snapshot rows = %d, want 1", len(rows))
	}
	snap, ok := rows[0].(ospflsdb.Snapshot)
	if !ok {
		t.Fatalf("database snapshot type = %T, want lsdb.Snapshot", rows[0])
	}
	if len(snap.Links) != 1 || snap.Links[0].Interface != "eth0" || len(snap.Links[0].LSAs) != 1 {
		t.Fatalf("link database snapshot = %+v", snap.Links)
	}
	row := snap.Links[0].LSAs[0]
	if row.Type != "link" || row.LinkLocalAddress != "fe80::1" || row.Interface != "eth0" {
		t.Fatalf("link database row = %+v", row)
	}
}
