// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing forwarding.
// Related: sr_install.go -- srInstallFromRoutes and srRoutes, the production join.
// Related: rfc8665_mapped_sid_spf_test.go -- the inter-area and external cases on this path.
//
// VALIDATES: RFC 8665 section 5 PHP for a Mapping Server Prefix-SID of an intra-area
// prefix on the post-SPF path: srRoutes copies each SPF route's originator into the route
// the installer judges, so the label is popped toward the router that originates the
// prefix and kept toward a router that only carries it.
// PREVENTS: the originator dropped between the SPF route table and the installer, which
// keeps every mapped intra-area label, and an intra-area mapped SID popped at a transit hop.
package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

var (
	// rfc8665IntraNear is 10.0.0.2's own stub network; rfc8665IntraFar is the stub of
	// 10.0.0.3, which 10.0.0.1 reaches through 10.0.0.2.
	rfc8665IntraNear = netip.MustParsePrefix("10.3.0.0/24")
	rfc8665IntraFar  = netip.MustParsePrefix("10.4.0.0/24")
)

// rfc8665IntraInstall builds router 10.0.0.1 with a real SPF computer and LSDB, all in the
// backbone: a point-to-point link to 10.0.0.2, which carries the stub 10.3.0.0/24 and a
// point-to-point link to 10.0.0.3, which carries the stub 10.4.0.0/24. 10.0.0.2 and the
// Mapping Server 10.0.0.50 advertise SRGB 16000/100; the Mapping Server binds index 11 to
// 10.3.0.0/24 and index 12 to 10.4.0.0/24 (Route Type 1, M-Flag). SPF runs,
// srInstallFromRoutes runs, and the mpls-fib entries toward 10.0.0.2 are read per FEC: a
// push names its FEC, a transit entry is keyed by this node's label (18011, 18012).
func rfc8665IntraInstall(t *testing.T) map[netip.Prefix]rfc8665SPFOutcome {
	t.Helper()
	srTestReset(t)
	eng := rfc5250ReceiverEngine(t)
	self := types.RouterID{10, 0, 0, 1}
	near := types.RouterID{10, 0, 0, 2}
	far := types.RouterID{10, 0, 0, 3}
	srWire.set(self, sr.SRConfig{Enabled: true, SRGB: []sr.LabelRange{{Base: 18000, Size: 100}}})
	bus := &srCaptureBus{}
	eng.srInstaller = newTestInstaller(bus)

	mask := [4]byte{255, 255, 255, 0}
	nearLSA := bgplsReachabilityRouter(near,
		packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(self), LinkData: [4]byte{10, 0, 0, 2}, Metric: 10},
		packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(far), LinkData: [4]byte{10, 0, 1, 2}, Metric: 10},
		packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID{10, 3, 0, 0}, LinkData: mask, Metric: 1})
	farLSA := bgplsReachabilityRouter(far,
		packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(near), LinkData: [4]byte{10, 0, 1, 3}, Metric: 10},
		packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID{10, 4, 0, 0}, LinkData: mask, Metric: 1})
	for _, lsa := range []packet.LSA{nearLSA, farLSA} {
		if !eng.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatalf("installing the Router-LSA of %s failed", lsa.Header.AdvertisingRouter)
		}
	}

	ri := packet.EncodeRITLVs([]packet.RITLV{
		{Type: sr.V4TypeSRAlgorithm, Value: []byte{0}},
		{Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(sr.LabelRange{Base: 16000, Size: 100})},
	})
	rfc8665SPFOriginate(t, eng, near, packet.RIOpaqueType, 0, ri)
	rfc8665SPFOriginate(t, eng, rfc8665MappingServer, packet.RIOpaqueType, 0, ri)
	mapped := func(prefix [4]byte, index uint32) packet.ExtPrefixTLV {
		sid := sr.PrefixSID{Flags: sr.SIDFlags{M: true}, Index: index}
		return packet.ExtPrefixTLV{RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 24,
			AF: packet.ExtPrefixAFIPv4Unicast, AddressPrefix: prefix,
			SubTLVs: []packet.ExtSubTLV{{Type: sr.V4TypePrefixSID, Value: sr.EncodePrefixSIDValue(sid)}}}
	}
	rfc8665SPFOriginate(t, eng, rfc8665MappingServer, packet.ExtPrefixOpaqueType, 1, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{
		Prefixes: []packet.ExtPrefixTLV{mapped([4]byte{10, 3, 0, 0}, 11), mapped([4]byte{10, 4, 0, 0}, 12)},
	}))

	eng.spf.Run()
	eng.srInstallFromRoutes()

	nh := netip.MustParseAddr("10.0.0.2")
	byLabel := map[uint32]netip.Prefix{18011: rfc8665IntraNear, 18012: rfc8665IntraFar}
	out := make(map[netip.Prefix]rfc8665SPFOutcome)
	for _, e := range bus.entries {
		if e.Action != mplsfibevents.ActionAdd || e.NextHop != nh {
			continue
		}
		if e.Op == mplsfibevents.OpPush {
			if len(e.OutLabels) != 1 {
				t.Fatalf("push for %s carries %d labels, want 1: %+v", e.FEC, len(e.OutLabels), e)
			}
			o := out[e.FEC]
			o.label, o.push = e.OutLabels[0], true
			out[e.FEC] = o
			continue
		}
		fec, ok := byLabel[e.InLabel]
		if !ok {
			t.Fatalf("transit entry for label %d, want 18011 or 18012: %+v", e.InLabel, e)
		}
		o := out[fec]
		switch e.Op {
		case mplsfibevents.OpPop:
			o.pop = true
		case mplsfibevents.OpSwap:
			o.swap = true
		default:
			t.Fatalf("unexpected mpls-fib op toward %s: %+v", nh, e)
		}
		out[fec] = o
	}
	return out
}

// RFC requirement: RFC8665-5-14 positive -- on the post-SPF install path, a Mapping Server
// Prefix-SID (M-Flag, index 11) for the intra-area prefix 10.3.0.0/24, whose SPF route has
// 10.0.0.2 as both originator and next hop, is popped: no label pushed toward 10.0.0.2 and
// the transit entry for 18011 is a pop.
func TestRFC8665MappedSIDIntraAreaPHPThroughSPFRoutes(t *testing.T) {
	// Goal: the SPF route's originator reaches the intra-area PHP decision. Method: a real
	// LSDB and SPF run, srInstallFromRoutes, the mpls-fib entries toward 10.0.0.2 read.
	o, ok := rfc8665IntraInstall(t)[rfc8665IntraNear]
	if !ok {
		t.Fatalf("%s: nothing installed toward 10.0.0.2", rfc8665IntraNear)
	}
	if o.push {
		t.Fatalf("%s: pushed label %d toward its originator, want PHP (no push)", rfc8665IntraNear, o.label)
	}
	if !o.pop || o.swap {
		t.Fatalf("%s: transit entry pop=%v swap=%v, want a pop", rfc8665IntraNear, o.pop, o.swap)
	}
}

// RFC requirement: RFC8665-5-14 negative -- on the same path, a Mapping Server Prefix-SID
// (index 12) for the intra-area prefix 10.4.0.0/24, originated by 10.0.0.3 and reached
// through 10.0.0.2, keeps its label: push 16012 toward 10.0.0.2 and a swap for 18012.
func TestRFC8665MappedSIDIntraAreaKeepsLabelTowardTransitRouter(t *testing.T) {
	// Goal: an intra-area mapped SID is popped only toward the prefix originator. Method:
	// the positive's network, the entries for 10.0.0.3's stub read.
	o := rfc8665IntraInstall(t)[rfc8665IntraFar]
	if !o.push {
		t.Fatalf("%s: no label pushed toward 10.0.0.2, want 16012 kept: %+v", rfc8665IntraFar, o)
	}
	if o.label != 16012 {
		t.Fatalf("%s: pushed label %d, want 16012", rfc8665IntraFar, o.label)
	}
	if o.pop || !o.swap {
		t.Fatalf("%s: transit entry pop=%v swap=%v, want a swap", rfc8665IntraFar, o.pop, o.swap)
	}
}
