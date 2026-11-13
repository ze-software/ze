// Design: docs/architecture/rib/unified-locrib.md -- complete selected member ownership.
// VALIDATES: explicit secondary targets survive when their producer is not best.

package locrib

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

func explicitForwardingCandidate() Path {
	path := forwardingCandidate("static", "static-primary", 1)
	path.Labels = []uint32{701, 702}
	path.SRv6SID = netip.MustParseAddr("2001:db8:252::1")
	path.Weight = 2
	path.ECMP = []NextHop{{Interface: "static-secondary", Weight: 5, OnLink: true}}
	return path
}

func TestLocRIBForwardingMembersNonwinnerExplicitGroup(t *testing.T) {
	for _, staticWins := range []bool{true, false} {
		t.Run(map[bool]string{true: "winner-control", false: "nonwinner"}[staticWins], func(t *testing.T) {
			rib := NewRIB()
			prefix := netip.MustParsePrefix("198.18.252.0/24")
			primary := forwardingCandidate("bgp", "bgp-primary", 1)
			static := explicitForwardingCandidate()
			if staticWins {
				rib.Insert(family.IPv4Unicast, prefix, static)
			}
			rib.Insert(family.IPv4Unicast, prefix, primary)
			rib.Insert(family.IPv4Unicast, prefix, static)
			rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
				best := group.Paths[group.Best]
				wantSource := primary.Source
				if staticWins {
					wantSource = static.Source
				}
				if best.Source != wantSource {
					t.Fatalf("primary source = %v, want %v", best.Source, wantSource)
				}
				members := group.ForwardingMembers(best)
				if len(members) != 2 {
					t.Fatalf("complete explicit forwarding group = %+v, want two secondary targets", members)
				}
				for _, member := range members {
					if member.Interface != "static-secondary" {
						continue
					}
					if member.Source != static.Source || member.Weight != 5 || !member.OnLink || !slices.Equal(member.Labels, static.Labels) || member.SRv6SID != static.SRv6SID {
						t.Fatalf("explicit secondary lost owner or metadata: %+v", member)
					}
					return
				}
				t.Fatal("static explicit secondary disappeared behind another primary")
			})
		})
	}
}

func TestLocRIBNonwinnerExplicitSecondaryChangesNotify(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.253.0/24")
	primary := forwardingCandidate("bgp", "bgp-primary", 1)
	static := explicitForwardingCandidate()
	rib.Insert(family.IPv4Unicast, prefix, primary)
	rib.Insert(family.IPv4Unicast, prefix, static)
	var changes []Change
	unsubscribe := rib.OnChange(func(change Change) { changes = append(changes, change) })
	defer unsubscribe()
	before := rib.Revision()
	rib.Insert(family.IPv4Unicast, prefix, static)
	if len(changes) != 0 || rib.Revision() != before {
		t.Fatal("unchanged explicit announcement changed the selected view")
	}
	static.ECMP = []NextHop{{Interface: "static-replacement", Weight: 7, OnLink: true}}
	best, changed := rib.Insert(family.IPv4Unicast, prefix, static)
	if changed || best.Source != primary.Source {
		t.Fatal("secondary-only update changed primary selection")
	}
	if len(changes) != 1 || changes[0].Kind != ChangeUpdate || changes[0].Best.Source != primary.Source || rib.Revision() != before+1 {
		t.Fatalf("explicit secondary update lost its notification: changes=%+v revision=%d want=%d", changes, rib.Revision(), before+1)
	}
	if len(changes[0].ECMP) != 0 {
		t.Fatal("cross-source targets entered the intra-source observation payload")
	}
	rib.Insert(family.IPv4Unicast, prefix, static)
	if len(changes) != 1 || rib.Revision() != before+1 {
		t.Fatal("unchanged replacement explicit announcement emitted another update")
	}
}

func TestLocRIBExplicitMembersRespectAdmissionRankAndDuplicates(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.254.0/24")
	primary := forwardingCandidate("bgp", "bgp-primary", 1)
	static := explicitForwardingCandidate()
	static.ECMP = append(static.ECMP, static.ECMP[0])
	hidden := forwardingCandidate("isis", "hidden-primary", 1)
	hidden.ECMP = []NextHop{{Interface: "hidden-secondary", Weight: 9}}
	usePolicy(t, rib, map[redistevents.ProtocolID]ribpolicy.Chain{hidden.Source: chainFunc(func(ribpolicy.Route) ribpolicy.Verdict {
		return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
	})}, nil)
	unequal := forwardingCandidate("ospf", "unequal-primary", 1)
	unequal.Metric++
	unequal.ECMP = []NextHop{{Interface: "unequal-secondary", Weight: 11}}
	rib.Insert(family.IPv4Unicast, prefix, primary)
	rib.Insert(family.IPv4Unicast, prefix, static)
	rib.Insert(family.IPv4Unicast, prefix, hidden)
	rib.Insert(family.IPv4Unicast, prefix, unequal)
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		members := group.ForwardingMembers(group.Paths[group.Best])
		if len(members) != 2 {
			t.Fatalf("hidden, unequal or duplicate targets changed admission: %+v", members)
		}
		for _, member := range members {
			if member.Source != static.Source {
				t.Fatalf("nonselected source entered forwarding membership: %+v", member)
			}
		}
		if len(group.Paths) != 4 {
			t.Fatalf("nonselected candidates were removed: %+v", group.Paths)
		}
	})
}

func BenchmarkLocRIBInsertUnchangedExplicitGroup(b *testing.B) {
	for _, crossSource := range []bool{false, true} {
		b.Run(map[bool]string{false: "winner", true: "nonwinner"}[crossSource], func(b *testing.B) {
			rib := NewRIB()
			prefix := netip.MustParsePrefix("198.18.255.0/24")
			path := explicitForwardingCandidate()
			if crossSource {
				rib.Insert(family.IPv4Unicast, prefix, forwardingCandidate("bgp", "bgp-primary", 1))
			}
			rib.Insert(family.IPv4Unicast, prefix, path)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				rib.Insert(family.IPv4Unicast, prefix, path)
			}
		})
	}
}
