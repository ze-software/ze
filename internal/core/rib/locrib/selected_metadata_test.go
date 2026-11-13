// VALIDATES: selected sibling metadata changes invalidate the authoritative view.
// PREVENTS: a stable winner hiding a member's label or SID change from consumers.

package locrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

func TestLocRIBSelectedMemberMetadataDelta(t *testing.T) {
	for _, field := range []string{"labels", "sid"} {
		t.Run(field, func(t *testing.T) {
			rib := NewRIB()
			prefix := netip.MustParsePrefix("198.18.231.0/24")
			primary := ospfPath("192.0.2.1", 1)
			member := ospfPath("192.0.2.2", 2)
			member.Labels = []uint32{201}
			member.SRv6SID = netip.MustParseAddr("2001:db8:231::1")
			var changes []Change
			unsubscribe := rib.OnChange(func(change Change) { changes = append(changes, change) })
			defer unsubscribe()
			rib.Insert(family.IPv4Unicast, prefix, primary)
			rib.Insert(family.IPv4Unicast, prefix, member)
			if len(changes) != 2 || len(changes[1].ECMP) != 1 {
				t.Fatalf("healthy selected sibling control: %+v", changes)
			}
			changes = nil
			before := rib.Revision()
			rib.Insert(family.IPv4Unicast, prefix, member)
			if len(changes) != 0 || rib.Revision() != before {
				t.Fatal("identical member generated a selected change")
			}
			if field == "labels" {
				member.Labels = []uint32{202, 203}
			} else {
				member.SRv6SID = netip.MustParseAddr("2001:db8:231::2")
			}
			best, changed := rib.Insert(family.IPv4Unicast, prefix, member)
			if changed || best.Instance != primary.Instance {
				t.Fatal("member metadata changed the selected winner")
			}
			if len(changes) != 1 || changes[0].Kind != ChangeUpdate || rib.Revision() != before+1 {
				t.Fatalf("selected member %s edit lost notification: changes=%+v revision=%d want=%d", field, changes, rib.Revision(), before+1)
			}
		})
	}
}

func TestLocRIBUnselectedMemberMetadataDoesNotNotify(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run(map[bool]string{false: "higher-metric", true: "hidden"}[hidden], func(t *testing.T) {
			rib := NewRIB()
			prefix := netip.MustParsePrefix("198.18.232.0/24")
			primary := ospfPath("192.0.2.1", 1)
			member := ospfPath("192.0.2.2", 2)
			if hidden {
				usePolicy(t, rib, map[redistevents.ProtocolID]ribpolicy.Chain{primary.Source: chainFunc(func(route ribpolicy.Route) ribpolicy.Verdict {
					if route.NextHop == member.NextHop {
						return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
					}
					return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
				})}, nil)
			} else {
				member.Metric++
			}
			rib.Insert(family.IPv4Unicast, prefix, primary)
			rib.Insert(family.IPv4Unicast, prefix, member)
			var changes int
			unsubscribe := rib.OnChange(func(Change) { changes++ })
			defer unsubscribe()
			before := rib.Revision()
			member.Labels = []uint32{301}
			member.SRv6SID = netip.MustParseAddr("2001:db8:232::1")
			rib.Insert(family.IPv4Unicast, prefix, member)
			if changes != 0 || rib.Revision() != before {
				t.Fatal("unselected metadata dirtied the selected view")
			}
			if best, ok := rib.Best(family.IPv4Unicast, prefix); !ok || best.Instance != primary.Instance {
				t.Fatalf("healthy winner was lost: %+v %v", best, ok)
			}
		})
	}
}
