// VALIDATES: the SR sub-TLV wire callbacks in sr.go -- the RFC 7684 Extended Link
// LAN-Adj-SID builder (srBuildLANAdjSID), the ext-4 Receive validators that count a
// malformed sub-TLV (srReceivePrefixSID/srReceiveAdjSID/srReceiveLANAdjSID), and the
// `show ospf database opaque-area` render callbacks (srRenderPrefixSID/srRenderAdjSID/
// srRenderLANAdjSID via srRenderAdj). Each asserts the DECODED fields of a value built
// through the sr/ codec, not a shape-only check.
// PREVENTS: a builder emitting the wrong sub-TLV type or losing the Neighbor ID; a
// render callback printing the wrong SID field (label vs index) or the wrong algorithm;
// a receive validator failing to count a malformed sub-TLV (RFC 8665 §5 V/L check) or
// panicking on truncated input.
package ospf

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestSRBuildLANAdjSID(t *testing.T) {
	srWire.clear()
	t.Cleanup(srWire.clear)

	router := types.RouterID{10, 0, 0, 1}
	linkData := [4]byte{10, 0, 50, 1}
	lan := sr.AdjSID{
		Flags:      sr.AdjSIDFlags{V: true, L: true},
		Weight:     3,
		Label:      40009,
		IsLabel:    true,
		IsLAN:      true,
		NeighborID: [4]byte{2, 2, 2, 2},
	}
	srWire.setAdj(router, linkData, lan)

	subs := srBuildLANAdjSID(extSubTLVContext{Router: router, LinkData: linkData})
	if len(subs) != 1 || subs[0].Type != sr.V4TypeLANAdjSID {
		t.Fatalf("LAN-Adj-SID sub-TLV = %+v, want one of type %d", subs, sr.V4TypeLANAdjSID)
	}
	got, err := sr.DecodeLANAdjSIDValue(subs[0].Value)
	if err != nil {
		t.Fatalf("decode LAN-Adj-SID value: %v", err)
	}
	if !got.IsLAN || got.NeighborID != ([4]byte{2, 2, 2, 2}) || !got.IsLabel || got.Label != 40009 {
		t.Fatalf("decoded LAN-Adj-SID = %+v, want IsLAN neighbor 2.2.2.2 label 40009", got)
	}

	// A NON-LAN adjacency stored for the same link must NOT be emitted by the LAN builder
	// (it belongs to srBuildAdjSID), so the LAN builder contributes nothing.
	srWire.setAdj(router, linkData, sr.AdjSID{Flags: sr.AdjSIDFlags{V: true, L: true}, Label: 40001, IsLabel: true})
	if got := srBuildLANAdjSID(extSubTLVContext{Router: router, LinkData: linkData}); got != nil {
		t.Fatalf("non-LAN adjacency must not produce a LAN-Adj-SID sub-TLV, got %+v", got)
	}

	// An unknown link contributes nothing.
	if got := srBuildLANAdjSID(extSubTLVContext{Router: router, LinkData: [4]byte{9, 9, 9, 9}}); got != nil {
		t.Fatalf("unknown link must contribute no LAN-Adj-SID, got %+v", got)
	}
}

// srCountingRegistry returns a counting CounterVec for the SR malformed-TLV series so a
// test can observe srReceive* incrementing ze_ospf_sr_malformed_tlvs_total. Every other
// series falls through to the no-op backend.
type srCountingRegistry struct {
	metrics.NopRegistry
	vec *srCountingCounterVec
}

func (r srCountingRegistry) CounterVec(name, help string, labels []string) metrics.CounterVec {
	if name == "ze_ospf_sr_malformed_tlvs_total" {
		return r.vec
	}
	return r.NopRegistry.CounterVec(name, help, labels)
}

type srCountingCounterVec struct{ counts map[string]int }

func (v *srCountingCounterVec) With(labelValues ...string) metrics.Counter {
	return srCountingCounter{v: v, key: strings.Join(labelValues, "/")}
}
func (v *srCountingCounterVec) Delete(...string) bool { return false }

type srCountingCounter struct {
	v   *srCountingCounterVec
	key string
}

func (c srCountingCounter) Inc()      { c.v.counts[c.key]++ }
func (srCountingCounter) Add(float64) {}

func TestSRReceiveSubTLVCountsMalformed(t *testing.T) {
	vec := &srCountingCounterVec{counts: map[string]int{}}
	saved := srMetrics.Load()
	srMetrics.Store(newSRMetrics(srCountingRegistry{vec: vec}))
	t.Cleanup(func() { srMetrics.Store(saved) })

	// Well-formed values (index form, V=0/L=0 is valid per RFC 8665 §5) MUST NOT be counted.
	srReceivePrefixSID(sr.EncodePrefixSIDValue(sr.PrefixSID{Index: 5}))
	srReceiveAdjSID(sr.EncodeAdjSIDValue(sr.AdjSID{Index: 3}))
	srReceiveLANAdjSID(sr.EncodeLANAdjSIDValue(sr.AdjSID{NeighborID: [4]byte{4, 4, 4, 4}, Index: 3}))
	if n := len(vec.counts); n != 0 {
		t.Fatalf("well-formed sub-TLVs must not be counted as malformed, got counts %+v", vec.counts)
	}

	// Truncated values fail the length/V-L check and MUST be counted, without panicking.
	srReceivePrefixSID([]byte{0x00})             // < 4 octets
	srReceiveAdjSID([]byte{0x00, 0x00})          // < 4 octets
	srReceiveLANAdjSID([]byte{0, 0, 0, 0, 0, 0}) // < 8 octets

	if vec.counts["ipv4/prefix-sid"] != 1 {
		t.Fatalf("malformed prefix-sid count = %d, want 1", vec.counts["ipv4/prefix-sid"])
	}
	if vec.counts["ipv4/adj-sid"] != 1 {
		t.Fatalf("malformed adj-sid count = %d, want 1", vec.counts["ipv4/adj-sid"])
	}
	if vec.counts["ipv4/lan-adj-sid"] != 1 {
		t.Fatalf("malformed lan-adj-sid count = %d, want 1", vec.counts["ipv4/lan-adj-sid"])
	}
}

func TestSRRenderPrefixSID(t *testing.T) {
	// Index form (V=0/L=0): renders the index and algorithm.
	idx := srRenderPrefixSID(sr.EncodePrefixSIDValue(sr.PrefixSID{Index: 7, Algorithm: 0}))
	if idx != "Prefix-SID index=7 algo=0" {
		t.Fatalf("index-form render = %q, want %q", idx, "Prefix-SID index=7 algo=0")
	}
	// Label form (V=1/L=1): renders the absolute local label.
	lbl := srRenderPrefixSID(sr.EncodePrefixSIDValue(sr.PrefixSID{Flags: sr.SIDFlags{V: true, L: true}, Label: 16050}))
	if lbl != "Prefix-SID label=16050 algo=0" {
		t.Fatalf("label-form render = %q, want %q", lbl, "Prefix-SID label=16050 algo=0")
	}
	// A malformed value renders to the empty string (skipped in the show output).
	if got := srRenderPrefixSID([]byte{0x00}); got != "" {
		t.Fatalf("malformed prefix-SID render = %q, want empty", got)
	}
}

func TestSRRenderAdjSID(t *testing.T) {
	// Label form.
	lbl := srRenderAdjSID(sr.EncodeAdjSIDValue(sr.AdjSID{Flags: sr.AdjSIDFlags{V: true, L: true}, Label: 24001}))
	if lbl != "Adj-SID label=24001" {
		t.Fatalf("label-form Adj-SID render = %q, want %q", lbl, "Adj-SID label=24001")
	}
	// Index form.
	idx := srRenderAdjSID(sr.EncodeAdjSIDValue(sr.AdjSID{Index: 9}))
	if idx != "Adj-SID index=9" {
		t.Fatalf("index-form Adj-SID render = %q, want %q", idx, "Adj-SID index=9")
	}
	if got := srRenderAdjSID([]byte{0x00}); got != "" {
		t.Fatalf("malformed Adj-SID render = %q, want empty", got)
	}
}

func TestSRRenderLANAdjSID(t *testing.T) {
	lan := sr.AdjSID{Flags: sr.AdjSIDFlags{V: true, L: true}, Label: 24009, IsLabel: true, IsLAN: true, NeighborID: [4]byte{4, 4, 4, 4}}
	got := srRenderLANAdjSID(sr.EncodeLANAdjSIDValue(lan))
	if got != "LAN-Adj-SID label=24009" {
		t.Fatalf("LAN-Adj-SID render = %q, want %q", got, "LAN-Adj-SID label=24009")
	}
	// Too short for the 8-octet LAN header + SID: renders empty.
	if got := srRenderLANAdjSID([]byte{0, 0, 0, 0}); got != "" {
		t.Fatalf("malformed LAN-Adj-SID render = %q, want empty", got)
	}
}
