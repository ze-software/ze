// VALIDATES: RFC 8666 Section 6 -- for an IPv6 Prefix-SID, the next-hop router's own E-,
// NP- and M-Flags shape the outgoing label whenever that router advertised the SID for the
// prefix, even when the SPF best path resolves the prefix to another advertiser of the same
// (anycast) SID.
// PREVENTS: an anycast IPv6 next hop whose own NP clear or E set is ignored because the
// best path ends elsewhere, so a transit swap is installed toward it instead.
package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

// TestRFC8666NextHopFlagsAppliedOffBestPath checks that the flags of an IPv6 next hop which
// advertised the SID are applied when the best path does not end at it. Goal: RFC 8666
// Section 6, "This MUST be done regardless of whether the next-hop router contributes to
// the best path to the prefix." Method: the anycast prefix 2001:db8::99/128 is bound to
// index 9 by 10.0.0.9, where the best path ends, and by the next hop 10.0.0.2 itself; the
// next hop's own flags must decide the outgoing label.
// RFC requirement: RFC8666-6-18 positive -- with the best path ending at 10.0.0.9, the next
// hop 10.0.0.2 that also advertised the SID gets its own flags applied: its NP clear pops
// (no label imposed toward it, PHP pop of 18009), and its NP and E set impose the IPv6
// Explicit NULL label 2 rather than its SRGB label 17009.
func TestRFC8666NextHopFlagsAppliedOffBestPath(t *testing.T) {
	cases := []struct {
		name  string
		flags sr.SIDFlags
		push  uint32
		pop   bool
	}{
		{name: "np-clear-pops", flags: sr.SIDFlags{}, pop: true},
		{name: "np-and-e-explicit-null", flags: sr.SIDFlags{NP: true, E: true}, push: sr.ExplicitNullV6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newOffBestPathRouters()
			prefix := netip.MustParsePrefix("2001:db8::99/128")
			nh := netip.MustParseAddr("fe80::2")
			bus := &srCaptureBus{}
			entries := installOffBestPath(t, newTestInstallerV6(bus), bus, r, prefix, nh, []srRemotePrefixSID{
				{Originator: r.best, SID: sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: 9}},
				{Originator: r.next, SID: sr.PrefixSID{Flags: tc.flags, Index: 9}},
			})
			push := entryOp(entries, mplsfibevents.OpPush)
			if tc.pop {
				if push != nil {
					t.Fatalf("NP clear from the advertising next hop must impose no label toward it: %+v", push)
				}
				if pop := entryOp(entries, mplsfibevents.OpPop); pop == nil || pop.InLabel != 18009 || pop.NextHop != nh {
					t.Fatalf("NP clear from the advertising next hop must program the PHP pop of 18009: %+v", entries)
				}
				return
			}
			if push == nil || len(push.OutLabels) != 1 || push.OutLabels[0] != tc.push || push.NextHop != nh {
				t.Fatalf("flags %+v from the advertising next hop must impose label %d: %+v", tc.flags, tc.push, push)
			}
		})
	}
}

// TestRFC8666OffPathAdvertiserFlagsNotAppliedToNextHop checks that only the IPv6 next hop's
// own advertisement counts. Goal: RFC 8666 Section 6 applies the flags "advertised by the
// next-hop router if that router advertised the SID for the prefix". Method: the anycast
// prefix is bound by 10.0.0.9 (best path) and 10.0.0.7 (off path), both NP clear; the next
// hop 10.0.0.2 advertised nothing for it.
// RFC requirement: RFC8666-6-18 negative -- an IPv6 next hop that did not advertise the SID
// has no flags of its own: the NP clear of the best-path and off-path advertisers is not
// applied toward it, and its SRGB label 17009 is imposed, with no PHP pop.
func TestRFC8666OffPathAdvertiserFlagsNotAppliedToNextHop(t *testing.T) {
	r := newOffBestPathRouters()
	prefix := netip.MustParsePrefix("2001:db8::99/128")
	nh := netip.MustParseAddr("fe80::2")
	bus := &srCaptureBus{}
	entries := installOffBestPath(t, newTestInstallerV6(bus), bus, r, prefix, nh, []srRemotePrefixSID{
		{Originator: r.best, SID: sr.PrefixSID{Index: 9}},
		{Originator: r.other, SID: sr.PrefixSID{Index: 9}},
	})
	push := entryOp(entries, mplsfibevents.OpPush)
	if push == nil || len(push.OutLabels) != 1 || push.OutLabels[0] != 17009 || push.NextHop != nh {
		t.Fatalf("an IPv6 next hop that advertised no SID must get its SRGB label 17009 imposed: %+v", push)
	}
	if pop := entryOp(entries, mplsfibevents.OpPop); pop != nil {
		t.Fatalf("no PHP pop toward an IPv6 next hop that advertised no SID: %+v", pop)
	}
}
