// VALIDATES: RFC 8665 (OSPF Extensions for Segment Routing) at the OSPFv2 engine seam: the
// SR-Algorithm / SID-Label-Range / SRLB / SRMS RI capability TLVs are built area-scoped with
// Algorithm 0 always present and one TLV per range, the reception path resolves a repeated
// SR-Algorithm or SRMS Preference TLV to its FIRST occurrence, a Prefix-SID whose algorithm the
// originator never advertised is not installed, duplicate Prefix-SIDs for one prefix are all
// ignored, the next-hop router's NP/E/M flags drive the outgoing label only where that router
// advertised the SID, and an Adj-SID is withdrawn when its adjacency drops.
// PREVENTS: an SR-Algorithm TLV without Algorithm 0, SR capability TLVs leaking into a
// non-area flooding scope, a later TLV instance overriding the first, a Prefix-SID installed
// for an unadvertised algorithm or from a duplicate advertisement, originator flags applied at
// a transit hop, and a stale Adj-SID left advertised after the adjacency is gone.
package ospf

import (
	"bytes"
	"net/netip"
	"testing"

	mplsfibevents "codeberg.org/thomas-mangin/ze/internal/core/mplsfib"
	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/sr"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// srRITypes returns the RI TLV types registered for one flooding scope.
func srRITypes(scope OpaqueScope) []uint16 {
	entries := riTLVBuildersForScope(scope)
	out := make([]uint16, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.tlvType)
	}
	return out
}

// srConfiguredRouter seeds a router's SR config in the origination store.
func srConfiguredRouter(t *testing.T, cfg sr.SRConfig) types.RouterID {
	t.Helper()
	router := types.RouterID{10, 0, 0, 1}
	srWire.set(router, cfg)
	return router
}

// RFC requirement: RFC8665-3.1-1 positive -- the SR-Algorithm TLV this router advertises always
// carries Algorithm 0 (SPF): the builder encodes the literal list {0} (srBuildAlgorithm,
// sr.go:154-160).
// RFC requirement: RFC8665-3.1-2 positive -- the advertised algorithm list is exactly {0}, so
// this router never claims support for Algorithm 1 (Strict SPF); the installer likewise refuses
// any Prefix-SID whose algorithm is not 0 (srInstaller.installRoutes, sr_install.go:89-92), so
// no local policy of ours can alter an Algorithm 1 path we never compute.
func TestRFC8665SRAlgorithmTLVAdvertisesAlgorithmZeroOnly(t *testing.T) {
	srTestReset(t)
	router := srConfiguredRouter(t, sr.SRConfig{
		Enabled: true,
		SRGB:    []sr.LabelRange{{Base: 16000, Size: 100}},
	})
	tlvs := srBuildAlgorithm(router)
	if len(tlvs) != 1 || tlvs[0].Type != sr.V4TypeSRAlgorithm {
		t.Fatalf("SR-Algorithm TLVs = %+v", tlvs)
	}
	algos, err := sr.DecodeAlgorithmValue(tlvs[0].Value)
	if err != nil {
		t.Fatalf("decode SR-Algorithm value: %v", err)
	}
	if !sr.HasAlgorithm(algos, 0) {
		t.Fatalf("advertised algorithms %v must include Algorithm 0", algos)
	}
	if len(algos) != 1 {
		t.Fatalf("advertised algorithms = %v, want exactly {0} (Algorithm 1 never claimed)", algos)
	}
}

// RFC requirement: RFC8665-3.1-1 negative -- a router with no SR configuration advertises no
// SR-Algorithm TLV at all rather than an empty one, so a TLV missing Algorithm 0 is never put on
// the wire (srBuildAlgorithm returns nil for an unknown router, sr.go:155-157). Absence of the
// TLV is the RFC 8665 §3.1 encoding for "not SR capable".
func TestRFC8665NoSRAlgorithmTLVWhenSRUnconfigured(t *testing.T) {
	srTestReset(t)
	router := types.RouterID{10, 0, 0, 7}
	if tlvs := srBuildAlgorithm(router); tlvs != nil {
		t.Fatalf("an SR-unconfigured router must advertise no SR-Algorithm TLV: %+v", tlvs)
	}
}

// RFC requirement: RFC8665-3.1-7 positive -- the SR-Algorithm (8), SID/Label Range (9) and SRLB
// (14) capability TLVs are registered for AREA-scoped flooding, so the RI originator emits them
// only into the area-scoped RI LSA (srRegisterWire, register_sr.go:24-34; the scope filter is
// riTLVBuildersForScope, ri_registry.go:79-90).
func TestRFC8665SRCapabilityTLVsAreAreaScoped(t *testing.T) {
	srTestReset(t)
	got := srRITypes(OpaqueScopeArea)
	want := map[uint16]bool{sr.V4TypeSRAlgorithm: false, sr.V4TypeSRGB: false, sr.V4TypeSRLB: false}
	for _, typ := range got {
		if _, ok := want[typ]; ok {
			want[typ] = true
		}
	}
	for typ, seen := range want {
		if !seen {
			t.Fatalf("RI TLV type %d is not registered at area scope: %v", typ, got)
		}
	}
}

// RFC requirement: RFC8665-3.1-7 negative -- the same capability TLVs are absent from the AS and
// link flooding scopes, so an RI LSA at another scope carries none of them: the required
// area-scoped flooding is enforced by the registration, not merely documented.
func TestRFC8665SRCapabilityTLVsAbsentFromOtherScopes(t *testing.T) {
	srTestReset(t)
	for _, scope := range []OpaqueScope{OpaqueScopeAS, OpaqueScopeLink} {
		for _, typ := range srRITypes(scope) {
			if typ == sr.V4TypeSRAlgorithm || typ == sr.V4TypeSRGB || typ == sr.V4TypeSRLB {
				t.Fatalf("SR capability TLV %d registered at scope %v", typ, scope)
			}
		}
	}
}

// RFC requirement: RFC8665-3.1-3 positive -- an RI LSA carrying a single SR-Algorithm TLV
// resolves to that TLV's algorithm list (srDecodeRemoteCapabilities, sr.go:328-336).
// RFC requirement: RFC8665-3.4-1 positive -- an RI LSA carrying a single SRMS Preference TLV
// resolves to that preference (sr.go:349-358).
func TestRFC8665SingleAlgorithmAndSRMSInstanceUsed(t *testing.T) {
	srTestReset(t)
	body := packet.EncodeRITLVs([]packet.RITLV{
		{Type: sr.V4TypeSRAlgorithm, Value: sr.EncodeAlgorithmValue([]uint8{0})},
		{Type: sr.V4TypeSRMS, Value: sr.EncodeSRMSValue(100)},
	})
	caps := srDecodeRemoteCapabilities(interfaceFamilyIPv4, body)
	if !sr.HasAlgorithm(caps.Algorithms, 0) || len(caps.Algorithms) != 1 {
		t.Fatalf("algorithms = %v, want {0}", caps.Algorithms)
	}
	if !caps.HasSRMS || caps.SRMSPref != 100 {
		t.Fatalf("SRMS = %v/%d, want true/100", caps.HasSRMS, caps.SRMSPref)
	}
}

// RFC requirement: RFC8665-3.1-3 negative -- when a router advertises more than one SR-Algorithm
// TLV in one RI Opaque LSA, the FIRST occurrence is used and every later instance is ignored, so
// a trailing TLV cannot redefine the algorithm set (sr.go:329-333).
// RFC requirement: RFC8665-3.4-1 negative -- a second SRMS Preference TLV in the same RI Opaque
// LSA is likewise ignored in favor of the first (sr.go:350-354).
func TestRFC8665RepeatedAlgorithmAndSRMSInstancesIgnored(t *testing.T) {
	srTestReset(t)
	body := packet.EncodeRITLVs([]packet.RITLV{
		{Type: sr.V4TypeSRAlgorithm, Value: sr.EncodeAlgorithmValue([]uint8{0})},
		{Type: sr.V4TypeSRMS, Value: sr.EncodeSRMSValue(100)},
		// Later instances: both MUST be ignored.
		{Type: sr.V4TypeSRAlgorithm, Value: sr.EncodeAlgorithmValue([]uint8{1})},
		{Type: sr.V4TypeSRMS, Value: sr.EncodeSRMSValue(250)},
	})
	caps := srDecodeRemoteCapabilities(interfaceFamilyIPv4, body)
	if len(caps.Algorithms) != 1 || !sr.HasAlgorithm(caps.Algorithms, 0) {
		t.Fatalf("second SR-Algorithm TLV overrode the first: %v", caps.Algorithms)
	}
	if caps.SRMSPref != 100 {
		t.Fatalf("second SRMS Preference TLV overrode the first: %d", caps.SRMSPref)
	}
}

// RFC requirement: RFC8665-3.2-4 positive -- each configured SID/Label range
// is encoded into its OWN SID/Label Range TLV, never packed together: srBuildSRGB emits one
// packet.RITLV per range (sr.go:167-172) and each value carries exactly one SID/Label sub-TLV.
func TestRFC8665EachRangeInItsOwnTLV(t *testing.T) {
	srTestReset(t)
	router := srConfiguredRouter(t, sr.SRConfig{
		Enabled: true,
		SRGB:    []sr.LabelRange{{Base: 16000, Size: 100}, {Base: 20000, Size: 50}},
	})
	tlvs := srBuildSRGB(router)
	if len(tlvs) != 2 {
		t.Fatalf("two ranges must produce two SID/Label Range TLVs, got %d", len(tlvs))
	}
	wantBases := []uint32{16000, 20000}
	for i, tlv := range tlvs {
		if tlv.Type != sr.V4TypeSRGB {
			t.Fatalf("TLV %d type = %d, want %d", i, tlv.Type, sr.V4TypeSRGB)
		}
		r, err := sr.DecodeRangeValue(tlv.Value)
		if err != nil {
			t.Fatalf("TLV %d does not decode as a single-SID/Label range: %v", i, err)
		}
		if r.Base != wantBases[i] {
			t.Fatalf("TLV %d base = %d, want %d", i, r.Base, wantBases[i])
		}
	}
}

// RFC requirement: RFC8665-3.2-5 positive -- the advertised range order is a
// pure function of the configured SRGB list: srBuildSRGB walks cfg.SRGB in slice order with no
// sort and no map iteration (sr.go:168-172), and parseSegmentRouting rebuilds that list from the
// configuration in document order on every start (sr_config.go:27-30). Rebuilding the config
// from the same source after a restart therefore yields byte-identical, identically ordered
// SID/Label Range TLVs.
func TestRFC8665RangeOrderStableAcrossRestart(t *testing.T) {
	srTestReset(t)
	ranges := []sr.LabelRange{{Base: 16000, Size: 100}, {Base: 20000, Size: 50}, {Base: 30000, Size: 10}}
	router := srConfiguredRouter(t, sr.SRConfig{Enabled: true, SRGB: ranges})
	first := packet.EncodeRITLVs(srBuildSRGB(router))
	// Restart: the store is rebuilt from scratch and the same configuration re-applied.
	srWire.clear()
	srConfiguredRouter(t, sr.SRConfig{Enabled: true, SRGB: ranges})
	second := packet.EncodeRITLVs(srBuildSRGB(router))
	if !bytes.Equal(first, second) {
		t.Fatalf("range TLV order changed across a restart:\n%v\n%v", first, second)
	}
	// Repeated builds within one run are stable too (no map iteration in the builder).
	for range 10 {
		if !bytes.Equal(packet.EncodeRITLVs(srBuildSRGB(router)), first) {
			t.Fatalf("range TLV order is not deterministic within one run")
		}
	}
}

// RFC requirement: RFC8665-6.1-2 positive -- ze allocates every Adj-SID
// dynamically from the SRLB at the moment the adjacency comes up and frees it when the adjacency
// goes down, so it never advertises the P-Flag: the allocated Adj-SID sets only V and L
// (srAdjManager.neighborFull, sr_adjsid.go:62-69) and the flag encoder writes P only when asked
// (AdjSIDFlags.toByte, sr/codec.go:128-131). A persistence claim ze cannot honor is therefore
// never made.
// RFC requirement: RFC8665-6.2-1 positive -- the LAN Adjacency SID is
// allocated by the same code path with lan=true, so it too is advertised with the P-Flag clear.
func TestRFC8665AdjSIDNeverClaimsPersistence(t *testing.T) {
	srTestReset(t)
	bus := &srCaptureBus{}
	m, store := newTestAdjManager(bus)
	nbr := types.RouterID{10, 0, 0, 2}
	nh := netip.MustParseAddr("10.0.12.2")
	p2p := [4]byte{10, 0, 12, 1}
	lan := [4]byte{10, 0, 13, 1}
	if !m.neighborFull("eth0", nbr, p2p, nh, false, [4]byte{}) {
		t.Fatalf("neighborFull must allocate a point-to-point Adj-SID")
	}
	if !m.neighborFull("eth1", nbr, lan, nh, true, nbr) {
		t.Fatalf("neighborFull must allocate a LAN Adj-SID")
	}
	for _, linkData := range [][4]byte{p2p, lan} {
		adj, ok := store.adjFor(m.self, linkData)
		if !ok {
			t.Fatalf("no Adj-SID stored for link data %v", linkData)
		}
		if adj.Flags.P {
			t.Fatalf("Adj-SID for %v claims persistence: %+v", linkData, adj.Flags)
		}
		var value []byte
		if adj.IsLAN {
			value = sr.EncodeLANAdjSIDValue(adj)
		} else {
			value = sr.EncodeAdjSIDValue(adj)
		}
		const pFlagMask = 0x08
		if value[0]&pFlagMask != 0 {
			t.Fatalf("encoded Adj-SID flags %#x set the P-Flag", value[0])
		}
	}
}

// RFC requirement: RFC8665-5-4 positive -- a Prefix-SID whose algorithm appears in the
// originator's SR-Algorithm TLV is installed: the installer checks the originator's advertised
// list before programming any label (sr.HasAlgorithm, sr_install.go:93).
// RFC requirement: RFC8665-5-6 positive -- a prefix carrying exactly one Prefix-SID is installed
// (the Duplicate marker is clear, sr_install.go:79-85).
func TestRFC8665PrefixSIDInstalledWhenAlgorithmAdvertised(t *testing.T) {
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nh := netip.MustParseAddr("10.0.0.2")
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{{Addr: nh, Router: orig}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{
		fec: {Originator: orig, SID: sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: 9}},
	}
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	inst.installRoutes(routes, sids, caps, map[types.RouterID][]uint8{orig: {0}}, sr.SRGB{})
	var pushed bool
	for _, e := range bus.entries {
		if e.Op == mplsfibevents.OpPush && e.Action == mplsfibevents.ActionAdd && e.OutLabels[0] == 16009 {
			pushed = true
		}
	}
	if !pushed {
		t.Fatalf("a Prefix-SID for an advertised algorithm must install: %+v", bus.entries)
	}
}

// RFC requirement: RFC8665-5-4 negative -- when the originator's SR-Algorithm TLV does not
// advertise the Prefix-SID's algorithm, the Prefix-SID sub-TLV is ignored and nothing is
// programmed, even though the originator has a usable SRGB (sr_install.go:93-95).
func TestRFC8665PrefixSIDIgnoredWhenAlgorithmNotAdvertised(t *testing.T) {
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{{Addr: netip.MustParseAddr("10.0.0.2"), Router: orig}}}}
	sids := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: orig, SID: sr.PrefixSID{Index: 9}}}
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	// The originator advertises Algorithm 1 only: Algorithm 0 is not in its list.
	inst.installRoutes(routes, sids, caps, map[types.RouterID][]uint8{orig: {1}}, sr.SRGB{})
	if len(bus.entries) != 0 {
		t.Fatalf("a Prefix-SID for an unadvertised algorithm must not install: %+v", bus.entries)
	}
}

// srInstallExtPrefixLSA installs one Extended Prefix Opaque LSA advertising prefix 10.0.0.9/32
// with a Prefix-SID sub-TLV carrying index, as if received from adv.
func srInstallExtPrefixLSA(t *testing.T, eng *engine, adv types.RouterID, opaqueID, index uint32) {
	t.Helper()
	value := sr.EncodePrefixSIDValue(sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: index})
	body := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
		RouteType:     packet.ExtRouteTypeIntraArea,
		PrefixLength:  32,
		AF:            packet.ExtPrefixAFIPv4Unicast,
		AddressPrefix: [4]byte{10, 0, 0, 9},
		SubTLVs:       []packet.ExtSubTLV{{Type: sr.V4TypePrefixSID, Value: value}},
	}}})
	if _, ok := eng.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router:     adv,
		OpaqueType: packet.ExtPrefixOpaqueType,
		OpaqueID:   opaqueID,
		Scope:      types.LSTypeOpaqueArea,
		Area:       types.BackboneArea,
		Options:    types.OptionO,
		Body:       body,
	}); !ok {
		t.Fatalf("installing Extended Prefix Opaque LSA %d failed", opaqueID)
	}
}

// RFC requirement: RFC8665-5-6 negative -- when a router advertises more than one Prefix-SID for
// the same prefix, ALL of them are ignored rather than one being picked: the reception path marks
// the prefix Duplicate (srRemotePrefixSIDs, sr_install.go:274-278) and the installer programs
// nothing for a Duplicate entry (sr_install.go:79-85).
func TestRFC8665DuplicatePrefixSIDsAllIgnored(t *testing.T) {
	srTestReset(t)
	eng, _ := newRedistEngine(t, extOrigCfg)
	adv := types.RouterID{10, 0, 0, 9}
	srInstallExtPrefixLSA(t, eng, adv, 1, 9)
	fec := netip.MustParsePrefix("10.0.0.9/32")
	single := eng.srRemotePrefixSIDs()
	if rs, ok := single[fec]; !ok || rs.Duplicate {
		t.Fatalf("one Prefix-SID for %s must not be marked duplicate: %+v", fec, rs)
	}
	// A second Prefix-SID for the same prefix, in another LSA from the same router.
	srInstallExtPrefixLSA(t, eng, adv, 2, 11)
	dup := eng.srRemotePrefixSIDs()
	rs, ok := dup[fec]
	if !ok || !rs.Duplicate {
		t.Fatalf("two Prefix-SIDs for %s must mark the prefix duplicate: %+v", fec, rs)
	}
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	routes := []srRoute{{Prefix: fec, Origin: adv, NextHops: []srNextHop{{Addr: netip.MustParseAddr("10.0.0.2"), Router: adv}}}}
	caps := map[types.RouterID]sr.SRGB{adv: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	inst.installRoutes(routes, dup, caps, map[types.RouterID][]uint8{adv: {0}}, sr.SRGB{})
	if len(bus.entries) != 0 {
		t.Fatalf("duplicate Prefix-SIDs must all be ignored: %+v", bus.entries)
	}
}

// RFC requirement: RFC8665-5-7 positive -- the outgoing label is computed from the NEXT-HOP
// router's advertised state, and that next-hop's E/NP/M flags are applied when the next-hop is
// the router that advertised the SID: with the next-hop equal to the originator and NP clear,
// the penultimate-hop PHP rule fires and no label is pushed (srInstaller.forwarding,
// sr_install.go:158-163). Both next-hops of an ECMP route are evaluated independently
// (sr_install.go:103-111), so the decision does not depend on which one "wins" the best path.
func TestRFC8665NextHopFlagsAppliedWhereSIDAdvertised(t *testing.T) {
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	transit := types.RouterID{10, 0, 0, 2}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nhOrig := netip.MustParseAddr("10.0.9.9")
	nhTransit := netip.MustParseAddr("10.0.0.2")
	sids := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: orig, SID: sr.PrefixSID{Index: 9}}}
	caps := map[types.RouterID]sr.SRGB{
		orig:    sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}}),
		transit: sr.NewSRGB([]sr.LabelRange{{Base: 17000, Size: 100}}),
	}
	algos := map[types.RouterID][]uint8{orig: {0}, transit: {0}}
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{
		{Addr: nhTransit, Router: transit},
		{Addr: nhOrig, Router: orig},
	}}}
	inst.installRoutes(routes, sids, caps, algos, sr.NewSRGB([]sr.LabelRange{{Base: 18000, Size: 100}}))
	for _, e := range bus.entries {
		if e.Op == mplsfibevents.OpPush && e.Action == mplsfibevents.ActionAdd && e.NextHop == nhOrig {
			t.Fatalf("the originator next-hop advertised NP=0, so PHP applies and no label is pushed toward it: %+v", e)
		}
	}
	var swapToOrigin bool
	for _, e := range bus.entries {
		if e.Op == mplsfibevents.OpPop && e.NextHop == nhOrig {
			swapToOrigin = true
		}
	}
	if !swapToOrigin {
		t.Fatalf("the penultimate-hop next-hop must get a pop entry: %+v", bus.entries)
	}
}

// RFC requirement: RFC8665-5-7 negative -- a next-hop router that did NOT advertise the SID has
// no flags of its own to apply, so the originator's NP/E/M flags are not used there: the label is
// taken from the transit next-hop's own SRGB and swapped on unconditionally, with no PHP
// (sr_install.go:160-163). Applying the originator's flags at a transit hop would blackhole the
// packet.
func TestRFC8665OriginatorFlagsNotAppliedAtTransitHop(t *testing.T) {
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	transit := types.RouterID{10, 0, 0, 2}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nhTransit := netip.MustParseAddr("10.0.0.2")
	// NP clear on the originator's Prefix-SID: it MUST NOT cause PHP at the transit hop.
	sids := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: orig, SID: sr.PrefixSID{Index: 9}}}
	caps := map[types.RouterID]sr.SRGB{
		orig:    sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}}),
		transit: sr.NewSRGB([]sr.LabelRange{{Base: 17000, Size: 100}}),
	}
	algos := map[types.RouterID][]uint8{orig: {0}, transit: {0}}
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{{Addr: nhTransit, Router: transit}}}}
	inst.installRoutes(routes, sids, caps, algos, sr.NewSRGB([]sr.LabelRange{{Base: 18000, Size: 100}}))
	var push *mplsfibevents.Entry
	for i := range bus.entries {
		if bus.entries[i].Op == mplsfibevents.OpPush && bus.entries[i].Action == mplsfibevents.ActionAdd {
			push = &bus.entries[i]
		}
	}
	if push == nil || push.OutLabels[0] != 17009 {
		t.Fatalf("transit hop must push the next-hop SRGB label 17009 with no PHP: %+v", push)
	}
}

// RFC requirement: RFC8665-7.4.1-1 positive -- when the adjacency drops below 2-Way the Adj-SID
// advertisement is withdrawn from the area: the store entry the Extended Link LSA reads is
// cleared, the SRLB label freed and the pop entry removed (srAdjManager.neighborLost,
// sr_adjsid.go:92-104).
func TestRFC8665AdjSIDWithdrawnWhenAdjacencyDrops(t *testing.T) {
	bus := &srCaptureBus{}
	m, store := newTestAdjManager(bus)
	nbr := types.RouterID{10, 0, 0, 2}
	linkData := [4]byte{10, 0, 12, 1}
	m.neighborFull("eth0", nbr, linkData, netip.MustParseAddr("10.0.12.2"), false, [4]byte{})
	adj, ok := store.adjFor(m.self, linkData)
	if !ok {
		t.Fatalf("Adj-SID must be advertised while the adjacency is up")
	}
	before := len(bus.entries)

	m.neighborLost("eth0", nbr)

	if _, still := store.adjFor(m.self, linkData); still {
		t.Fatalf("the Adj-SID advertisement must be withdrawn from the area")
	}
	var removed bool
	for _, e := range bus.entries[before:] {
		if e.Action == mplsfibevents.ActionRemove && e.Op == mplsfibevents.OpPop && e.InLabel == adj.Label {
			removed = true
		}
	}
	if !removed {
		t.Fatalf("the Adj-SID forwarding entry must be withdrawn: %+v", bus.entries[before:])
	}
	if m.alloc.InUse() != 0 {
		t.Fatalf("the SRLB label must be freed, InUse=%d", m.alloc.InUse())
	}
}

// RFC requirement: RFC8665-7.4.1-1 negative -- the withdraw is keyed by the exact adjacency
// (interface plus neighbor Router ID): a transition reported for an adjacency that holds no
// Adj-SID withdraws nothing, so a live neighbor's Adj-SID is never torn down by another
// neighbor's state change (the lookup miss returns early, sr_adjsid.go:94-97).
func TestRFC8665AdjSIDWithdrawKeyedByAdjacency(t *testing.T) {
	bus := &srCaptureBus{}
	m, store := newTestAdjManager(bus)
	live := types.RouterID{10, 0, 0, 2}
	other := types.RouterID{10, 0, 0, 3}
	linkData := [4]byte{10, 0, 12, 1}
	m.neighborFull("eth0", live, linkData, netip.MustParseAddr("10.0.12.2"), false, [4]byte{})
	before := len(bus.entries)

	m.neighborLost("eth0", other) // unknown neighbor on the same interface
	m.neighborLost("eth9", live)  // known neighbor on an interface with no Adj-SID

	if _, ok := store.adjFor(m.self, linkData); !ok {
		t.Fatalf("an unrelated adjacency transition must not withdraw a live Adj-SID")
	}
	if len(bus.entries) != before {
		t.Fatalf("an unrelated adjacency transition must emit no forwarding change: %+v", bus.entries[before:])
	}
	if m.alloc.InUse() != 1 {
		t.Fatalf("the live Adj-SID label must stay allocated, InUse=%d", m.alloc.InUse())
	}
}
