// Design: docs/architecture/rib/unified-locrib.md -- complete selected member ownership.
// VALIDATES: both readers preserve a nonwinner's configured explicit group.

package sysrib

import (
	"encoding/json"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// explicitSelectedFromAuthority crosses the actual JSON selected-page boundary
// for the external reader; the local reader copies under the authority lock.
func explicitSelectedFromAuthority(t *testing.T, rib *locrib.RIB, prefix netip.Prefix, projected bool) routeViewSelectedChange {
	t.Helper()
	if !projected {
		return selectedFromAuthority(t, rib, prefix, false)
	}
	var entry rpc.RouteViewEntry
	var copyErr error
	rib.Inspect(family.IPv4Unicast, prefix, func(group locrib.PathGroup) {
		copyErr = routeViewCopyEntry(family.IPv4Unicast, prefix, &group, &entry)
	})
	if copyErr != nil {
		t.Fatal(copyErr)
	}
	page := rpc.RouteViewReadOutput{Session: "explicit-members", Epoch: "daemon", Snapshot: "initial", Cursor: 2,
		Phase: rpc.RouteViewPhaseSnapshot, Complete: true, Entries: []rpc.RouteViewEntry{entry}}
	wire, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var received rpc.RouteViewReadOutput
	if err := json.Unmarshal(wire, &received); err != nil {
		t.Fatal(err)
	}
	projection, err := newSelectedProjection(rpc.RouteViewOpenOutput{Session: page.Session, Epoch: page.Epoch, Snapshot: page.Snapshot})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projection.Apply(&received); err != nil {
		t.Fatal(err)
	}
	var prepared routeViewSelectedChange
	projection.Inspect(family.IPv4Unicast, prefix, func(value *selectedProjectionPath) {
		prepared, copyErr = prepareRouteViewSelectedMetadata(locrib.Change{Kind: locrib.ChangeAdd,
			Family: family.IPv4Unicast, Prefix: prefix, Best: value.path, ECMP: value.path.ECMP}, value.members)
	})
	if copyErr != nil {
		t.Fatal(copyErr)
	}
	if prepared == nil {
		t.Fatal("authority JSON page produced no selected route")
	}
	return prepared
}

func TestRouteViewNonwinnerExplicitGroupBothReaders(t *testing.T) {
	for _, projected := range []bool{false, true} {
		t.Run(map[bool]string{false: "local-authority", true: "selected-json"}[projected], func(t *testing.T) {
			rib := locrib.NewRIB()
			prefix := netip.MustParsePrefix("198.18.252.0/24")
			primary := crossSourceCandidate("bgp", "bgp-primary", 101)
			static := crossSourceCandidate("static", "static-primary", 701)
			static.Weight = 2
			static.SRv6SID = netip.MustParseAddr("2001:db8:252::1")
			static.ECMP = []locrib.NextHop{{Interface: "static-secondary", Weight: 5, OnLink: true}}
			rib.Insert(family.IPv4Unicast, prefix, primary)
			rib.Insert(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:252::/64"), crossSourceCandidate("connected", "sid-link", 0))
			state := newSysRIB()
			state.resolver.Store(newNHResolver(rib))
			state.applyFIBImport(map[string]bool{"bgp": true, "static": true})
			apply := func() []outgoingChange {
				t.Helper()
				batches, err := state.applySelectedBatch([]routeViewSelectedChange{explicitSelectedFromAuthority(t, rib, prefix, projected)})
				if err != nil {
					t.Fatal(err)
				}
				return batches[family.IPv4Unicast]
			}
			if initial := apply(); len(initial) != 1 || initial[0].Action != routeaction.Add || initial[0].Interface != primary.Interface || len(initial[0].ECMPPaths) != 0 {
				t.Fatalf("useful selected primary control: %+v", initial)
			}
			rib.Insert(family.IPv4Unicast, prefix, static)
			assertGroup := func(changes []outgoingChange, device string, weight uint8) {
				t.Helper()
				if len(changes) != 1 || changes[0].Protocol != "bgp" || changes[0].Interface != primary.Interface || len(changes[0].ECMPPaths) != 2 {
					t.Fatalf("nonwinner explicit group missing: %+v", changes)
				}
				for _, member := range changes[0].ECMPPaths {
					if member.Interface != device {
						continue
					}
					if member.Weight != weight || !member.OnLink || !slices.Equal(member.Labels, static.Labels) {
						t.Fatalf("explicit secondary lost forwarding metadata: %+v", member)
					}
					return
				}
				t.Fatalf("secondary %s missing: %+v", device, changes)
			}
			assertGroup(apply(), "static-secondary", 5)
			prepared := explicitSelectedFromAuthority(t, rib, prefix, projected).(routeViewSelectedAdd)
			if len(prepared.route.ecmpMetadata) != 2 {
				t.Fatalf("reader lost member ownership: %+v", prepared.route.ecmpMetadata)
			}
			for _, member := range prepared.route.ecmpMetadata {
				if member.source != static.Source || member.sid != static.SRv6SID || !slices.Equal(member.labels, static.Labels) {
					t.Fatalf("reader lost source, labels or SID: %+v", member)
				}
			}
			before := rib.Revision()
			notifications := 0
			unsubscribe := rib.OnChange(func(locrib.Change) { notifications++ })
			defer unsubscribe()
			rib.Insert(family.IPv4Unicast, prefix, static)
			if notifications != 0 || rib.Revision() != before {
				t.Fatal("unchanged explicit group notified the reader")
			}
			static.ECMP = []locrib.NextHop{{Interface: "static-replacement", Weight: 7, OnLink: true}}
			rib.Insert(family.IPv4Unicast, prefix, static)
			if notifications != 1 || rib.Revision() != before+1 {
				t.Fatalf("secondary-only change did not wake current reader: notifications=%d revision=%d want=%d", notifications, rib.Revision(), before+1)
			}
			assertGroup(apply(), "static-replacement", 7)
			withheld := state.applyFIBImport(map[string]bool{"bgp": true, "static": false})[family.IPv4Unicast]
			if len(withheld) != 1 || withheld[0].Interface != primary.Interface || len(withheld[0].ECMPPaths) != 0 {
				t.Fatalf("static permission did not remove its whole explicit group: %+v", withheld)
			}
			assertGroup(state.applyFIBImport(map[string]bool{"bgp": true, "static": true})[family.IPv4Unicast], "static-replacement", 7)
			replayed := state.replayBatches(&replay.Request{ReplayID: replay.Broadcast})
			if len(replayed) != 1 || len(replayed[0].Changes) != 1 {
				t.Fatalf("explicit group replay: %+v", replayed)
			}
			assertGroup(replayed[0].Changes, "static-replacement", 7)
		})
	}
}
