// Design: docs/architecture/rib/unified-locrib.md -- complete selected target sets.
// VALIDATES: both readers retain accepted paths with an ECMP-only target.

package sysrib

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

func TestRouteViewECMPOnlySourceBothReaders(t *testing.T) {
	for _, projected := range []bool{false, true} {
		t.Run(map[bool]string{false: "local-authority", true: "selected-json"}[projected], func(t *testing.T) {
			rib := locrib.NewRIB()
			prefix := netip.MustParsePrefix("198.18.229.0/24")
			primary := crossSourceCandidate("bgp", "primary", 101)
			empty := crossSourceCandidate("isis", "", 0)
			// applyRouteInstall accepts a RouteInstallEntry with no primary
			// gateway/device and a device-only ECMP member. This is its complete
			// locrib.Path representation, including source, rank and metadata.
			member := crossSourceCandidate("static", "", 701)
			member.SRv6SID = netip.MustParseAddr("2001:db8:229::1")
			member.ECMP = []locrib.NextHop{{Interface: "member", Weight: 5, OnLink: true}}
			rib.Insert(family.IPv4Unicast, prefix, primary)
			rib.Insert(family.IPv4Unicast, prefix, empty)
			rib.Insert(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:229::/64"), crossSourceCandidate("connected", "sid-link", 0))
			state := newSysRIB()
			state.resolver.Store(newNHResolver(rib))
			state.applyFIBImport(map[string]bool{"bgp": true, "static": true, "isis": true})
			apply := func() []outgoingChange {
				t.Helper()
				batches, err := state.applySelectedBatch([]routeViewSelectedChange{explicitSelectedFromAuthority(t, rib, prefix, projected)})
				if err != nil {
					t.Fatal(err)
				}
				return batches[family.IPv4Unicast]
			}
			if initial := apply(); len(initial) != 1 || initial[0].Interface != primary.Interface || len(initial[0].ECMPPaths) != 0 {
				t.Fatalf("empty source disturbed the useful primary: %+v", initial)
			}
			rib.Insert(family.IPv4Unicast, prefix, member)
			changes := apply()
			if len(changes) != 1 || changes[0].Interface != primary.Interface || len(changes[0].ECMPPaths) != 1 {
				t.Fatalf("accepted ECMP-only target never reached this reader: %+v", changes)
			}
			target := changes[0].ECMPPaths[0]
			if target.Interface != "member" || target.Weight != 5 || !target.OnLink || !slices.Equal(target.Labels, member.Labels) {
				t.Fatalf("ECMP-only target metadata changed: %+v", target)
			}
			before := rib.Revision()
			member.ECMP = []locrib.NextHop{{Interface: "replacement", Weight: 7, OnLink: true}}
			rib.Insert(family.IPv4Unicast, prefix, member)
			if rib.Revision() != before+1 {
				t.Fatal("ECMP-only secondary update did not dirty the reader")
			}
			changes = apply()
			if len(changes) != 1 || len(changes[0].ECMPPaths) != 1 || changes[0].ECMPPaths[0].Interface != "replacement" || changes[0].ECMPPaths[0].Weight != 7 {
				t.Fatalf("ECMP-only secondary update did not reach current forwarding: %+v", changes)
			}
		})
	}
}
