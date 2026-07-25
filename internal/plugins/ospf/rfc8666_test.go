// VALIDATES: RFC 8666 (OSPFv3 Extensions for Segment Routing) forwarding and lifecycle
// obligations -- the outgoing label for a received IPv6 Prefix-SID follows the NP/E/M
// truth table of the NEXT-HOP router (§6), a Prefix-SID whose algorithm the originator
// never advertised or that is duplicated for one prefix installs nothing, the ABR
// propagates a Prefix-SID between areas with a conformant Range Size, and an Adj-SID is
// withdrawn when its adjacency drops.
// PREVENTS: a penultimate hop that pops a No-PHP Prefix-SID, an Explicit-NULL that
// imposes the IPv4 label 0 on an IPv6 FEC, a transit hop that applies the originator's
// PHP flags, an unadvertised-algorithm SID reaching the MPLS FIB, a stale Adj-SID pop
// entry left behind after the neighbor is gone.
package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
)

// rfc8666Fixture is the shared IPv6 SR install fixture: one loopback FEC advertised by
// router 9.9.9.9 with SID index 9, reachable over fe80::2. The originator's SRGB starts
// at 16000 (so index 9 is label 16009) and this node's own SRGB at 18000 (transit
// in-label 18009).
type rfc8666Fixture struct {
	orig   types.RouterID
	prefix netip.Prefix
	nh     netip.Addr
	caps   map[types.RouterID]sr.SRGB
	algos  map[types.RouterID][]uint8
	mySRGB sr.SRGB
}

func newRFC8666Fixture() rfc8666Fixture {
	orig := types.RouterID{9, 9, 9, 9}
	return rfc8666Fixture{
		orig:   orig,
		prefix: netip.MustParsePrefix("2001:db8::9/128"),
		nh:     netip.MustParseAddr("fe80::2"),
		caps:   map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})},
		algos:  map[types.RouterID][]uint8{orig: {0}},
		mySRGB: sr.NewSRGB([]sr.LabelRange{{Base: 18000, Size: 100}}),
	}
}

// installV6 runs the IPv6 SR installer for one Prefix-SID carried through the real OSPFv3
// wire codec (encode as an E-Intra-Area-Prefix TLV, decode it back) toward one next-hop
// router, and returns every mpls-fib entry it emitted.
func (f rfc8666Fixture) installV6(t *testing.T, sid sr.PrefixSID, nhRouter types.RouterID) []mplsfibevents.Entry {
	t.Helper()
	value := ospfv3packet.AppendSubTLVs(
		v6PrefixTLVFixed(f.prefix),
		[]ospfv3packet.ExtendedTLV{{Type: sr.V6TypePrefixSID, Value: sr.EncodePrefixSIDValueV6(sid)}},
	)
	pfx, decoded, ok := v6PrefixSIDFromTLV(ospfv3packet.ExtendedTLV{Type: extTLVIntraAreaPrefix, Value: value})
	if !ok || pfx != f.prefix {
		t.Fatalf("Prefix-SID did not survive the OSPFv3 carriage: %v ok=%v", pfx, ok)
	}
	bus := &srCaptureBus{}
	inst := newTestInstallerV6(bus)
	routes := []srRoute{{Prefix: pfx, Origin: f.orig, NextHops: []srNextHop{{Addr: f.nh, Router: nhRouter}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{pfx: {Originator: f.orig, SID: decoded}}
	inst.installRoutes(routes, sids, f.caps, f.algos, f.mySRGB)
	return bus.entries
}

// v6PrefixTLVFixed builds the RFC 8362 §3.11 Intra-Area Prefix TLV fixed fields for a
// prefix: Metric(4) PrefixLength(1) PrefixOptions(1) Reserved(2) AddressPrefix(words).
func v6PrefixTLVFixed(prefix netip.Prefix) []byte {
	plen := uint8(prefix.Bits())
	words := v6PrefixTLVWordBytes(plen)
	fixed := make([]byte, 8+words)
	fixed[4] = plen
	addr := prefix.Addr().As16()
	copy(fixed[8:8+words], addr[:min(words, len(addr))])
	return fixed
}

// entryOp returns the first Add entry with the given op, or nil.
func entryOp(entries []mplsfibevents.Entry, op mplsfibevents.Op) *mplsfibevents.Entry {
	for i := range entries {
		if entries[i].Op == op && entries[i].Action == mplsfibevents.ActionAdd {
			return &entries[i]
		}
	}
	return nil
}

// RFC requirement: RFC8666-6-1 positive -- NP set: the penultimate hop does NOT pop. The
// label stays on the stack (ingress push 16009, transit swap 18009 -> 16009) and no pop
// entry is programmed for this node's own SID label.
// RFC requirement: RFC8666-6-12 positive -- NP set with E clear keeps the Prefix-SID on
// top of the stack.
// RFC requirement: RFC8666-6-11 negative -- with NP SET the upstream neighbor must NOT
// pop: no pop entry appears (the pop is conditional on NP being clear).
// RFC requirement: RFC8666-6-2 negative -- with the E-Flag CLEAR the Prefix-SID is not
// replaced by the Explicit NULL label: the imposed label is the SRGB label 16009, not 2.
// RFC requirement: RFC8666-6-13 negative -- NP set and E clear does not yield an Explicit
// NULL label.
func TestRFC8666NoPHPKeepsPrefixSIDOnStack(t *testing.T) {
	f := newRFC8666Fixture()
	entries := f.installV6(t, sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: 9}, f.orig)

	push := entryOp(entries, mplsfibevents.OpPush)
	if push == nil || len(push.OutLabels) != 1 || push.OutLabels[0] != 16009 || push.NextHop != f.nh {
		t.Fatalf("NP set must impose the Prefix-SID label 16009 at ingress: %+v", push)
	}
	swap := entryOp(entries, mplsfibevents.OpSwap)
	if swap == nil || swap.InLabel != 18009 || swap.OutLabels[0] != 16009 {
		t.Fatalf("NP set must swap 18009 -> 16009 in transit: %+v", swap)
	}
	if pop := entryOp(entries, mplsfibevents.OpPop); pop != nil {
		t.Fatalf("NP set must not program a pop (no PHP): %+v", pop)
	}
	if push.OutLabels[0] == sr.ExplicitNullV6 {
		t.Fatalf("E clear must not impose the IPv6 Explicit NULL label")
	}
}

// RFC requirement: RFC8666-6-11 positive -- NP clear: the upstream neighbor pops the
// Prefix-SID. This node's own SID label 18009 is programmed as a pop/forward entry and no
// label is imposed at ingress.
// RFC requirement: RFC8666-6-1 negative -- the No-PHP guard is conditional: with NP CLEAR
// the penultimate hop does pop, so the MUST NOT of RFC8666-6-1 is not a blanket refusal
// to ever pop.
// RFC requirement: RFC8666-6-12 negative -- with NP clear the Prefix-SID is NOT kept on
// top of the stack.
func TestRFC8666PHPPopsPrefixSID(t *testing.T) {
	f := newRFC8666Fixture()
	entries := f.installV6(t, sr.PrefixSID{Flags: sr.SIDFlags{}, Index: 9}, f.orig)

	pop := entryOp(entries, mplsfibevents.OpPop)
	if pop == nil || pop.InLabel != 18009 || pop.NextHop != f.nh {
		t.Fatalf("NP clear must program a PHP pop of 18009: %+v", pop)
	}
	if push := entryOp(entries, mplsfibevents.OpPush); push != nil {
		t.Fatalf("PHP must not impose an ingress label: %+v", push)
	}
	if swap := entryOp(entries, mplsfibevents.OpSwap); swap != nil {
		t.Fatalf("PHP must pop, not swap: %+v", swap)
	}
}

// RFC requirement: RFC8666-6-2 positive -- E set: the upstream neighbor replaces the
// Prefix-SID with the Explicit NULL label, which for the IPv6 family is label 2.
// RFC requirement: RFC8666-6-13 positive -- NP and E both set yields the Explicit NULL
// label at ingress and in the transit swap.
func TestRFC8666ExplicitNullReplacesPrefixSID(t *testing.T) {
	f := newRFC8666Fixture()
	entries := f.installV6(t, sr.PrefixSID{Flags: sr.SIDFlags{NP: true, E: true}, Index: 9}, f.orig)

	push := entryOp(entries, mplsfibevents.OpPush)
	if push == nil || push.OutLabels[0] != sr.ExplicitNullV6 {
		t.Fatalf("E set must impose the IPv6 Explicit NULL label 2: %+v", push)
	}
	if sr.ExplicitNullV6 != 2 {
		t.Fatalf("IPv6 Explicit NULL label = %d, want 2", sr.ExplicitNullV6)
	}
	swap := entryOp(entries, mplsfibevents.OpSwap)
	if swap == nil || swap.InLabel != 18009 || swap.OutLabels[0] != sr.ExplicitNullV6 {
		t.Fatalf("E set must swap our label to the Explicit NULL label: %+v", swap)
	}
}

// RFC requirement: RFC8666-6-14 positive -- M set: the NP- and E-Flags are ignored on
// reception. With M=1, NP=0 and E=1 the label is kept (no PHP pop, no Explicit NULL),
// which is exactly what ignoring NP and E produces.
func TestRFC8666MappingServerFlagIgnoresNPAndE(t *testing.T) {
	f := newRFC8666Fixture()
	entries := f.installV6(t, sr.PrefixSID{Flags: sr.SIDFlags{M: true, E: true}, Index: 9}, f.orig)

	push := entryOp(entries, mplsfibevents.OpPush)
	if push == nil || push.OutLabels[0] != 16009 {
		t.Fatalf("M set must keep the Prefix-SID label 16009: %+v", push)
	}
	if push.OutLabels[0] == sr.ExplicitNullV6 {
		t.Fatalf("M set must ignore the E-Flag, not impose Explicit NULL")
	}
	if pop := entryOp(entries, mplsfibevents.OpPop); pop != nil {
		t.Fatalf("M set must ignore the clear NP-Flag, not pop: %+v", pop)
	}
}

// RFC requirement: RFC8666-6-14 negative -- the same NP=0/E=1 advertisement WITHOUT the
// M-Flag honors NP and E: the PHP pop happens. The ignore is conditional on M.
func TestRFC8666WithoutMappingServerFlagNPAndEApply(t *testing.T) {
	f := newRFC8666Fixture()
	entries := f.installV6(t, sr.PrefixSID{Flags: sr.SIDFlags{E: true}, Index: 9}, f.orig)

	if pop := entryOp(entries, mplsfibevents.OpPop); pop == nil || pop.InLabel != 18009 {
		t.Fatalf("without M, NP clear must still pop: %+v", pop)
	}
	if push := entryOp(entries, mplsfibevents.OpPush); push != nil {
		t.Fatalf("without M, NP clear must not impose an ingress label: %+v", push)
	}
}

// RFC requirement: RFC8666-6-5 positive -- a Prefix-SID whose Algorithm the originator DID
// advertise in its SR-Algorithm TLV is honored and installed.
func TestRFC8666PrefixSIDAdvertisedAlgorithmInstalls(t *testing.T) {
	f := newRFC8666Fixture()
	f.algos = map[types.RouterID][]uint8{f.orig: {0, 1}}
	entries := f.installV6(t, sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Algorithm: 0, Index: 9}, f.orig)
	if push := entryOp(entries, mplsfibevents.OpPush); push == nil || push.OutLabels[0] != 16009 {
		t.Fatalf("an advertised algorithm must install: %+v", push)
	}
}

// RFC requirement: RFC8666-6-5 negative -- a Prefix-SID whose Algorithm value the remote
// node never advertised in its SR-Algorithm TLV is ignored: nothing reaches the MPLS FIB.
func TestRFC8666PrefixSIDUnadvertisedAlgorithmIgnored(t *testing.T) {
	f := newRFC8666Fixture()
	f.algos = map[types.RouterID][]uint8{f.orig: {1}} // algorithm 0 is NOT advertised
	entries := f.installV6(t, sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Algorithm: 0, Index: 9}, f.orig)
	if len(entries) != 0 {
		t.Fatalf("a Prefix-SID with an unadvertised algorithm must install nothing: %+v", entries)
	}
}

// Untagged: this exercises only the CONSUMER half of RFC8666-6-7. It hand-injects
// Duplicate:true, so it stays green even if the detector that sets that flag
// (srRemotePrefixSIDsV6, internal/plugins/ospf/sr_reception_v6.go:174) is deleted. The
// requirement is tagged on TestRFC8666DuplicatePrefixSIDsDetectedAndIgnored below, which
// drives the detector from two conflicting LSAs. Kept as a direct regression test that
// installRoutes honors the flag (internal/plugins/ospf/sr_install.go:80-85).
func TestRFC8666DuplicatePrefixSIDsAllIgnored(t *testing.T) {
	f := newRFC8666Fixture()
	bus := &srCaptureBus{}
	inst := newTestInstallerV6(bus)
	routes := []srRoute{{Prefix: f.prefix, Origin: f.orig, NextHops: []srNextHop{{Addr: f.nh, Router: f.orig}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{f.prefix: {
		Originator: f.orig,
		SID:        sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: 9},
		Duplicate:  true,
	}}
	inst.installRoutes(routes, sids, f.caps, f.algos, f.mySRGB)
	if len(bus.entries) != 0 {
		t.Fatalf("duplicate Prefix-SIDs must install nothing: %+v", bus.entries)
	}
}

// TestRFC8666DuplicatePrefixSIDsDetectedAndIgnored drives the conflict detection AND the
// suppression from real input.
//
// VALIDATES: two remote E-Intra-Area-Prefix LSAs binding DIFFERENT SIDs to one prefix are
// installed into the LSDB, srRemotePrefixSIDsV6 marks the prefix Duplicate, and the
// installer emits no mpls-fib entry for it.
//
// PREVENTS: a conflicting pair of Prefix-SIDs being resolved to whichever LSA was read
// first and programmed into the forwarding plane.
//
// Nothing is hand-injected: the Duplicate verdict comes from the detector
// (internal/plugins/ospf/sr_reception_v6.go:163-178). Both originators advertise algorithm
// 0 and an SRGB wide enough for either index, so the duplicate verdict is the only thing
// standing between this prefix and an installed push entry -- remove it and both
// assertions below redden.
//
// RFC requirement: RFC8666-6-7 positive -- when a prefix carries more than one (differing)
// Prefix-SID for the same topology and algorithm, ALL of them are ignored: no forwarding
// entry is installed for that prefix.
func TestRFC8666DuplicatePrefixSIDsDetectedAndIgnored(t *testing.T) {
	eng := newV6RIEngine(t)
	loop := netip.MustParsePrefix("2001:db8::5/128")
	r5 := types.RouterID{5, 5, 5, 5}
	r6 := types.RouterID{6, 6, 6, 6}
	installRemoteV6EPrefix(t, eng, types.BackboneArea, r5, 1, []sr.PrefixSIDConfig{{Prefix: loop, Index: 5}})
	installRemoteV6EPrefix(t, eng, types.BackboneArea, r6, 2, []sr.PrefixSIDConfig{{Prefix: loop, Index: 99}})

	sids := eng.srRemotePrefixSIDsV6()
	entry, ok := sids[loop]
	if !ok || !entry.Duplicate {
		t.Fatalf("two differing Prefix-SIDs for one prefix must be detected as duplicate: %+v", entry)
	}

	srgb := sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 200}})
	bus := &srCaptureBus{}
	inst := newTestInstallerV6(bus)
	routes := []srRoute{{
		Prefix:   loop,
		Origin:   entry.Originator,
		NextHops: []srNextHop{{Addr: netip.MustParseAddr("fe80::2"), Router: entry.Originator}},
	}}
	caps := map[types.RouterID]sr.SRGB{r5: srgb, r6: srgb}
	algos := map[types.RouterID][]uint8{r5: {0}, r6: {0}}
	mySRGB := sr.NewSRGB([]sr.LabelRange{{Base: 18000, Size: 200}})
	inst.installRoutes(routes, sids, caps, algos, mySRGB)
	if len(bus.entries) != 0 {
		t.Fatalf("a duplicated Prefix-SID must reach no forwarding entry: %+v", bus.entries)
	}

	// Control: the identical fixture with the SAME SID advertised twice is NOT a conflict
	// (RFC 8666 §8.2), and it DOES install. This proves the assertion above is carried by the
	// duplicate verdict and not by some other reason the installer had to skip the prefix --
	// caps, algorithms, SRGB range and next-hop are unchanged between the two halves.
	eng2 := newV6RIEngine(t)
	installRemoteV6EPrefix(t, eng2, types.BackboneArea, r5, 1, []sr.PrefixSIDConfig{{Prefix: loop, Index: 5}})
	installRemoteV6EPrefix(t, eng2, types.BackboneArea, r6, 2, []sr.PrefixSIDConfig{{Prefix: loop, Index: 5}})
	agreed := eng2.srRemotePrefixSIDsV6()
	if agreed[loop].Duplicate {
		t.Fatalf("two identical Prefix-SIDs for one prefix are not a conflict: %+v", agreed[loop])
	}
	bus2 := &srCaptureBus{}
	inst2 := newTestInstallerV6(bus2)
	routes2 := []srRoute{{
		Prefix:   loop,
		Origin:   agreed[loop].Originator,
		NextHops: []srNextHop{{Addr: netip.MustParseAddr("fe80::2"), Router: agreed[loop].Originator}},
	}}
	inst2.installRoutes(routes2, agreed, caps, algos, mySRGB)
	if len(bus2.entries) == 0 {
		t.Fatal("control: an unconflicted Prefix-SID must install, otherwise the suppression assertion above proves nothing")
	}
}

// RFC requirement: RFC8666-6-8 positive -- the outgoing label is computed from the flags
// and SRGB of the NEXT-HOP router. Here the next-hop (2.2.2.2) is not the SID originator
// and advertises its own SRGB starting at 20000, so index 9 maps through the NEXT-HOP's
// block to 20009, not through the originator's 16009.
func TestRFC8666OutgoingLabelUsesNextHopRouter(t *testing.T) {
	f := newRFC8666Fixture()
	transit := types.RouterID{2, 2, 2, 2}
	f.caps[transit] = sr.NewSRGB([]sr.LabelRange{{Base: 20000, Size: 100}})
	entries := f.installV6(t, sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: 9}, transit)

	push := entryOp(entries, mplsfibevents.OpPush)
	if push == nil || push.OutLabels[0] != 20009 {
		t.Fatalf("the label must come from the next-hop router's SRGB (20009): %+v", push)
	}
	if push.OutLabels[0] == 16009 {
		t.Fatalf("the originator's SRGB must not source the outgoing label")
	}
}

// RFC requirement: RFC8666-6-8 negative -- the E-/NP-/M-Flags are taken into account only
// when the next-hop router itself advertised the SID. At a transit hop that did not, the
// originator's NP=0 (PHP) is NOT applied: the label is swapped on unconditionally.
func TestRFC8666TransitHopIgnoresOriginatorPHPFlags(t *testing.T) {
	f := newRFC8666Fixture()
	transit := types.RouterID{2, 2, 2, 2}
	f.caps[transit] = sr.NewSRGB([]sr.LabelRange{{Base: 20000, Size: 100}})
	entries := f.installV6(t, sr.PrefixSID{Flags: sr.SIDFlags{}, Index: 9}, transit)

	push := entryOp(entries, mplsfibevents.OpPush)
	if push == nil || push.OutLabels[0] != 20009 {
		t.Fatalf("a transit hop must impose the next-hop label despite NP=0: %+v", push)
	}
	if pop := entryOp(entries, mplsfibevents.OpPop); pop != nil {
		t.Fatalf("a transit hop must not apply the originator's PHP: %+v", pop)
	}
}

// RFC requirement: RFC8666-5-1 positive -- the Range Size advertised in an OSPFv3 Extended
// Prefix Range TLV never exceeds the number of prefixes the Prefix Length can satisfy: the
// ABR propagation advertises exactly one prefix per TLV (Range Size 1), which is within
// the single prefix a /128 can cover.
func TestRFC8666ExtPrefixRangeSizeWithinPrefixLength(t *testing.T) {
	loop := netip.MustParsePrefix("2001:db8:1::9/128")
	body := v6EInterAreaPrefixBody(loop, sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: 9})
	ext, err := ospfv3packet.DecodeExtendedLSABody(body)
	if err != nil || len(ext.TLVs) != 1 || ext.TLVs[0].Type != extTLVExtPrefixRange {
		t.Fatalf("E-Inter-Area-Prefix TLVs = %+v, err %v", ext.TLVs, err)
	}
	rng, err := sr.DecodeExtPrefixRangeValueV6(ext.TLVs[0].Value)
	if err != nil {
		t.Fatalf("Extended Prefix Range decode: %v", err)
	}
	if rng.PrefixLength != 128 || rng.AF != 1 {
		t.Fatalf("propagated range header = %+v", rng)
	}
	// A /128 satisfies exactly one prefix; the advertised Range Size must not exceed it.
	if rng.RangeSize != 1 {
		t.Fatalf("Range Size %d exceeds the 1 prefix a /128 can satisfy", rng.RangeSize)
	}
}

// RFC requirement: RFC8666-7.1-3 positive -- the Adj-SID this router allocates is NOT
// persistent (the SRLB label is freed when the adjacency drops), and the advertised
// P-Flag is correspondingly clear, so the "P set implies persistent" obligation is never
// violated by an advertisement of ours.
// RFC requirement: RFC8666-7.2-2 positive -- the same holds for the LAN Adjacency SID
// form, which shares the flags octet.
func TestRFC8666AdjSIDPersistentFlagClearForNonPersistentAllocation(t *testing.T) {
	bus := &srCaptureBus{}
	m := &srAdjManager{
		alloc:  sr.NewLabelAllocator([]sr.LabelRange{{Base: 40000, Size: 4}}),
		fib:    newSRFIB(bus, mplsSourceOSPFv3SR),
		store:  newSRWireStore(),
		self:   types.RouterID{1, 1, 1, 1},
		labels: map[srAdjKey]srAdjRecord{},
	}
	nbr := types.RouterID{2, 2, 2, 2}
	nh := netip.MustParseAddr("fe80::2")
	for _, lan := range []bool{false, true} {
		iface := "eth0"
		if lan {
			iface = "eth1"
		}
		if !m.neighborFull(iface, nbr, [4]byte(nbr), nh, lan, [4]byte(nbr)) {
			t.Fatalf("lan=%v: Adj-SID not allocated", lan)
		}
		adj, ok := m.adjFor(iface, nbr)
		if !ok {
			t.Fatalf("lan=%v: Adj-SID not stored", lan)
		}
		if adj.Flags.P {
			t.Fatalf("lan=%v: P-Flag set for a non-persistent SRLB allocation: %+v", lan, adj.Flags)
		}
		if adj.IsLAN != lan {
			t.Fatalf("lan=%v: LAN form not recorded", lan)
		}
		// The allocation is non-persistent: losing the adjacency returns the label to the
		// SRLB, so the same value is not guaranteed across a flap.
		before := m.alloc.InUse()
		m.neighborLost(iface, nbr)
		if m.alloc.InUse() != before-1 {
			t.Fatalf("lan=%v: the Adj-SID label must be freed (non-persistent)", lan)
		}
	}
}

// RFC requirement: RFC8666-8.4.1-1 positive -- when the adjacency drops, the Adj-SID
// advertisement is withdrawn from the area: it leaves the store, the E-Router-LSA no
// longer carries it, and its mpls-fib pop entry is removed.
func TestRFC8666AdjSIDWithdrawnWhenAdjacencyDrops(t *testing.T) {
	eng := newV6RIEngine(t)
	bus := &srCaptureBus{}
	self := types.RouterID{1, 1, 1, 1}
	nbr := types.RouterID{2, 2, 2, 2}
	m := &srAdjManager{
		alloc:  sr.NewLabelAllocator([]sr.LabelRange{{Base: 40000, Size: 4}}),
		fib:    newSRFIB(bus, mplsSourceOSPFv3SR),
		store:  newSRWireStore(),
		self:   self,
		labels: map[srAdjKey]srAdjRecord{},
	}
	eng.srAdj = m
	if !m.neighborFull("eth0", nbr, [4]byte(nbr), netip.MustParseAddr("fe80::2"), false, [4]byte(nbr)) {
		t.Fatalf("Adj-SID not allocated at Full")
	}
	ifaces := []ospflsdb.InterfaceInfo{{
		Name: "eth0", NetworkType: ospflsdb.NetworkPointToPoint, InterfaceID: 5, Cost: 10,
		Neighbors: []ospflsdb.NeighborInfo{{RouterID: nbr, State: ospflsdb.NeighborStateFull, InterfaceID: 6}},
	}}
	if _, ok := eng.v6BuildERouterBody(ifaces); !ok {
		t.Fatalf("E-Router-LSA must carry the Adj-SID while the adjacency is up")
	}
	label := m.labels[srAdjKey{iface: "eth0", router: nbr}].label

	m.neighborLost("eth0", nbr)

	if _, ok := m.adjFor("eth0", nbr); ok {
		t.Fatalf("the Adj-SID must be withdrawn when the adjacency drops")
	}
	if _, ok := m.store.adjFor(self, [4]byte(nbr)); ok {
		t.Fatalf("the Adj-SID must leave the origination store")
	}
	if _, ok := eng.v6BuildERouterBody(ifaces); ok {
		t.Fatalf("the E-Router-LSA must no longer advertise the withdrawn Adj-SID")
	}
	var removed bool
	for _, e := range bus.entries {
		if e.Action == mplsfibevents.ActionRemove && e.Op == mplsfibevents.OpPop && e.InLabel == label {
			removed = true
		}
	}
	if !removed {
		t.Fatalf("the Adj-SID pop entry must be withdrawn: %+v", bus.entries)
	}
}
