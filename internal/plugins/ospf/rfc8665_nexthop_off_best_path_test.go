// VALIDATES: RFC 8665 Section 5 -- the next-hop router's own E-, NP- and M-Flags shape the
// outgoing label whenever that router advertised the SID for the prefix, even when the SPF
// best path resolves the prefix to a different advertiser of the same (anycast) SID.
// PREVENTS: an anycast next hop whose own NP clear or E set is ignored because the route's
// best path ends at another advertiser, so a transit swap is installed toward it instead.
package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// offBestPathRouters are the four routers of the off-best-path fixture: best is the
// advertiser the SPF route resolves the prefix to, next is the SPF next hop, and other is a
// third router binding the same SID that is not on the path.
type offBestPathRouters struct {
	best, next, other types.RouterID
}

func newOffBestPathRouters() offBestPathRouters {
	return offBestPathRouters{
		best:  types.RouterID{10, 0, 0, 9},
		next:  types.RouterID{10, 0, 0, 2},
		other: types.RouterID{10, 0, 0, 7},
	}
}

// installOffBestPath builds the received Prefix-SID view through the real table
// (srPrefixSIDTable, the producer srRemotePrefixSIDs and srRemotePrefixSIDsV6 both use),
// with the best-path advertiser received first, then runs the installer for a route whose
// best path ends at r.best and whose single next hop is r.next. It returns every mpls-fib
// entry emitted.
func installOffBestPath(t *testing.T, inst *srInstaller, bus *srCaptureBus, r offBestPathRouters, prefix netip.Prefix, nh netip.Addr, received []srRemotePrefixSID) []mplsfibevents.Entry {
	t.Helper()
	table := newSRPrefixSIDTable()
	for _, rs := range received {
		table.add(rs, prefix, types.AreaID{})
	}
	sids := table.byPrefix()
	if got := sids[prefix]; got.Duplicate || got.Originator != r.best {
		t.Fatalf("fixture: the best-path advertiser %v must be the resolved originator: %+v", r.best, got)
	}
	caps := map[types.RouterID]sr.SRGB{
		r.best:  sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}}),
		r.next:  sr.NewSRGB([]sr.LabelRange{{Base: 17000, Size: 100}}),
		r.other: sr.NewSRGB([]sr.LabelRange{{Base: 19000, Size: 100}}),
	}
	algos := map[types.RouterID][]uint8{r.best: {0}, r.next: {0}, r.other: {0}}
	routes := []srRoute{{Prefix: prefix, Origin: r.best, NextHops: []srNextHop{{Addr: nh, Router: r.next}}}}
	inst.installRoutes(routes, sids, caps, algos, sr.NewSRGB([]sr.LabelRange{{Base: 18000, Size: 100}}))
	return bus.entries
}

// TestRFC8665NextHopFlagsAppliedOffBestPath checks that the flags of a next hop which
// advertised the SID are applied when the best path does not end at it. Goal: RFC 8665
// Section 5, "This MUST be done regardless of whether the next-hop router contributes to
// the best path to the prefix." Method: the anycast prefix 10.0.0.99/32 is bound to index 9
// by 10.0.0.9, where the best path ends, and by the next hop 10.0.0.2 itself; the next
// hop's own flags must decide the outgoing label.
// RFC requirement: RFC8665-5-16 positive -- with the best path ending at 10.0.0.9, the next
// hop 10.0.0.2 that also advertised the SID gets its own flags applied: its NP clear pops
// (no label imposed toward it, PHP pop of 18009), and its NP and E set impose the IPv4
// Explicit NULL label 0 rather than its SRGB label 17009.
func TestRFC8665NextHopFlagsAppliedOffBestPath(t *testing.T) {
	cases := []struct {
		name  string
		flags sr.SIDFlags
		push  uint32 // 0 with pop: no label imposed (PHP)
		pop   bool
	}{
		{name: "np-clear-pops", flags: sr.SIDFlags{}, pop: true},
		{name: "np-and-e-explicit-null", flags: sr.SIDFlags{NP: true, E: true}, push: sr.ExplicitNullV4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newOffBestPathRouters()
			prefix := netip.MustParsePrefix("10.0.0.99/32")
			nh := netip.MustParseAddr("10.0.12.2")
			bus := &srCaptureBus{}
			entries := installOffBestPath(t, newTestInstaller(bus), bus, r, prefix, nh, []srRemotePrefixSID{
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

// TestRFC8665OffPathAdvertiserFlagsNotAppliedToNextHop checks that only the next hop's own
// advertisement counts. Goal: RFC 8665 Section 5 applies the flags "advertised by the
// next-hop router if that router advertised the SID for the prefix". Method: the anycast
// prefix is bound by 10.0.0.9 (best path) and 10.0.0.7 (off path), both NP clear; the next
// hop 10.0.0.2 advertised nothing for it.
// RFC requirement: RFC8665-5-16 negative -- a next hop that did not advertise the SID has no
// flags of its own: the NP clear of the best-path and off-path advertisers is not applied
// toward it, and its SRGB label 17009 is imposed, with no PHP pop.
func TestRFC8665OffPathAdvertiserFlagsNotAppliedToNextHop(t *testing.T) {
	r := newOffBestPathRouters()
	prefix := netip.MustParsePrefix("10.0.0.99/32")
	nh := netip.MustParseAddr("10.0.12.2")
	bus := &srCaptureBus{}
	entries := installOffBestPath(t, newTestInstaller(bus), bus, r, prefix, nh, []srRemotePrefixSID{
		{Originator: r.best, SID: sr.PrefixSID{Index: 9}},
		{Originator: r.other, SID: sr.PrefixSID{Index: 9}},
	})
	push := entryOp(entries, mplsfibevents.OpPush)
	if push == nil || len(push.OutLabels) != 1 || push.OutLabels[0] != 17009 || push.NextHop != nh {
		t.Fatalf("a next hop that advertised no SID must get its SRGB label 17009 imposed: %+v", push)
	}
	if pop := entryOp(entries, mplsfibevents.OpPop); pop != nil {
		t.Fatalf("no PHP pop toward a next hop that advertised no SID: %+v", pop)
	}
}
