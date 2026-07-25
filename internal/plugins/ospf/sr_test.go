// VALIDATES: spec-ospf-ext-5 AC-1/AC-2/AC-3, A-2/A-3 -- SR registers RI capability
// TLV builders (SR-Algorithm/SRGB/SRLB) and the Extended-Prefix Prefix-SID sub-TLV
// builder into the RFC 7770 / RFC 7684 carriers, and emits them only when the
// router has SR configured (self-containment: unconfigured => no TLVs).
// PREVENTS: SR TLVs leaking into non-SR routers' LSAs; wrong TLV type codes;
// origination that does not round-trip through the codec.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func srTestReset(t *testing.T) {
	t.Helper()
	resetRITLVs()
	resetExtSubTLVs()
	srWire.clear()
	if err := srRegisterWire(); err != nil {
		t.Fatalf("srRegisterWire: %v", err)
	}
	t.Cleanup(func() {
		resetRITLVs()
		resetExtSubTLVs()
		srWire.clear()
	})
}

// srEncodeRIBody builds an RI LSA body from the SR RI capability builders for the
// given router, mirroring how the RI originator assembles the LSA.
func srEncodeRIBody(t *testing.T, router types.RouterID) []byte {
	t.Helper()
	tlvs := srBuildAlgorithm(router)
	tlvs = append(tlvs, srBuildSRGB(router)...)
	tlvs = append(tlvs, srBuildSRLB(router)...)
	tlvs = append(tlvs, srBuildSRMS(router)...)
	return packet.EncodeRITLVs(tlvs)
}

func TestSRRegistersRITLVs(t *testing.T) {
	srTestReset(t)
	router := types.RouterID{10, 0, 0, 1}
	srWire.set(router, sr.SRConfig{
		Enabled: true,
		SRGB:    []sr.LabelRange{{Base: 16000, Size: 8000}},
		SRLB:    []sr.LabelRange{{Base: 40000, Size: 1000}},
	})

	// SR-Algorithm builder emits Algorithm 0 (RFC 8665 §3.1).
	algo := srBuildAlgorithm(router)
	if len(algo) != 1 || algo[0].Type != sr.V4TypeSRAlgorithm {
		t.Fatalf("algorithm TLV = %+v", algo)
	}
	algos, err := sr.DecodeAlgorithmValue(algo[0].Value)
	if err != nil || !sr.HasAlgorithm(algos, 0) {
		t.Fatalf("algorithm value = %v,%v", algos, err)
	}

	// SRGB builder emits one range TLV (type 9) that decodes back.
	srgb := srBuildSRGB(router)
	if len(srgb) != 1 || srgb[0].Type != sr.V4TypeSRGB {
		t.Fatalf("SRGB TLV = %+v", srgb)
	}
	r, err := sr.DecodeRangeValue(srgb[0].Value)
	if err != nil || r.Base != 16000 || r.Size != 8000 {
		t.Fatalf("SRGB range = %+v,%v", r, err)
	}

	// SRLB builder emits a range TLV (type 14).
	srlb := srBuildSRLB(router)
	if len(srlb) != 1 || srlb[0].Type != sr.V4TypeSRLB {
		t.Fatalf("SRLB TLV = %+v", srlb)
	}
}

func TestSRNoTLVsWhenUnconfigured(t *testing.T) {
	srTestReset(t)
	// A router with no SR config contributes no TLVs (self-containment).
	router := types.RouterID{10, 0, 0, 2}
	if srBuildAlgorithm(router) != nil || srBuildSRGB(router) != nil || srBuildSRLB(router) != nil {
		t.Fatalf("unconfigured router must contribute no RI TLVs")
	}
}

func TestSRRegistersExtPrefixSubTLV(t *testing.T) {
	srTestReset(t)
	router := types.RouterID{10, 0, 0, 1}
	loop := netip.MustParsePrefix("10.0.0.1/32")
	srWire.set(router, sr.SRConfig{
		Enabled:  true,
		SRGB:     []sr.LabelRange{{Base: 16000, Size: 8000}},
		Prefixes: []sr.PrefixSIDConfig{{Prefix: loop, Index: 1, NodeSID: true, NoPHP: true}},
	})
	ctx := extSubTLVContext{Router: router, Prefix: loop}
	subs := srBuildPrefixSID(ctx)
	if len(subs) != 1 || subs[0].Type != sr.V4TypePrefixSID {
		t.Fatalf("prefix-SID sub-TLV = %+v", subs)
	}
	ps, err := sr.DecodePrefixSIDValue(subs[0].Value)
	if err != nil || ps.Index != 1 || !ps.Flags.NP {
		t.Fatalf("prefix-SID = %+v,%v", ps, err)
	}
	// A different prefix contributes nothing.
	other := extSubTLVContext{Router: router, Prefix: netip.MustParsePrefix("10.0.0.5/32")}
	if srBuildPrefixSID(other) != nil {
		t.Fatalf("non-configured prefix must contribute no Prefix-SID")
	}
}

func TestSRRegistersExtLinkSubTLV(t *testing.T) {
	srTestReset(t)
	router := types.RouterID{10, 0, 0, 1}
	linkData := [4]byte{10, 0, 12, 1}
	srWire.setAdj(router, linkData, sr.AdjSID{Flags: sr.AdjSIDFlags{V: true, L: true}, Weight: 1, Label: 40001})
	ctx := extSubTLVContext{Router: router, LinkData: linkData}
	subs := srBuildAdjSID(ctx)
	if len(subs) != 1 || subs[0].Type != sr.V4TypeAdjSID {
		t.Fatalf("adj-SID sub-TLV = %+v", subs)
	}
	a, err := sr.DecodeAdjSIDValue(subs[0].Value)
	if err != nil || a.Label != 40001 {
		t.Fatalf("adj-SID = %+v,%v", a, err)
	}
}

func TestSRAdjStoreLifecycle(t *testing.T) {
	srTestReset(t)
	router := types.RouterID{10, 0, 0, 1}
	linkData := [4]byte{10, 0, 12, 1}
	if _, ok := srWire.adjFor(router, linkData); ok {
		t.Fatalf("adj store must start empty")
	}
	srWire.setAdj(router, linkData, sr.AdjSID{Flags: sr.AdjSIDFlags{V: true, L: true}, Label: 40001})
	if _, ok := srWire.adjFor(router, linkData); !ok {
		t.Fatalf("adj must be present after setAdj")
	}
	// Clearing the adjacency (drop below 2-Way, RFC 8665 §7.4.1) removes it.
	srWire.clearAdj(router, linkData)
	if _, ok := srWire.adjFor(router, linkData); ok {
		t.Fatalf("adj must be gone after clearAdj")
	}
}

func TestSRReceptionDecodesRemoteCapabilities(t *testing.T) {
	srTestReset(t)
	// Build an RI body carrying SR-Algorithm + one SRGB range, then decode remote
	// capabilities from it (the reception path reads RI LSA bodies from the LSDB).
	router := types.RouterID{10, 0, 0, 3}
	srWire.set(router, sr.SRConfig{Enabled: true, SRGB: []sr.LabelRange{{Base: 18000, Size: 100}}})
	body := srEncodeRIBody(t, router)
	caps := srDecodeRemoteCapabilities(interfaceFamilyIPv4, body)
	if !sr.HasAlgorithm(caps.Algorithms, 0) {
		t.Fatalf("remote algorithm 0 not decoded: %+v", caps.Algorithms)
	}
	if caps.SRGB.TotalSize() != 100 {
		t.Fatalf("remote SRGB total = %d want 100", caps.SRGB.TotalSize())
	}
	if l, ok := caps.SRGB.Label(0); !ok || l != 18000 {
		t.Fatalf("remote SRGB label(0) = %d,%v want 18000", l, ok)
	}
}

func TestSRReceptionSkipsReservedSRGBRange(t *testing.T) {
	// FIX-3 regression (RFC 8665 §10): a received SRGB range whose base is a reserved MPLS
	// label (0..15) is skipped, so its SRGB is empty and no label can be installed from it,
	// while the rest of the RI capabilities (SR-Algorithm 0) still decode. Also observes the
	// ze_ospf_sr_malformed_tlvs_total{tlv="srgb"} counter (via srMetrics.observeMalformed).
	srTestReset(t)
	algo := packet.RITLV{Type: sr.V4TypeSRAlgorithm, Value: sr.EncodeAlgorithmValue([]uint8{0})}
	badSRGB := packet.RITLV{Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(sr.LabelRange{Base: 3, Size: 10})}
	body := packet.EncodeRITLVs([]packet.RITLV{algo, badSRGB})

	caps := srDecodeRemoteCapabilities(interfaceFamilyIPv4, body)
	if !caps.SRGB.Empty() {
		t.Fatalf("a reserved-base SRGB range must be skipped (no label source), got %+v", caps.SRGB.Ranges())
	}
	if !sr.HasAlgorithm(caps.Algorithms, 0) {
		t.Fatalf("a bad range must not discard the rest of the RI capabilities")
	}
}
