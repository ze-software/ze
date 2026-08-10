package evpn

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
)

// VALIDATES: Implemented() names exactly the route types ParseEVPN dispatches to a
// type-specific parser. Every other type falls through to EVPNGeneric, which is the
// definition of unrecognized.
// PREVENTS: Section 5.4's recognized set drifting from the parser. Adding a
// route type to ParseEVPN without adding it here would keep discarding routes ze can now
// read; removing one would relay routes ze can no longer read.
func TestImplementedMatchesParseEVPN(t *testing.T) {
	for rt := range 256 {
		routeType := EVPNRouteType(rt)
		// A body of zero length: ParseEVPN's type dispatch is chosen before the body is
		// read, and a type-specific parser rejecting an empty body still proves the
		// dispatch happened, because EVPNGeneric never errors.
		parsed, _, err := ParseEVPN([]byte{byte(rt), 0x00}, false)

		_, generic := parsed.(*eVPNGeneric)
		dispatched := err != nil || !generic

		if dispatched != routeType.Implemented() {
			t.Errorf("route type %d: ParseEVPN dispatched=%v but Implemented()=%v",
				rt, dispatched, routeType.Implemented())
		}
	}
}

// VALIDATES: RecognizeNLRI reads the route type at the front of a wire NLRI.
// PREVENTS: an implemented type being discarded, which would drop working EVPN routes.
func TestRecognizeNLRIAcceptsImplementedTypes(t *testing.T) {
	for _, rt := range []EVPNRouteType{
		EVPNRouteType1, EVPNRouteType2, EVPNRouteType3, EVPNRouteType4, EVPNRouteType5,
	} {
		if !RecognizeNLRI([]byte{byte(rt), 0x02, 0xaa, 0xbb}, false) {
			t.Errorf("route type %d is implemented and must be recognized", rt)
		}
	}
}

// VALIDATES: a type ze does not implement is not recognized, including type 0 (Reserved).
// PREVENTS: Section 5.4's discard never firing.
func TestRecognizeNLRIRejectsUnimplementedTypes(t *testing.T) {
	for _, rt := range []byte{0, 6, 7, 8, 9, 12, 99, 255} {
		if RecognizeNLRI([]byte{rt, 0x02, 0xaa, 0xbb}, false) {
			t.Errorf("route type %d is not implemented and must not be recognized", rt)
		}
	}
}

// VALIDATES: the 4-byte RFC 7911 path identifier is skipped before the type is read.
// PREVENTS: judging a path id byte as a route type, which would discard nearly every
// ADD-PATH EVPN route.
func TestRecognizeNLRISkipsPathID(t *testing.T) {
	// Path id 0x63 (99) would read as an unimplemented route type if not skipped.
	withPathID := []byte{0x00, 0x00, 0x00, 0x63, byte(EVPNRouteType2), 0x02, 0xaa, 0xbb}
	if !RecognizeNLRI(withPathID, true) {
		t.Fatal("the route type after the path id is implemented and must be recognized")
	}
	if RecognizeNLRI(withPathID, false) {
		t.Fatal("without add-path the leading byte is 0x00, a reserved type; must be refused")
	}
}

// VALIDATES: a slice too short to hold a route type is refused rather than read past.
// PREVENTS: an out-of-range read, and a truncated NLRI being relayed as recognized.
func TestRecognizeNLRIRefusesTruncated(t *testing.T) {
	if RecognizeNLRI(nil, false) {
		t.Error("an empty NLRI carries no route type")
	}
	if RecognizeNLRI([]byte{0x00, 0x00, 0x00}, true) {
		t.Error("a slice shorter than the path id carries no route type")
	}
}

// VALIDATES: the plugin's init() actually installed the recognizer, so RFC 7606 Section 5.4
// binds l2vpn/evpn in a real build.
// PREVENTS: the whole mechanism being present but unwired, which would leave the ledger
// claiming a conformance no running daemon has.
func TestRecognizerIsRegisteredForEVPN(t *testing.T) {
	fn := nlritype.Get(L2VPNEVPN)
	if fn == nil {
		t.Fatal("the evpn plugin must register its Section 5.4 recognizer at init")
	}
	if fn([]byte{byte(EVPNRouteType2), 0x00}, false) == false {
		t.Error("the registered recognizer must accept an implemented type")
	}
	if fn([]byte{99, 0x00}, false) {
		t.Error("the registered recognizer must refuse an unimplemented type")
	}
}
