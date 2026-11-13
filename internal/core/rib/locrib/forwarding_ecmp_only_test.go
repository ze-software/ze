// Design: docs/architecture/rib/unified-locrib.md -- complete selected target sets.
// VALIDATES: an accepted path can carry its usable target only in Path.ECMP.

package locrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

func TestLocRIBForwardingMembersECMPOnlySource(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.229.0/24")
	primary := forwardingCandidate("bgp", "primary", 1)
	empty := forwardingCandidate("isis", "", 1)
	member := explicitForwardingCandidate()
	member.Interface = ""
	rib.Insert(family.IPv4Unicast, prefix, primary)
	before := rib.Revision()
	rib.Insert(family.IPv4Unicast, prefix, empty)
	if rib.Revision() != before {
		t.Fatal("a genuinely empty source changed selected forwarding")
	}
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		if group.Paths[group.Best].Source != primary.Source || len(group.ForwardingMembers(group.Paths[group.Best])) != 0 {
			t.Fatal("empty source disturbed the useful primary control")
		}
	})
	rib.Insert(family.IPv4Unicast, prefix, member)
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		members := group.ForwardingMembers(group.Paths[group.Best])
		if len(members) != 1 || members[0].Source != member.Source || members[0].NextHop != member.ECMP[0] {
			t.Fatalf("accepted ECMP-only source lost its complete target: %+v", members)
		}
		if members[0].SRv6SID != member.SRv6SID {
			t.Fatal("ECMP-only source lost its Service SID")
		}
	})
}

func TestLocRIBECMPOnlySecondaryChangesNotify(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.230.0/24")
	primary := forwardingCandidate("bgp", "primary", 1)
	member := explicitForwardingCandidate()
	member.Interface = ""
	rib.Insert(family.IPv4Unicast, prefix, primary)
	rib.Insert(family.IPv4Unicast, prefix, member)
	notifications := 0
	unsubscribe := rib.OnChange(func(Change) { notifications++ })
	defer unsubscribe()
	before := rib.Revision()
	rib.Insert(family.IPv4Unicast, prefix, member)
	if notifications != 0 || rib.Revision() != before {
		t.Fatal("unchanged ECMP-only source emitted a selected event")
	}
	member.ECMP = []NextHop{{Interface: "replacement", Weight: 7, OnLink: true}}
	best, changed := rib.Insert(family.IPv4Unicast, prefix, member)
	if changed || best.Source != primary.Source {
		t.Fatal("ECMP-only secondary update changed the primary")
	}
	if notifications != 1 || rib.Revision() != before+1 {
		t.Fatalf("ECMP-only secondary update was invisible: notifications=%d revision=%d want=%d", notifications, rib.Revision(), before+1)
	}
}
