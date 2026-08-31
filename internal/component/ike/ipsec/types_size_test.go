// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model types
// Related: types.go -- SiteToSitePeer, whose packing this test holds
package ipsec

import (
	"testing"
	"unsafe"
)

// hugeParamThreshold is the gocritic sizeThreshold this repository lints Go against.
// A struct at or above it makes every function taking one by value a lint finding.
const hugeParamThreshold = 288

// VALIDATES: SiteToSitePeer stays under the size at which passing it by value becomes a
// lint finding, so a member added to it does not turn a dozen unrelated functions red.
// PREVENTS: the cost of a new member landing on the twelve call sites that take a peer by
// value. PolicyPriority was written above Mode first, which added eight octets rather than
// filling the existing tail padding, and the struct reached exactly 288.
func TestSiteToSitePeerStaysUnderTheHugeParamThreshold(t *testing.T) {
	size := unsafe.Sizeof(SiteToSitePeer{})
	if size >= hugeParamThreshold {
		t.Errorf("SiteToSitePeer is %d bytes and gocritic flags a value parameter at %d. "+
			"Pack the new member into the tail padding beside Mode and TransportRequired, "+
			"or take the peer by pointer at every call site", size, hugeParamThreshold)
	}
}
