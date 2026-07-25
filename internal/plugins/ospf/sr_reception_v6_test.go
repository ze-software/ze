// VALIDATES: spec-ospf-ext-5 AC-5/AC-6/AC-8/AC-9/AC-11/AC-16/AC-19 (IPv6) -- a received
// IPv6 Prefix-SID (Intra-Area Prefix TLV or Extended Prefix Range TLV) is parsed, keyed by
// prefix, and drives the shared install to the right mpls-fib push/swap label (originator
// SRGB base + index) with the IPv6 Explicit NULL label 2; conflicting duplicates are
// ignored; malformed carriage never panics.
// PREVENTS: a v6 Prefix-SID that never installs, a wrong label from the SRGB math, a panic
// on a hostile Extended LSA, a duplicate installing anyway.
package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
)

func newTestInstallerV6(bus *srCaptureBus) *srInstaller {
	return &srInstaller{fib: newSRFIB(bus, mplsSourceOSPFv3SR), af: interfaceFamilyIPv6, explicitNull: sr.ExplicitNullV6}
}

// v6ReceivedPrefixSIDFromBody decodes the first Prefix-SID carried in an E-Intra-Area-Prefix
// body (origination -> reception round-trip without the LSDB).
func v6ReceivedPrefixSIDFromBody(t *testing.T, body []byte) (netip.Prefix, sr.PrefixSID) {
	t.Helper()
	ext, err := ospfv3packet.DecodeExtendedLSABody(body[eIntraPrefixHeaderLen:])
	if err != nil || len(ext.TLVs) == 0 {
		t.Fatalf("decode E-Intra body: %v", err)
	}
	pfx, ps, ok := v6PrefixSIDFromTLV(ext.TLVs[0])
	if !ok {
		t.Fatalf("no Prefix-SID recovered")
	}
	return pfx, ps
}

func TestOSPFv3PrefixSIDInstallsPush(t *testing.T) {
	orig := types.RouterID{9, 9, 9, 9}
	loop := netip.MustParsePrefix("2001:db8::9/128")
	body := v6EIntraAreaPrefixBody(orig, []sr.PrefixSIDConfig{{Prefix: loop, Index: 9, NoPHP: true}})
	pfx, ps := v6ReceivedPrefixSIDFromBody(t, body)

	bus := &srCaptureBus{}
	inst := newTestInstallerV6(bus)
	nh := netip.MustParseAddr("fe80::2")
	// Next-hop router == originator (penultimate hop): next-hop SRGB is the originator's,
	// label 16009 (RFC 8666 §6).
	routes := []srRoute{{Prefix: pfx, Origin: orig, NextHops: []srNextHop{{Addr: nh, Router: orig}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{pfx: {Originator: orig, SID: ps}}
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	algos := map[types.RouterID][]uint8{orig: {0}}
	mySRGB := sr.NewSRGB([]sr.LabelRange{{Base: 18000, Size: 100}})

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
	// NP set (no PHP) -> keep the label: push 16009 at ingress, swap our 18009 -> 16009 transit.
	if push == nil || push.OutLabels[0] != 16009 || push.NextHop != nh {
		t.Fatalf("ingress push wrong: %+v", push)
	}
	if swap == nil || swap.InLabel != 18009 || swap.OutLabels[0] != 16009 {
		t.Fatalf("transit swap wrong: %+v", swap)
	}
	if bus.entries[0].Source != mplsSourceOSPFv3SR {
		t.Fatalf("v6 SR entries must carry the OSPFv3 SR source tag, got %d", bus.entries[0].Source)
	}
}

func TestOSPFv3PrefixSIDExplicitNull(t *testing.T) {
	orig := types.RouterID{9, 9, 9, 9}
	loop := netip.MustParsePrefix("2001:db8::9/128")
	// NP=1 + E=1 -> Explicit NULL: the IPv6 Explicit NULL label is 2 (RFC 8666 §6).
	body := v6EIntraAreaPrefixBody(orig, []sr.PrefixSIDConfig{{Prefix: loop, Index: 9, NoPHP: true, ExplicitNull: true}})
	pfx, ps := v6ReceivedPrefixSIDFromBody(t, body)
	if !ps.Flags.NP || !ps.Flags.E {
		t.Fatalf("NP/E flags lost in carriage: %+v", ps.Flags)
	}
	bus := &srCaptureBus{}
	inst := newTestInstallerV6(bus)
	routes := []srRoute{{Prefix: pfx, Origin: orig, NextHops: []srNextHop{{Addr: netip.MustParseAddr("fe80::2"), Router: orig}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{pfx: {Originator: orig, SID: ps}}
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	inst.installRoutes(routes, sids, caps, map[types.RouterID][]uint8{orig: {0}}, sr.SRGB{})
	var push *mplsfibevents.Entry
	for i := range bus.entries {
		if bus.entries[i].Op == mplsfibevents.OpPush {
			push = &bus.entries[i]
		}
	}
	if push == nil || push.OutLabels[0] != sr.ExplicitNullV6 {
		t.Fatalf("Explicit-NULL push must impose IPv6 label 2, got %+v", push)
	}
}

func TestOSPFv3PrefixSIDPHP(t *testing.T) {
	orig := types.RouterID{9, 9, 9, 9}
	loop := netip.MustParsePrefix("2001:db8::9/128")
	// NP clear -> PHP: no ingress push (forward as plain IP).
	body := v6EIntraAreaPrefixBody(orig, []sr.PrefixSIDConfig{{Prefix: loop, Index: 9}})
	pfx, ps := v6ReceivedPrefixSIDFromBody(t, body)
	bus := &srCaptureBus{}
	inst := newTestInstallerV6(bus)
	routes := []srRoute{{Prefix: pfx, Origin: orig, NextHops: []srNextHop{{Addr: netip.MustParseAddr("fe80::2"), Router: orig}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{pfx: {Originator: orig, SID: ps}}
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	inst.installRoutes(routes, sids, caps, map[types.RouterID][]uint8{orig: {0}}, sr.SRGB{})
	for i := range bus.entries {
		if bus.entries[i].Op == mplsfibevents.OpPush && bus.entries[i].Action == mplsfibevents.ActionAdd {
			t.Fatalf("PHP (NP=0) must not impose an ingress push: %+v", bus.entries[i])
		}
	}
}

func TestOSPFv3ReceivedRangeTLV(t *testing.T) {
	// AC-19: an Extended Prefix Range TLV (AF=1) is a valid Prefix-SID carriage; the SID is
	// the starting value and the prefix decodes from ((PrefixLength+31)/32) words.
	loop := netip.MustParsePrefix("2001:db8::1/128")
	addr := loop.Addr().As16()
	rangeVal := sr.EncodeExtPrefixRangeValueV6(128, addr[:], 1, sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: 50})
	tlv := ospfv3packet.ExtendedTLV{Type: extTLVExtPrefixRange, Value: rangeVal}
	pfx, ps, ok := v6PrefixSIDFromTLV(tlv)
	if !ok || pfx != loop || ps.Index != 50 {
		t.Fatalf("Range TLV Prefix-SID = %v/%+v ok=%v", pfx, ps, ok)
	}
}

func TestOSPFv3ReceptionDuplicateIgnored(t *testing.T) {
	// AC-11: two routers advertise DIFFERENT Prefix-SIDs for the same prefix -> all ignored.
	eng := newV6RIEngine(t)
	loop := netip.MustParsePrefix("2001:db8::5/128")
	installRemoteV6EPrefix(t, eng, types.BackboneArea, types.RouterID{5, 5, 5, 5}, 1,
		[]sr.PrefixSIDConfig{{Prefix: loop, Index: 5}})
	installRemoteV6EPrefix(t, eng, types.BackboneArea, types.RouterID{6, 6, 6, 6}, 2,
		[]sr.PrefixSIDConfig{{Prefix: loop, Index: 99}})
	entry, ok := eng.srRemotePrefixSIDsV6()[loop]
	if !ok || !entry.Duplicate {
		t.Fatalf("conflicting Prefix-SIDs for one prefix must be marked Duplicate: %+v", entry)
	}
}

// RFC requirement: RFC8666-6-7 negative -- "multiple Prefix-SIDs for the same prefix" means
// CONFLICTING ones: two advertisements binding the SAME SID to one prefix (an ABR
// re-advertising an intra-area Prefix-SID, RFC 8666 §8.2) are not duplicates and are not
// ignored.
func TestOSPFv3ReceptionSameSIDNotDuplicate(t *testing.T) {
	// An ABR re-advertising the SAME SID inter-area is not a conflict (RFC 8666 §8.2).
	eng := newV6RIEngine(t)
	loop := netip.MustParsePrefix("2001:db8::5/128")
	installRemoteV6EPrefix(t, eng, types.BackboneArea, types.RouterID{5, 5, 5, 5}, 1,
		[]sr.PrefixSIDConfig{{Prefix: loop, Index: 5}})
	installRemoteV6EPrefix(t, eng, types.BackboneArea, types.RouterID{6, 6, 6, 6}, 2,
		[]sr.PrefixSIDConfig{{Prefix: loop, Index: 5}})
	if entry := eng.srRemotePrefixSIDsV6()[loop]; entry.Duplicate {
		t.Fatalf("identical re-advertised SID must NOT be a duplicate: %+v", entry)
	}
}

// RFC requirement: RFC8666-11-1 positive -- a malformed Extended-LSA prefix TLV body (a
// truncated value, an out-of-range Prefix Length, a word count that would overrun a
// 128-bit address) is detected at the OSPFv3 reception seam without crashing the routing
// process.
func TestOSPFv3ReceptionMalformedNoPanic(t *testing.T) {
	// AC-16: malformed prefix TLV values and word counts never panic (RFC 8666 §11).
	inputs := [][]byte{nil, {}, {0}, {0, 0, 0, 0, 200}, {0, 0, 0, 0, 128, 0, 0, 0}, make([]byte, 3)}
	for i, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("input %d panicked: %v", i, r)
				}
			}()
			// Assert the REJECTION, not merely the absence of a panic: every input here is
			// shorter than the 8-octet fixed header, or (the fifth) declares a /128 whose
			// 16 address octets are absent. A decoder that stopped bound-checking would
			// return ok=true with a zero prefix and a zero SID, and a panic-only assertion
			// would stay green while that zero-valued SID was installed.
			if pfx, ps, ok := v6PrefixSIDFromPrefixTLV(in); ok {
				t.Fatalf("input %d: a malformed prefix TLV must be rejected, got %v/%+v", i, pfx, ps)
			}
			if _, ok := v6PrefixFromWords(200, in); ok {
				t.Fatalf("input %d: PrefixLength 200 must be rejected", i)
			}
		}()
	}
}
