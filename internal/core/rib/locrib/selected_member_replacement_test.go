// VALIDATES: removal and policy reselection retain selected member metadata.
// PREVENTS: an unchanged target hiding the replacement member's forwarding state.

package locrib

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

func TestLocRIBSelectedMemberReplacementMetadata(t *testing.T) {
	for _, operation := range []string{"remove", "reselect"} {
		t.Run(operation, func(t *testing.T) {
			rib := NewRIB()
			prefix := netip.MustParsePrefix("198.18.236.0/24")
			primary := ospfPath("192.0.2.1", 1)
			first := ospfPath("192.0.2.2", 2)
			first.Labels = []uint32{201}
			first.SRv6SID = netip.MustParseAddr("2001:db8:236::1")
			first.Tag, first.HasTag = 2, true
			second := ospfPath("192.0.2.2", 3)
			second.Labels = []uint32{301}
			second.SRv6SID = netip.MustParseAddr("2001:db8:236::2")
			rib.Insert(family.IPv4Unicast, prefix, primary)
			rib.Insert(family.IPv4Unicast, prefix, first)
			rib.Insert(family.IPv4Unicast, prefix, second)
			assertMember := func(want Path) {
				t.Helper()
				rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
					members := group.ECMPMembers(group.Paths[group.Best])
					if len(members) != 1 || members[0].Addr != want.NextHop || !slices.Equal(members[0].Labels, want.Labels) || members[0].SRv6SID != want.SRv6SID {
						t.Fatalf("selected member metadata: %+v want path %+v", members, want)
					}
				})
			}
			assertMember(first)
			var changes []Change
			unsubscribe := rib.OnChange(func(change Change) { changes = append(changes, change) })
			defer unsubscribe()
			before := rib.Revision()
			if operation == "remove" {
				best, changed := rib.Remove(family.IPv4Unicast, prefix, first.Source, first.Instance)
				if changed || best.Instance != primary.Instance {
					t.Fatal("member removal changed the winner")
				}
			} else {
				usePolicy(t, rib, map[redistevents.ProtocolID]ribpolicy.Chain{first.Source: chainFunc(func(route ribpolicy.Route) ribpolicy.Verdict {
					if route.HasTag && route.Tag == 2 {
						return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
					}
					return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
				})}, nil)
			}
			if len(changes) != 1 || changes[0].Kind != ChangeUpdate || changes[0].Best.Instance != primary.Instance || rib.Revision() != before+1 {
				t.Fatalf("%s hid replacement metadata: changes=%+v revision=%d want=%d", operation, changes, rib.Revision(), before+1)
			}
			assertMember(second)
		})
	}
}
