// VALIDATES: spec-ospf-ext-5 AC-6/AC-8/AC-9/AC-10/AC-11 -- the reception->install
// driver computes the outgoing label from the NEXT-HOP router's SRGB (RFC 8665 §5:
// SRGB(next-hop).Label(index), not the originator's), applies the NP/E/M PHP rules
// only at the penultimate hop (next-hop == originator) and swaps unconditionally at a
// transit hop, emits push (ingress) + swap (transit) toward the SPF next-hop, ignores
// unknown-algorithm and duplicate Prefix-SIDs, and withdraws stale FECs.
// PREVENTS: a mislabelled push (originator SRGB used at a transit hop), an install for
// a non-SR next hop, PHP applied at a transit hop, a leaked entry.
package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func newTestInstaller(bus *srCaptureBus) *srInstaller {
	return &srInstaller{fib: newSRFIB(bus, mplsSourceOSPFSR), af: "ipv4", explicitNull: sr.ExplicitNullV4}
}

func TestSRInstallPrefixSIDPushAndSwap(t *testing.T) {
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nh := netip.MustParseAddr("10.0.0.2")
	// RFC 8665 §5: the label is SRGB(next-hop).Label(index). Here the SPF next-hop router
	// IS the SID originator (the destination is directly attached: this node is the
	// penultimate hop), so the next-hop SRGB is the originator's and the label is 16009 --
	// the same value as before, but now for the RFC-correct reason (next-hop SRGB, not a
	// hard-coded "originator SRGB"). The heterogeneous-SRGB transit case is covered by
	// TestSRInstallHeterogeneousSRGB.
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{{Addr: nh, Router: orig}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{
		fec: {Originator: orig, SID: sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: 9}},
	}
	// Next-hop (== originator) SRGB base 16000 -> label 16009; my SRGB base 16500 -> in-label 16509.
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	algos := map[types.RouterID][]uint8{orig: {0}}
	mySRGB := sr.NewSRGB([]sr.LabelRange{{Base: 16500, Size: 100}})

	inst.installRoutes(routes, sids, caps, algos, mySRGB)
	var push, swap *mplsfibevents.Entry
	for i := range bus.entries {
		switch bus.entries[i].Op {
		case mplsfibevents.OpPush:
			push = &bus.entries[i]
		case mplsfibevents.OpSwap:
			swap = &bus.entries[i]
		default:
		}
	}
	if push == nil || push.OutLabels[0] != 16009 || push.NextHop != nh {
		t.Fatalf("ingress push wrong: %+v", push)
	}
	if swap == nil || swap.InLabel != 16509 || swap.OutLabels[0] != 16009 {
		t.Fatalf("transit swap wrong: %+v", swap)
	}
}

func TestSRInstallUnknownAlgorithmIgnored(t *testing.T) {
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{{Addr: netip.MustParseAddr("10.0.0.2"), Router: orig}}}}
	// Algorithm 1 (Strict SPF) is not computed by Ze -> recorded, not installed (AC-10).
	sids := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: orig, SID: sr.PrefixSID{Algorithm: 1, Index: 9}}}
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	algos := map[types.RouterID][]uint8{orig: {0, 1}}
	inst.installRoutes(routes, sids, caps, algos, sr.SRGB{})
	if len(bus.entries) != 0 {
		t.Fatalf("unsupported-algorithm Prefix-SID must not install: %+v", bus.entries)
	}
}

func TestSRInstallDuplicateIgnored(t *testing.T) {
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{{Addr: netip.MustParseAddr("10.0.0.2"), Router: orig}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: orig, SID: sr.PrefixSID{Index: 9}, Duplicate: true}}
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	algos := map[types.RouterID][]uint8{orig: {0}}
	inst.installRoutes(routes, sids, caps, algos, sr.SRGB{})
	if len(bus.entries) != 0 {
		t.Fatalf("duplicate Prefix-SID must not install: %+v", bus.entries)
	}
}

func TestSRInstallNonSRNextHop(t *testing.T) {
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{{Addr: netip.MustParseAddr("10.0.0.2"), Router: orig}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: orig, SID: sr.PrefixSID{Index: 9}}}
	// No capabilities for the next-hop router -> non-SR next hop, nothing installed.
	inst.installRoutes(routes, sids, map[types.RouterID]sr.SRGB{}, map[types.RouterID][]uint8{}, sr.SRGB{})
	if len(bus.entries) != 0 {
		t.Fatalf("must not install toward a non-SR originator: %+v", bus.entries)
	}
}

func TestSRInstallIndexOutOfRange(t *testing.T) {
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{{Addr: netip.MustParseAddr("10.0.0.2"), Router: orig}}}}
	// Index 500 beyond the SRGB size 100 -> rejected (AC-6).
	sids := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: orig, SID: sr.PrefixSID{Index: 500}}}
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	algos := map[types.RouterID][]uint8{orig: {0}}
	inst.installRoutes(routes, sids, caps, algos, sr.SRGB{})
	if len(bus.entries) != 0 {
		t.Fatalf("out-of-range index must not install: %+v", bus.entries)
	}
}

func TestSRInstallWithdrawsStale(t *testing.T) {
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nh := netip.MustParseAddr("10.0.0.2")
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{{Addr: nh, Router: orig}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: orig, SID: sr.PrefixSID{Flags: NPtrue(), Index: 9}}}
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	algos := map[types.RouterID][]uint8{orig: {0}}
	mySRGB := sr.NewSRGB([]sr.LabelRange{{Base: 16500, Size: 100}})
	inst.installRoutes(routes, sids, caps, algos, mySRGB)
	before := len(bus.entries)
	// Next run: the route is gone -> the FEC must be withdrawn.
	inst.installRoutes(nil, nil, caps, algos, mySRGB)
	var sawRemovePush bool
	for _, e := range bus.entries[before:] {
		if e.Action == mplsfibevents.ActionRemove && e.Op == mplsfibevents.OpPush && e.FEC == fec {
			sawRemovePush = true
		}
	}
	if !sawRemovePush {
		t.Fatalf("stale FEC must be withdrawn on the next run")
	}
}

// TestSRInstallHeterogeneousSRGB is the RFC 8665 §5 regression: the pushed/swapped label
// is SRGB(NEXT-HOP router).Label(index), NOT the originator's SRGB, and the NP/E/M
// PHP/Explicit-NULL rules apply ONLY at the penultimate hop (next-hop == originator).
// Topology I -> T2 -> E: E (originator) SRGB base 16000 index 9 -> its own label 16009;
// T2 (transit) SRGB base 17000 -> label 17009. At the ingress the SPF next-hop is T2, so
// the pushed label is T2's 17009 (a homogeneous-SRGB fix would wrongly push 16009), and
// E's NP flag is ignored at the transit hop (swap on unconditionally, no PHP).
func TestSRInstallHeterogeneousSRGB(t *testing.T) {
	e := types.RouterID{10, 0, 0, 9}  // originator / egress
	t2 := types.RouterID{10, 0, 0, 2} // transit next-hop router
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nhT2 := netip.MustParseAddr("10.0.0.2") // interface address that resolves to T2
	// NP set on the Prefix-SID: it MUST be ignored at the transit hop (next-hop != originator).
	sids := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: e, SID: sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: 9}}}
	caps := map[types.RouterID]sr.SRGB{
		e:  sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}}),
		t2: sr.NewSRGB([]sr.LabelRange{{Base: 17000, Size: 100}}),
	}
	algos := map[types.RouterID][]uint8{e: {0}, t2: {0}}
	mySRGB := sr.NewSRGB([]sr.LabelRange{{Base: 18000, Size: 100}}) // my in-label 18009

	// Transit case: next-hop is T2 (T2 != E) -> label from T2's SRGB, NP ignored.
	busTransit := &srCaptureBus{}
	instTransit := newTestInstaller(busTransit)
	instTransit.installRoutes(
		[]srRoute{{Prefix: fec, Origin: e, NextHops: []srNextHop{{Addr: nhT2, Router: t2}}}},
		sids, caps, algos, mySRGB)
	var push, swap *mplsfibevents.Entry
	for i := range busTransit.entries {
		switch busTransit.entries[i].Op {
		case mplsfibevents.OpPush:
			push = &busTransit.entries[i]
		case mplsfibevents.OpSwap:
			swap = &busTransit.entries[i]
		default:
		}
	}
	if push == nil || push.OutLabels[0] != 17009 || push.NextHop != nhT2 {
		t.Fatalf("transit ingress push must use T2's SRGB label 17009 (not the originator's 16009): %+v", push)
	}
	if swap == nil || swap.InLabel != 18009 || swap.OutLabels[0] != 17009 {
		t.Fatalf("transit swap must be 18009 -> T2's 17009: %+v", swap)
	}

	// Penultimate case: next-hop IS E (E == originator) with NP clear -> PHP: no ingress push.
	// This proves the NP/E/M rules apply only at the penultimate hop.
	sidsPHP := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: e, SID: sr.PrefixSID{Index: 9}}}
	nhE := netip.MustParseAddr("10.0.9.9") // interface address that resolves to E
	busPHP := &srCaptureBus{}
	instPHP := newTestInstaller(busPHP)
	instPHP.installRoutes(
		[]srRoute{{Prefix: fec, Origin: e, NextHops: []srNextHop{{Addr: nhE, Router: e}}}},
		sidsPHP, caps, algos, mySRGB)
	for i := range busPHP.entries {
		if busPHP.entries[i].Op == mplsfibevents.OpPush && busPHP.entries[i].Action == mplsfibevents.ActionAdd {
			t.Fatalf("penultimate hop with NP=0 must PHP (no ingress push): %+v", busPHP.entries[i])
		}
	}
}

// NPtrue returns a SIDFlags with NP set (test helper avoiding an inline struct literal).
func NPtrue() sr.SIDFlags { return sr.SIDFlags{NP: true} }
