// Design: docs/architecture/rib/unified-locrib.md -- owned selected metadata.

package sysrib

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

func TestRouteViewLocalSelectedMetadataOwnsMemberLabels(t *testing.T) {
	source := redistevents.RegisterProtocol("static")
	prefix := netip.MustParsePrefix("198.18.235.0/24")
	group := locrib.PathGroup{Best: 0, Paths: []locrib.Path{
		{Source: source, Instance: 1, Interface: "primary", Labels: []uint32{101}},
		{Source: source, Instance: 2, Interface: "member", Labels: []uint32{202}, Weight: 3},
	}}
	prepared, err := prepareRouteViewLocal(family.IPv4Unicast, prefix, &group)
	if err != nil {
		t.Fatal(err)
	}
	group.Paths[0].Labels[0] = 303
	group.Paths[1].Labels[0] = 404
	state := newSysRIB()
	state.resolver.Store(nil)
	batches, err := state.applySelectedBatch([]routeViewSelectedChange{prepared})
	if err != nil {
		t.Fatal(err)
	}
	changes := batches[family.IPv4Unicast]
	if len(changes) != 1 {
		t.Fatalf("healthy local selection emitted %d entries", len(changes))
	}
	entry := changes[0]
	if entry.Interface != "primary" || !slices.Equal(entry.Labels, []uint32{101}) {
		t.Fatalf("local primary retained authority storage: %+v", entry)
	}
	if len(entry.ECMPPaths) != 1 || entry.ECMPPaths[0].Interface != "member" || entry.ECMPPaths[0].Weight != 3 || !slices.Equal(entry.ECMPPaths[0].Labels, []uint32{202}) {
		t.Fatalf("local member retained or replaced authority metadata: %+v", entry.ECMPPaths)
	}
}
