// VALIDATES: cross-source selected forwarding membership stays authoritative.
// PREVENTS: a consumer reconstructing a second candidate selector for ECMP.

package locrib

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

func forwardingCandidate(protocol, device string, instance uint32) Path {
	return Path{Source: redistevents.RegisterProtocol(protocol), Instance: instance,
		Interface: device, DistanceOverride: 20, HasDistanceOverride: true, Metric: 10}
}

func TestLocRIBForwardingMembersCrossSource(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.240.0/24")
	primary := forwardingCandidate("static", "primary", 1)
	local := forwardingCandidate("static", "local", 2)
	member := forwardingCandidate("ospf", "member", 1)
	member.Labels = []uint32{201, 202}
	member.SRv6SID = netip.MustParseAddr("2001:db8:240::1")
	rib.Insert(family.IPv4Unicast, prefix, primary)
	rib.Insert(family.IPv4Unicast, prefix, local)
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		members := group.ForwardingMembers(group.Paths[group.Best])
		if len(members) != 1 || members[0].Source != primary.Source || members[0].Interface != "local" {
			t.Fatalf("healthy same-source control: %+v", members)
		}
	})
	rib.Insert(family.IPv4Unicast, prefix, member)
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		best := group.Paths[group.Best]
		if best.Source != primary.Source || best.Instance != primary.Instance {
			t.Fatal("adding an equal-cost member changed the authoritative primary")
		}
		if siblings := group.ECMPMembers(best); len(siblings) != 1 || siblings[0].Interface != "local" {
			t.Fatalf("cross-source membership changed the BGP observation contract: %+v", siblings)
		}
		members := group.ForwardingMembers(best)
		if len(members) != 2 {
			t.Fatalf("selected forwarding lost a cross-source target: %+v", members)
		}
		for _, got := range members {
			if got.Source == member.Source {
				if got.Interface != member.Interface || !slices.Equal(got.Labels, member.Labels) || got.SRv6SID != member.SRv6SID {
					t.Fatalf("cross-source forwarding metadata changed: %+v", got)
				}
				return
			}
		}
		t.Fatal("cross-source forwarding member lost its source identity")
	})
}

func TestLocRIBForwardingMembersKeepDistinctOwners(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.241.0/24")
	primary := forwardingCandidate("static", "primary", 1)
	first := forwardingCandidate("ospf", "member", 1)
	second := forwardingCandidate("isis", "member", 1)
	rib.Insert(family.IPv4Unicast, prefix, primary)
	rib.Insert(family.IPv4Unicast, prefix, first)
	rib.Insert(family.IPv4Unicast, prefix, second)
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		members := group.ForwardingMembers(group.Paths[group.Best])
		if len(members) != 2 {
			t.Fatalf("equal targets were deduplicated before owner permission: %+v", members)
		}
		owners := map[redistevents.ProtocolID]bool{}
		for _, member := range members {
			owners[member.Source] = true
			if member.Interface != "member" {
				t.Fatalf("member target changed: %+v", member)
			}
		}
		if !owners[first.Source] || !owners[second.Source] {
			t.Fatalf("duplicate target owners were erased: %+v", members)
		}
	})
}

func TestLocRIBForwardingMembersKeepOtherOwnerAtPrimaryTarget(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.249.0/24")
	primary := forwardingCandidate("static", "shared", 1)
	primary.Labels = []uint32{101}
	sameSource := forwardingCandidate("static", "shared", 2)
	sameSource.Labels = []uint32{102}
	rib.Insert(family.IPv4Unicast, prefix, primary)
	rib.Insert(family.IPv4Unicast, prefix, sameSource)
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		if members := group.ForwardingMembers(group.Paths[group.Best]); len(members) != 0 {
			t.Fatalf("same-owner primary target entered its own member set: %+v", members)
		}
	})
	otherSource := forwardingCandidate("ospf", "shared", 1)
	otherSource.Labels = []uint32{201}
	rib.Insert(family.IPv4Unicast, prefix, otherSource)
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		best := group.Paths[group.Best]
		if best.Source != primary.Source || best.Instance != primary.Instance {
			t.Fatal("equal-cost other owner changed the authoritative primary")
		}
		if siblings := group.ECMPMembers(best); len(siblings) != 0 {
			t.Fatalf("other owner changed the intra-source observation: %+v", siblings)
		}
		members := group.ForwardingMembers(best)
		if len(members) != 1 || members[0].Source != otherSource.Source || members[0].Interface != "shared" || !slices.Equal(members[0].Labels, otherSource.Labels) {
			t.Fatalf("other owner at the primary target lost its metadata: %+v", members)
		}
	})
}

func TestLocRIBForwardingMembersUnionExplicitGroup(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.248.0/24")
	primary := forwardingCandidate("static", "primary", 1)
	primary.Labels = []uint32{101}
	primary.ECMP = []NextHop{{Interface: "explicit", Weight: 3}}
	member := forwardingCandidate("ospf", "other-source", 1)
	member.Labels = []uint32{201}
	rib.Insert(family.IPv4Unicast, prefix, primary)
	rib.Insert(family.IPv4Unicast, prefix, member)
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		best := group.Paths[group.Best]
		if !slices.Equal(group.ECMPNextHops(best), primary.ECMP) {
			t.Fatal("forwarding membership rewrote the producer's explicit ECMP group")
		}
		members := group.ForwardingMembers(best)
		if len(members) != 2 {
			t.Fatalf("explicit producer group suppressed another selected source: %+v", members)
		}
		seen := map[string]bool{}
		for _, got := range members {
			seen[got.Interface] = true
			if got.Interface == "explicit" {
				if got.Source != primary.Source || got.Weight != 3 || !slices.Equal(got.Labels, primary.Labels) {
					t.Fatalf("explicit target lost producer ownership/metadata: %+v", got)
				}
			}
			if got.Interface == "other-source" {
				if got.Source != member.Source || !slices.Equal(got.Labels, member.Labels) {
					t.Fatalf("other source lost ownership/metadata: %+v", got)
				}
			}
		}
		if !seen["explicit"] || !seen["other-source"] {
			t.Fatalf("selected union lost a permitted target: %+v", members)
		}
	})
}

func TestLocRIBCrossSourceMemberChangesNotify(t *testing.T) {
	for _, operation := range []string{"labels", "sid", "remove", "policy"} {
		t.Run(operation, func(t *testing.T) {
			rib := NewRIB()
			prefix := netip.MustParsePrefix("198.18.242.0/24")
			primary := forwardingCandidate("static", "primary", 1)
			member := forwardingCandidate("ospf", "member", 1)
			member.Labels = []uint32{201}
			rib.Insert(family.IPv4Unicast, prefix, primary)
			rib.Insert(family.IPv4Unicast, prefix, member)
			var changes []Change
			unsubscribe := rib.OnChange(func(change Change) { changes = append(changes, change) })
			defer unsubscribe()
			before := rib.Revision()
			rib.Insert(family.IPv4Unicast, prefix, member)
			if len(changes) != 0 || rib.Revision() != before {
				t.Fatal("healthy duplicate candidate control changed the selected view")
			}
			switch operation {
			case "labels":
				member.Labels = []uint32{202, 203}
				rib.Insert(family.IPv4Unicast, prefix, member)
			case "sid":
				member.SRv6SID = netip.MustParseAddr("2001:db8:242::1")
				rib.Insert(family.IPv4Unicast, prefix, member)
			case "remove":
				rib.Remove(family.IPv4Unicast, prefix, member.Source, member.Instance)
			case "policy":
				usePolicy(t, rib, map[redistevents.ProtocolID]ribpolicy.Chain{member.Source: chainFunc(func(ribpolicy.Route) ribpolicy.Verdict {
					return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
				})}, nil)
			}
			if len(changes) != 1 || changes[0].Kind != ChangeUpdate || changes[0].Best.Source != primary.Source || rib.Revision() != before+1 {
				t.Fatalf("cross-source %s lost authoritative notification: changes=%+v revision=%d want=%d", operation, changes, rib.Revision(), before+1)
			}
			if len(changes[0].ECMP) != 0 {
				t.Fatal("forwarding notification altered the intra-source observation payload")
			}
		})
	}
}

func TestLocRIBForwardingMembersRespectAdmissionAndRank(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.243.0/24")
	primary := forwardingCandidate("static", "primary", 1)
	permitted := forwardingCandidate("ospf", "permitted", 1)
	hidden := forwardingCandidate("isis", "hidden", 1)
	usePolicy(t, rib, map[redistevents.ProtocolID]ribpolicy.Chain{hidden.Source: chainFunc(func(ribpolicy.Route) ribpolicy.Verdict {
		return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
	})}, nil)
	rib.Insert(family.IPv4Unicast, prefix, primary)
	rib.Insert(family.IPv4Unicast, prefix, permitted)
	rib.Insert(family.IPv4Unicast, prefix, hidden)
	higherMetric := forwardingCandidate("ospf", "higher-metric", 2)
	higherMetric.Metric++
	rib.Insert(family.IPv4Unicast, prefix, higherMetric)
	higherDistance := forwardingCandidate("ospf", "higher-distance", 3)
	higherDistance.DistanceOverride++
	rib.Insert(family.IPv4Unicast, prefix, higherDistance)
	noTarget := forwardingCandidate("ospf", "", 4)
	rib.Insert(family.IPv4Unicast, prefix, noTarget)
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		members := group.ForwardingMembers(group.Paths[group.Best])
		if len(members) != 1 || members[0].Interface != permitted.Interface || members[0].Source != permitted.Source {
			t.Fatalf("admission/rank changed the permitted forwarding set: %+v", members)
		}
		if len(group.Paths) != 6 {
			t.Fatalf("unselected candidates were erased instead of retained: %+v", group.Paths)
		}
	})
}

func TestLocRIBForwardingMemberOwnerReplacementNotifies(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.247.0/24")
	primary := forwardingCandidate("static", "primary", 1)
	first := forwardingCandidate("ospf", "member", 1)
	second := forwardingCandidate("isis", "member", 1)
	reject := chainFunc(func(ribpolicy.Route) ribpolicy.Verdict { return ribpolicy.Verdict{Action: ribpolicy.ActionReject} })
	usePolicy(t, rib, map[redistevents.ProtocolID]ribpolicy.Chain{second.Source: reject}, nil)
	rib.Insert(family.IPv4Unicast, prefix, primary)
	rib.Insert(family.IPv4Unicast, prefix, first)
	rib.Insert(family.IPv4Unicast, prefix, second)
	var changes []Change
	unsubscribe := rib.OnChange(func(change Change) { changes = append(changes, change) })
	defer unsubscribe()
	before := rib.Revision()
	usePolicy(t, rib, map[redistevents.ProtocolID]ribpolicy.Chain{first.Source: reject}, nil)
	if len(changes) != 1 || changes[0].Kind != ChangeUpdate || changes[0].Best.Source != primary.Source || rib.Revision() != before+1 {
		t.Fatalf("owner-only replacement did not dirty the selected view: changes=%+v revision=%d want=%d", changes, rib.Revision(), before+1)
	}
	rib.Inspect(family.IPv4Unicast, prefix, func(group PathGroup) {
		members := group.ForwardingMembers(group.Paths[group.Best])
		if len(members) != 1 || members[0].Source != second.Source || members[0].Interface != "member" {
			t.Fatalf("owner-only replacement lost selected metadata: %+v", members)
		}
	})
}
