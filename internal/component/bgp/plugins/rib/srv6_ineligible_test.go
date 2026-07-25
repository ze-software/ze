package rib

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// buildOtherAttrsWithPrefixSID constructs OtherAttrs bytes containing a PrefixSID
// attribute (code 40) with the given value. Format: [type(1)][flags(1)][length(2)][value(n)].
func buildOtherAttrsWithPrefixSID(prefixSIDValue []byte) []byte {
	dst := []byte{byte(attribute.AttrPrefixSID), 0xC0, byte(len(prefixSIDValue) >> 8), byte(len(prefixSIDValue))}
	return append(dst, prefixSIDValue...)
}

// buildValidSRv6PrefixSID builds a PrefixSID value with a valid SRv6 L3 Service TLV.
func buildValidSRv6PrefixSID() []byte {
	// SID Info Sub-TLV: Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1) = 21
	sidValue := make([]byte, 21)
	sidValue[0] = 0              // Reserved
	copy(sidValue[1:17], []byte{ // SRv6 SID: 2001:db8::1
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 1,
	})
	subTLV := append([]byte{1, 0, byte(len(sidValue))}, sidValue...)

	// Service TLV value: Reserved(1) + Sub-TLVs
	serviceValue := append([]byte{0}, subTLV...)

	// Service TLV header: type=5 (L3 Service), length
	return append([]byte{5, byte(len(serviceValue) >> 8), byte(len(serviceValue))}, serviceValue...)
}

// buildInvalidSRv6PrefixSID builds a PrefixSID value with an SRv6 L3 Service TLV
// that has a truncated SID Info Sub-TLV (too short for a valid SID).
func buildInvalidSRv6PrefixSID() []byte {
	// SID Info Sub-TLV with length 5 (way too short for Reserved + 16-byte SID)
	subTLV := []byte{1, 0, 5, 0, 0, 0, 0, 0}

	// Service TLV value: Reserved(1) + Sub-TLVs
	serviceValue := append([]byte{0}, subTLV...)

	// Service TLV header: type=5 (L3 Service)
	return append([]byte{5, byte(len(serviceValue) >> 8), byte(len(serviceValue))}, serviceValue...)
}

// buildSRMPLSPrefixSID builds a PrefixSID value with only SR-MPLS TLVs (type 1).
func buildSRMPLSPrefixSID() []byte {
	// Label-Index TLV (type 1, length 7): Reserved(1) + Flags(2) + LabelIndex(4)
	return []byte{1, 0, 7, 0, 0, 0, 0, 0, 0, 42}
}

func makeRouteEntryWithOtherAttrs(otherAttrsData []byte) storage.RouteEntry {
	bundle := storage.NewBundle()
	if len(otherAttrsData) > 0 {
		h, _ := pool.OtherAttrs.Intern(otherAttrsData)
		bundle.OtherAttrs = h
	}
	bundleH := storage.Bundles.Intern(bundle)
	return storage.RouteEntry{Bundle: bundleH}
}

// TestIsSRv6Ineligible_ValidSID checks best-path eligibility of a route whose
// Prefix-SID attribute carries a valid SRv6 Service SID.
//
// VALIDATES: RFC 9252 Section 5 -- isSRv6Ineligible admits a route with an
// extractable SRv6 SID to best-path candidacy.
// PREVENTS: a valid SRv6 VPN/EVPN route being silently dropped from best-path.
//
// RFC requirement: RFC9252-5-1 positive -- a path whose Prefix-SID carries a valid SRv6 SID is eligible for best-path selection.
func TestIsSRv6Ineligible_ValidSID(t *testing.T) {
	otherAttrs := buildOtherAttrsWithPrefixSID(buildValidSRv6PrefixSID())
	entry := makeRouteEntryWithOtherAttrs(otherAttrs)
	defer entry.Release()

	if isSRv6Ineligible(entry) {
		t.Error("route with valid SRv6 SID should be eligible")
	}
}

// RFC requirement: RFC9252-5-1 negative -- a path whose Prefix-SID carries SRv6 Service TLVs but no extractable valid SID is ineligible for best-path selection.
func TestIsSRv6Ineligible_InvalidSID(t *testing.T) {
	otherAttrs := buildOtherAttrsWithPrefixSID(buildInvalidSRv6PrefixSID())
	entry := makeRouteEntryWithOtherAttrs(otherAttrs)
	defer entry.Release()

	if !isSRv6Ineligible(entry) {
		t.Error("route with SRv6 TLVs but no valid SID should be ineligible")
	}
}

func TestIsSRv6Ineligible_NoSRv6TLVs(t *testing.T) {
	otherAttrs := buildOtherAttrsWithPrefixSID(buildSRMPLSPrefixSID())
	entry := makeRouteEntryWithOtherAttrs(otherAttrs)
	defer entry.Release()

	if isSRv6Ineligible(entry) {
		t.Error("route with only SR-MPLS TLVs (no SRv6) should be eligible")
	}
}

func TestIsSRv6Ineligible_NoOtherAttrs(t *testing.T) {
	entry := makeRouteEntryWithOtherAttrs(nil)
	defer entry.Release()

	if isSRv6Ineligible(entry) {
		t.Error("route with no OtherAttrs should be eligible")
	}
}
