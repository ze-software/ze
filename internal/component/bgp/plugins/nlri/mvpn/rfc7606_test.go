package mvpn

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
)

// VALIDATES: Implemented names exactly the route types String can name. RFC 6514
// Section 4 enumerates 1..5 for A-D routes and 6..7 for C-multicast routes, and
// String renders every other value as "type(N)".
// PREVENTS: Section 5.4's recognized set drifting from the rest of the package.
// Adding a route type to String without adding it here would keep discarding routes
// ze can now name; removing one would relay routes ze can no longer name.
func TestImplementedMatchesRouteTypeNames(t *testing.T) {
	for rt := range 256 {
		routeType := MVPNRouteType(rt)
		named := !strings.HasPrefix(routeType.String(), "type(")
		if named != routeType.Implemented() {
			t.Errorf("route type %d: String names it=%v but Implemented()=%v",
				rt, named, routeType.Implemented())
		}
	}
}

// VALIDATES: RecognizeNLRI accepts every route type RFC 6514 Section 4 defines.
// PREVENTS: an implemented type being discarded, which would drop working MVPN routes.
func TestRecognizeNLRIAcceptsImplementedTypes(t *testing.T) {
	for _, rt := range []MVPNRouteType{
		MVPNIntraASIPMSIAD, MVPNInterASIPMSIAD, MVPNSPMSIAD, MVPNLeafAD,
		MVPNSourceActive, MVPNSharedTreeJoin, MVPNSourceTreeJoin,
	} {
		if !RecognizeNLRI([]byte{byte(rt), 0x02, 0xaa, 0xbb}, false) {
			t.Errorf("route type %d is implemented and must be recognized", rt)
		}
	}
}

// VALIDATES: a type RFC 6514 Section 4 does not define is not recognized, type 0
// (Reserved) included.
// PREVENTS: Section 5.4's discard never firing for MCAST-VPN, the family RFC 7606
// Section 5.4 names first.
//
// RFC requirement: RFC7606-5.4-1 positive -- an MCAST-VPN route type ze does not implement is not recognized, so the Section 5.4 discard removes it.
func TestRecognizeNLRIRejectsUnimplementedTypes(t *testing.T) {
	for _, rt := range []byte{0, 8, 9, 10, 99, 255} {
		if RecognizeNLRI([]byte{rt, 0x02, 0xaa, 0xbb}, false) {
			t.Errorf("route type %d is not defined by RFC 6514 and must not be recognized", rt)
		}
	}
}

// VALIDATES: the 4-byte RFC 7911 path identifier is skipped before the type is read.
// PREVENTS: judging a path id byte as a route type.
func TestRecognizeNLRISkipsPathID(t *testing.T) {
	// Path id 0x63 (99) would read as an undefined route type if not skipped.
	withPathID := []byte{0x00, 0x00, 0x00, 0x63, byte(MVPNSPMSIAD), 0x02, 0xaa, 0xbb}
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

// VALIDATES: the plugin's init actually installed the recognizer for both AFIs, so
// RFC 7606 Section 5.4 binds ipv4/mvpn and ipv6/mvpn in a real build.
// PREVENTS: the whole mechanism being present but unwired, which would leave the
// ledger claiming a conformance no running daemon has.
func TestRecognizerIsRegisteredForBothMVPNFamilies(t *testing.T) {
	for _, fam := range []Family{IPv4MVPN, IPv6MVPN} {
		fn := nlritype.Get(fam)
		if fn == nil {
			t.Fatalf("the mvpn plugin must register its Section 5.4 recognizer for %s", fam)
		}
		if !fn([]byte{byte(MVPNSPMSIAD), 0x00}, false) {
			t.Errorf("%s: the registered recognizer must accept an implemented type", fam)
		}
		if fn([]byte{99, 0x00}, false) {
			t.Errorf("%s: the registered recognizer must refuse an undefined type", fam)
		}
	}
}
