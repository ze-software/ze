// Design: docs/architecture/rib/unified-locrib.md -- authoritative selected views.
// Related: route_view_feed.go -- takeDirty and synchronize, the journal order.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: a delta page carries its records in the order the authority changed,
// never in a map's order. Method: after an empty snapshot, insert 32 prefixes
// in descending order (neither sorted nor any map's order), read one delta
// page, and require exactly the insertion order. One hundred rounds on the
// same session leave a map-ordered journal no chance to pass by luck.
func TestRouteViewDeltaKeepsCaptureOrder(t *testing.T) {
	rib := locrib.NewRIB()
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, owner, opened.Session, opened.Cursor, 4096)
	if page.Phase != rpc.RouteViewPhaseSnapshot || !page.Complete {
		t.Fatalf("empty snapshot did not complete: %+v", page)
	}
	path := locrib.Path{Source: redistevents.RegisterProtocol("static"), NextHop: netip.MustParseAddr("192.0.2.1"), Metric: 1}
	cursor := page.Cursor
	for round := range 100 {
		inserted := make([]netip.Prefix, 0, 32)
		for host := 31; host >= 0; host-- {
			prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(round), byte(host), 0}), 24)
			rib.Insert(family.IPv4Unicast, prefix, path)
			inserted = append(inserted, prefix)
		}
		delta := routeViewReadTest(t, &authority, owner, opened.Session, cursor, 4096)
		if delta.Phase != rpc.RouteViewPhaseDelta {
			t.Fatalf("round %d: read %v, want a delta page", round, delta.Phase)
		}
		if len(delta.Entries) != len(inserted) {
			t.Fatalf("round %d: delta carries %d records, want %d", round, len(delta.Entries), len(inserted))
		}
		for i := range inserted {
			if delta.Entries[i].Prefix != inserted[i] {
				t.Fatalf("round %d: record %d is %s, want %s: the journal order is not the capture order", round, i, delta.Entries[i].Prefix, inserted[i])
			}
		}
		cursor = delta.Cursor
	}
}
