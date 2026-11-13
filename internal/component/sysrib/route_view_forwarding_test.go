// Design: docs/architecture/rib/unified-locrib.md -- authoritative selected-route consumption.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// The selected reader replaces the prior source outright. Consumer-local
// preferences must neither keep the old source nor resurrect it on withdrawal.
func TestRouteViewRuntimeSelectedReplacementHasNoStandby(t *testing.T) {
	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })
	s := newSysRIB()
	s.resolver.Store(newNHResolver(loc))
	s.adminDist = map[string]int{"static": 200, "bgp": 0, "ebgp": 0}
	s.fibPermit = map[string]bool{"static": true, "bgp": true}
	prefix := netip.MustParsePrefix("198.18.211.0/24")
	key := prefixKey{family: family.IPv4Unicast, prefix: prefix}
	static := locrib.Path{Source: redistevents.RegisterProtocol("static"), AdminDistance: 7, RouteType: routetype.Blackhole}
	bgp := locrib.Path{Source: redistevents.RegisterProtocol("bgp"), AdminDistance: 20, RouteType: routetype.Blackhole, IsBGP: true, IsEBGP: true}
	for index, path := range []locrib.Path{static, bgp} {
		changes, err := s.applySelected(locrib.Change{Family: key.family, Prefix: prefix, Kind: locrib.ChangeAdd, Best: path})
		if err != nil {
			t.Fatal(err)
		}
		wantAction := routeaction.Add
		if index != 0 {
			wantAction = routeaction.Update
		}
		if len(changes) != 1 || changes[0].Action != wantAction || changes[0].Protocol != redistevents.ProtocolName(path.Source) {
			t.Fatalf("selected source %d did not control forwarding: %+v", path.Source, changes)
		}
		if len(s.routes[key]) != 1 || s.best[key].priority != int(path.AdminDistance) {
			t.Fatalf("consumer selected or retained a different route: %+v", s.routes[key])
		}
	}
	changes, err := s.applySelected(locrib.Change{Family: key.family, Prefix: prefix, Kind: locrib.ChangeRemove})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != routeaction.Withdraw || len(s.routes) != 0 || len(s.best) != 0 || len(s.installed) != 0 {
		t.Fatalf("withdrawal retained or resurrected a consumer candidate: changes=%+v best=%+v", changes, s.best)
	}
}
