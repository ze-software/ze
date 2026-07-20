// RFC: rfc/short/rfc8669.md — BGP Prefix-SID attribute (code 40) SR-MPLS TLVs
// Overview: srv6sid.go — ExtractSRv6SIDFull walks the attribute's TLVs at best-path time
//
// The SR-MPLS TLVs of RFC 8669 (Label-Index type 1, Originator SRGB type 3) share the
// attribute with the RFC 9252 SRv6 Service TLVs (types 5 and 6). ze consumes only the
// SRv6 Service TLVs; these tests pin that the SR-MPLS TLVs are stepped over rather than
// misread, on the VPN/EVPN families where the SRv6 extraction runs, and that a repeated
// recognized TLV never displaces the first one.

package pool

import (
	"bytes"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/nlri/nlrisplit"
)

// TestRFC8669NLRILabelIsTheOutboundLabel follows the label of a received labeled-unicast
// NLRI along the path that hands it to the forwarding plane.
//
// VALIDATES: RFC 8669 §4.1 — "the label NLRI defines the outbound label". ze splits the
// label stack off the received NLRI (nlrisplit.ExtractLabels), interns it as side data on
// the peer RIB (InternLabels, from rib_structured.go insertLabeled) and resolves it back
// for the winning path (ResolveLabels, from rib_bestchange.go lookupLabelsForBest), which
// the kernel FIB turns into the MPLS encapsulation pushed toward the next hop
// (internal/plugins/fib/kernel/nexthop_linux.go:75). The label that comes out is the
// label that came in, unmodified.
// PREVENTS: a label mangled by the 20-bit-in-24-bit shift on the way to the data plane,
// which would blackhole every labeled-unicast route while looking healthy in the RIB.
//
// RFC requirement: RFC8669-4.1-8 positive -- the 20-bit label carried in a received labeled-unicast NLRI is the exact value resolved for the best path and handed to the forwarding plane as the outbound label.
func TestRFC8669NLRILabelIsTheOutboundLabel(t *testing.T) {
	label := uint32(24000)

	// RFC 8277 NLRI: totalBits(1) + label(3, S-bit set) + prefix bytes, for 10.0.0.0/24.
	entry := []byte{
		24 + 24,
		byte(label >> 12), byte(label >> 4), byte(label<<4) | 0x01,
		10, 0, 0,
	}

	labels, cidr, err := nlrisplit.ExtractLabels(entry, false)
	if err != nil {
		t.Fatalf("ExtractLabels() error = %v", err)
	}
	if len(labels) != 1 || labels[0] != label {
		t.Fatalf("ExtractLabels() labels = %v, want [%d]", labels, label)
	}
	if want := []byte{24, 10, 0, 0}; !bytes.Equal(cidr, want) {
		t.Fatalf("ExtractLabels() cidr = %v, want %v", cidr, want)
	}

	h := InternLabels(labels)
	if !h.IsValid() {
		t.Fatal("InternLabels() must return a valid handle for a one-label stack")
	}
	defer func() { _ = Labels.Release(h) }()

	got := ResolveLabels(h)
	if len(got) != 1 || got[0] != label {
		t.Errorf("ResolveLabels() = %v, want [%d]: the NLRI label must reach the FIB unchanged", got, label)
	}
}

// rfc8669LabelIndexTLV builds a Label-Index TLV (type 1, length 7): Reserved(1) +
// Flags(2) + Label Index(4).
func rfc8669LabelIndexTLV(index uint32) []byte {
	return []byte{
		1, 0, 7,
		0, 0, 0,
		byte(index >> 24), byte(index >> 16), byte(index >> 8), byte(index),
	}
}

// rfc8669SRGBTLV builds an Originator SRGB TLV (type 3) with one 3-octet base and one
// 3-octet range entry: Flags(2) + base(3) + range(3).
func rfc8669SRGBTLV(base, count uint32) []byte {
	return []byte{
		3, 0, 8,
		0, 0,
		byte(base >> 16), byte(base >> 8), byte(base),
		byte(count >> 16), byte(count >> 8), byte(count),
	}
}

// TestRFC8669LabelIndexIgnoredOnNonLabeledUnicastFamily feeds the extraction path that
// runs for VPN and EVPN routes a Prefix-SID attribute carrying a Label-Index TLV.
//
// VALIDATES: RFC 8669 §3.1 — the Label-Index TLV is ignored for AFI/SAFI combinations
// other than IPv4/IPv6 Labeled Unicast. ExtractSRv6SIDFull walks past TLV type 1 without
// reading its value, so an attribute carrying only a Label-Index yields nothing, and one
// carrying a Label-Index in front of an SRv6 Service TLV yields exactly the SRv6 SID.
// PREVENTS: a label index leaking into the SRv6 VPN/EVPN forwarding decision, where it
// has no defined meaning.
//
// RFC requirement: RFC8669-3.1-2 positive -- on the VPN/EVPN extraction path a Label-Index TLV contributes nothing: alone it yields no SID, and alongside an SRv6 Service TLV the SRv6 SID is unaffected by it.
func TestRFC8669LabelIndexIgnoredOnNonLabeledUnicastFamily(t *testing.T) {
	if got := ExtractSRv6SID(rfc8669LabelIndexTLV(777)); got.IsValid() {
		t.Errorf("a Label-Index-only Prefix-SID must yield no SID, got %v", got)
	}

	want := netip.MustParseAddr("2001:db8::1")
	ip6 := want.As16()
	value := append(rfc8669LabelIndexTLV(777), buildServiceTLV(5, buildSIDInfoSubTLV(ip6))...)

	r := ExtractSRv6SIDFull(value)
	if r.SID != want {
		t.Errorf("SID = %v, want %v: the Label-Index TLV must be stepped over, not misparsed", r.SID, want)
	}
	if r.HasTranspos {
		t.Error("a Label-Index TLV must not contribute transposition parameters")
	}
}

// TestRFC8669SRGBIgnoredOnNonLabeledUnicastFamily is the Originator SRGB counterpart.
//
// VALIDATES: RFC 8669 §3.2 — the Originator SRGB TLV is ignored for AFI/SAFI combinations
// other than Labeled Unicast: ze has no SRGB consumer, so the TLV is skipped by length
// and the SRv6 SID that follows it is still found.
// PREVENTS: an SRGB base/range being misread as SID bytes when the two TLVs share the
// attribute, which would install a bogus SRv6 SID in the FIB.
//
// RFC requirement: RFC8669-3.2-4 positive -- on the VPN/EVPN extraction path an Originator SRGB TLV contributes nothing: alone it yields no SID, and in front of an SRv6 Service TLV the SRv6 SID is unaffected by it.
func TestRFC8669SRGBIgnoredOnNonLabeledUnicastFamily(t *testing.T) {
	if got := ExtractSRv6SID(rfc8669SRGBTLV(800000, 4096)); got.IsValid() {
		t.Errorf("an SRGB-only Prefix-SID must yield no SID, got %v", got)
	}

	want := netip.MustParseAddr("2001:db8::2")
	ip6 := want.As16()
	value := append(rfc8669SRGBTLV(800000, 4096), buildServiceTLV(5, buildSIDInfoSubTLV(ip6))...)

	if got := ExtractSRv6SID(value); got != want {
		t.Errorf("SID = %v, want %v: the SRGB TLV must be stepped over, not misparsed", got, want)
	}
}

// TestRFC8669DuplicateRecognizedTLVFirstWins puts two well-formed SRv6 L3 Service TLVs,
// each carrying a different SID, in one Prefix-SID attribute.
//
// VALIDATES: RFC 8669 §6 — when a recognized TLV appears more than once, occurrences
// after the first are discarded: ExtractSRv6SIDFull returns as soon as a Service TLV
// yields a SID.
// PREVENTS: a last-wins walk, where appending a second Service TLV would let a peer
// steer traffic to a SID of its choosing after the first one was accepted.
//
// RFC requirement: RFC8669-6-3 positive -- with two recognized Service TLVs present, the SID of the FIRST one is the one returned.
func TestRFC8669DuplicateRecognizedTLVFirstWins(t *testing.T) {
	first := netip.MustParseAddr("2001:db8::1")
	second := netip.MustParseAddr("2001:db8::2")
	f16, s16 := first.As16(), second.As16()

	value := append(buildServiceTLV(5, buildSIDInfoSubTLV(f16)), buildServiceTLV(5, buildSIDInfoSubTLV(s16))...)

	if got := ExtractSRv6SID(value); got != first {
		t.Errorf("ExtractSRv6SID() = %v, want the first TLV's SID %v", got, first)
	}
}

// TestRFC8669DuplicateRecognizedTLVCannotOverrideFirst swaps the two SIDs of the previous
// test so the assertion cannot pass by coincidence of ordering.
//
// RFC requirement: RFC8669-6-3 negative -- the SID of a second recognized Service TLV is never returned; a duplicate cannot override the first occurrence.
func TestRFC8669DuplicateRecognizedTLVCannotOverrideFirst(t *testing.T) {
	first := netip.MustParseAddr("2001:db8::2")
	second := netip.MustParseAddr("2001:db8::1")
	f16, s16 := first.As16(), second.As16()

	value := append(buildServiceTLV(5, buildSIDInfoSubTLV(f16)), buildServiceTLV(5, buildSIDInfoSubTLV(s16))...)

	got := ExtractSRv6SID(value)
	if got == second {
		t.Errorf("the duplicate Service TLV's SID %v must be discarded, not returned", second)
	}
	if got != first {
		t.Errorf("ExtractSRv6SID() = %v, want the first TLV's SID %v", got, first)
	}
}
