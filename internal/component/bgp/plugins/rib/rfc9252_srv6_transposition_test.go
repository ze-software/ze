// RFC: rfc/short/rfc9252.md -- Section 3.2.1 transposition, Section 5.1 the VPNv4 label field
// Related: rib_bestchange.go -- srv6SIDFromResult, the reader under test
// Related: rib_structured_test.go -- feedReceived, the ingest entry point these drive

package rib

import (
	"net/netip"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// The SRv6 SID this file transposes, in the parts RFC 8986 Section 3.1 names.
// Locator Block 32 bits, Locator Node 16, Function 16 at bit offset 48,
// Argument 0. The sender moves the whole Function into the VPNv4 label field,
// so the SID it puts in the attribute carries zeros there (RFC 9252 Section
// 3.2.1: "The bits that have been shifted out MUST be set to 0 in the SID
// value") and the peer signals where they went.
const (
	srv6TransposOffset = 48
	srv6TransposLen    = 16
	srv6FunctionBits   = 0xE5A7

	// srv6PartialSID is what the Prefix-SID attribute carries on the wire:
	// the Function part zeroed out.
	srv6PartialSID = "2001:db8:1::"

	// srv6FullSID is the SID the peer actually signaled, Function restored.
	srv6FullSID = "2001:db8:1:e5a7::"
)

// vpnv4SRv6Update builds one VPNv4 UPDATE announcing 10.0.0.0/8 with a
// Prefix-SID attribute whose SRv6 SID has srv6TransposLen bits transposed
// into the NLRI label field. It returns the UPDATE body and the NLRI the RIB
// keys the route by, which is what the reader is handed for the same route.
//
// label is the 20-bit value written into the RFC 8277 label field. The caller
// picks it so a wrong one can be fed deliberately. transposLen is the
// Transposition Length the SID Structure Sub-Sub-TLV declares, so a length
// wider than the 20-bit field can be fed too (RFC 9252 Section 7).
func vpnv4SRv6Update(label uint32, transposLen byte) (body, nlriKey []byte) {
	// SID Structure Sub-Sub-TLV (RFC 9252 Section 3.2.1): type 1, length 6,
	// then LBL, LNL, FL, AL, Transposition Length, Transposition Offset.
	// The whole Function part is what moves, so FL tracks the Transposition
	// Length -- Section 5.1 bounds the length by "the FL" as well as by the
	// width of the label field, and only the second bound is under test here.
	sidStructure := []byte{
		1, 0, 6,
		32, 16, transposLen, 0, transposLen, srv6TransposOffset,
	}

	// SRv6 SID Information Sub-TLV (Section 3.2): Reserved(1) + SID(16) +
	// Flags(1) + Behavior(2) + Reserved(1), then Sub-Sub-TLVs.
	sidInfoValue := []byte{0}
	sidInfoValue = append(sidInfoValue, netip.MustParseAddr(srv6PartialSID).AsSlice()...)
	sidInfoValue = append(sidInfoValue, 0, 0, 0, 0) // Flags, Behavior(2), Reserved
	sidInfoValue = append(sidInfoValue, sidStructure...)
	sidInfo := append([]byte{1, byte(len(sidInfoValue) >> 8), byte(len(sidInfoValue))}, sidInfoValue...)

	// SRv6 L3 Service TLV (Section 3.1): type 5, Reserved(1) + Sub-TLVs.
	serviceValue := append([]byte{0}, sidInfo...)
	prefixSID := append([]byte{5, byte(len(serviceValue) >> 8), byte(len(serviceValue))}, serviceValue...)

	// VPNv4 NLRI (RFC 4364 Section 4.3.4): the one length octet counts the
	// label stack and the Route Distinguisher as well as the prefix, so
	// 24 + 64 + 8 bits for 10.0.0.0/8.
	nlri := []byte{24 + 64 + 8}
	// RFC 8277 Section 2.1: the 20-bit Label Value sits in the high-order
	// bits of the three octets; the bottom-of-stack bit is the lowest.
	nlri = append(nlri,
		byte(label>>12), byte(label>>4), byte(label<<4)|0x01,
		0, 0, 0, 0, 0, 0, 0, 0, // Route Distinguisher 0:0
		0x0a, // 10.0.0.0/8
	)

	// MP_REACH_NLRI (RFC 4760, RFC 4364): AFI 1, SAFI 128, a next hop of an
	// 8-octet zero RD followed by the IPv4 address.
	mpReach := []byte{0x00, 0x01, 0x80, 12,
		0, 0, 0, 0, 0, 0, 0, 0, 0x0a, 0x00, 0x00, 0x01,
		0x00, // Reserved
	}
	mpReach = append(mpReach, nlri...)

	// ORIGIN = IGP, then an empty AS_PATH.
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00}
	attrs = append(attrs, 0x80, 14, byte(len(mpReach)))
	attrs = append(attrs, mpReach...)
	attrs = append(attrs, 0xC0, 40, byte(len(prefixSID)))
	attrs = append(attrs, prefixSID...)

	body = []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))}
	return append(body, attrs...), nlri
}

// TestSRv6TranspositionRestoresFunctionBitsFromNLRILabel drives a real VPNv4
// UPDATE through the RIB ingest path and checks the SID the RIB reports for
// the installed route.
//
// VALIDATES: RFC 9252 Section 3.2.1 -- the receiver puts the SID back together
// from the partial SID in the Prefix-SID attribute and the bits the sender
// transposed into the VPNv4 label field.
// PREVENTS: ze installing the partial SID, which has zeros where the Function
// part belongs and therefore names a different SRv6 endpoint than the peer
// advertised.
func TestSRv6TranspositionRestoresFunctionBitsFromNLRILabel(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	peer := netip.MustParseAddr("192.0.2.1")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}

	// ApplyTransposition reads the transposed bits from the high-order end of
	// the label field, so a 16-bit Function in a 20-bit field sits 4 bits up.
	label := uint32(srv6FunctionBits) << (20 - srv6TransposLen)

	body, nlriKey := vpnv4SRv6Update(label, srv6TransposLen)
	feedReceived(r, peer, ctxID, body)

	got := r.lookupSRv6SIDForBest(fam, nlriKey, false, peer)
	want := netip.MustParseAddr(srv6FullSID)
	if got != want {
		t.Errorf("SRv6 SID = %v, want %v (partial SID on the wire was %s)", got, want, srv6PartialSID)
	}
	if got == netip.MustParseAddr(srv6PartialSID) {
		t.Error("SID was installed untransposed: the Function part is still zero")
	}
}

// TestSRv6TranspositionTracksTheLabelValue feeds a second label through the
// same path so a reader that ignores the label cannot pass both tests.
//
// VALIDATES: RFC 9252 Section 3.2.1 -- the reconstructed SID is a function of
// the label field the peer sent, not a constant.
// PREVENTS: a transposition that appears to work because the fixture's
// expected SID happens to match a hardcoded or zeroed reconstruction.
func TestSRv6TranspositionTracksTheLabelValue(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	peer := netip.MustParseAddr("192.0.2.2")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}

	const otherFunction = 0x1234
	label := uint32(otherFunction) << (20 - srv6TransposLen)

	body, nlriKey := vpnv4SRv6Update(label, srv6TransposLen)
	feedReceived(r, peer, ctxID, body)

	got := r.lookupSRv6SIDForBest(fam, nlriKey, false, peer)
	want := netip.MustParseAddr("2001:db8:1:1234::")
	if got != want {
		t.Errorf("SRv6 SID = %v, want %v", got, want)
	}
}

// TestSRv6TranspositionWiderThanLabelFieldIsIneligible pins the bound RFC 9252
// Section 7 puts on a transposition, for the VPN families Section 5.1 gives a
// 20-bit label field.
//
// VALIDATES: RFC 9252 Section 7 -- "The SRv6 SID value in the SRv6 SID
// Information Sub-TLV is invalid when the SID Structure Sub-Sub-TLV
// transposition length is greater than the number of bits of the label field",
// and such a path "MUST be considered ineligible during the selection of the
// best path".
// PREVENTS: ze reconstructing a SID from bits the label field never held, and
// then selecting that path.
//
// RFC requirement: RFC9252-5-1 negative -- a path left without a valid SRv6
// SID, here by a Transposition Length wider than the label field carrying it,
// is ineligible for best-path selection.
func TestSRv6TranspositionWiderThanLabelFieldIsIneligible(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	peer := netip.MustParseAddr("192.0.2.3")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}

	// 24 bits transposed into a 20-bit field: RFC 9252 Section 5.1 bounds the
	// Transposition Length at 20 for a VPN NLRI.
	const tooWide = 24
	label := uint32(srv6FunctionBits) << 4
	body, nlriKey := vpnv4SRv6Update(label, tooWide)

	feedReceived(r, peer, ctxID, body)

	if sid := r.lookupSRv6SIDForBest(fam, nlriKey, false, peer); sid.IsValid() {
		t.Errorf("SID = %v, want none: a %d-bit transposition does not fit a 20-bit label field", sid, tooWide)
	}

	// Section 7 also requires such a path to be INELIGIBLE for best-path
	// selection, which is a separate change to isSRv6Ineligible and is asserted
	// with it. What this test owns is the SID: there is none to install.
}
