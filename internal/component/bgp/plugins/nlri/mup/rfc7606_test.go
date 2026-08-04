package mup

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
)

// mupWireNLRI frames one BGP-MUP NLRI: [arch:1][route-type:2][length:1][body]
// (draft-ietf-bess-mup-safi Section 3.1).
func mupWireNLRI(arch byte, routeType uint16, body ...byte) []byte {
	out := []byte{arch, byte(routeType >> 8), byte(routeType), byte(len(body))}
	return append(out, body...)
}

// VALIDATES: Implemented names exactly the route types String can name, under the
// one architecture type the draft defines. draft-ietf-bess-mup-safi Section 3.1
// enumerates route types 1..4, and String renders every other value as "type(N)".
// PREVENTS: Section 5.4's recognized set drifting from the rest of the package.
func TestImplementedMatchesRouteTypeNames(t *testing.T) {
	for rt := range 256 {
		routeType := MUPRouteType(rt)
		named := !strings.HasPrefix(routeType.String(), "type(")
		if named != routeType.Implemented(MUPArch3GPP5G) {
			t.Errorf("route type %d: String names it=%v but Implemented()=%v",
				rt, named, routeType.Implemented(MUPArch3GPP5G))
		}
	}
}

// VALIDATES: RecognizeNLRI accepts every route type the draft defines under
// architecture type 1.
// PREVENTS: an implemented type being discarded, which would drop working MUP routes.
func TestRecognizeNLRIAcceptsImplementedTypes(t *testing.T) {
	for _, rt := range []MUPRouteType{MUPISD, MUPDSD, MUPT1ST, MUPT2ST} {
		if !RecognizeNLRI(mupWireNLRI(byte(MUPArch3GPP5G), uint16(rt), 0xaa, 0xbb), false) {
			t.Errorf("route type %d is implemented and must be recognized", rt)
		}
	}
}

// VALIDATES: a route type the draft does not define is not recognized, and so is a
// route type carried under an architecture ze does not implement. Section 3.1 says
// the Route Type specific encoding depends on Architecture Type AND Route Type, so
// the pair is what names the NLRI type.
// PREVENTS: Section 5.4's discard never firing for BGP-MUP, and an architecture ze
// cannot decode being relayed because its route-type octets happen to read as 1.
//
// RFC requirement: RFC7606-5.4-1 positive -- a BGP-MUP architecture and route type pair ze does not implement is not recognized, so the Section 5.4 discard removes it.
func TestRecognizeNLRIRejectsUnimplementedTypes(t *testing.T) {
	for _, rt := range []uint16{0, 5, 6, 99, 0xffff} {
		if RecognizeNLRI(mupWireNLRI(byte(MUPArch3GPP5G), rt, 0xaa), false) {
			t.Errorf("route type %d is not defined by the draft and must not be recognized", rt)
		}
	}
	for _, arch := range []byte{0, 2, 3, 255} {
		if RecognizeNLRI(mupWireNLRI(arch, uint16(MUPISD), 0xaa), false) {
			t.Errorf("architecture type %d is not implemented and must not be recognized", arch)
		}
	}
}

// VALIDATES: the 4-byte RFC 7911 path identifier is skipped before the architecture
// and route type are read.
// PREVENTS: judging a path id byte as an architecture type.
func TestRecognizeNLRISkipsPathID(t *testing.T) {
	withPathID := append([]byte{0x00, 0x00, 0x00, 0x63},
		mupWireNLRI(byte(MUPArch3GPP5G), uint16(MUPDSD), 0xaa)...)
	if !RecognizeNLRI(withPathID, true) {
		t.Fatal("the pair after the path id is implemented and must be recognized")
	}
	if RecognizeNLRI(withPathID, false) {
		t.Fatal("without add-path the leading byte is 0x00, not an implemented architecture")
	}
}

// VALIDATES: a slice too short to hold the architecture and route type is refused
// rather than read past.
// PREVENTS: an out-of-range read, and a truncated NLRI being relayed as recognized.
func TestRecognizeNLRIRefusesTruncated(t *testing.T) {
	if RecognizeNLRI(nil, false) {
		t.Error("an empty NLRI carries no type")
	}
	if RecognizeNLRI([]byte{byte(MUPArch3GPP5G), 0x00}, false) {
		t.Error("two bytes cannot hold a one-octet architecture and a two-octet route type")
	}
	if RecognizeNLRI([]byte{0x00, 0x00, 0x00, byte(MUPArch3GPP5G), 0x00}, true) {
		t.Error("a slice shorter than the path id plus the type carries no type")
	}
}

// VALIDATES: the plugin's init actually installed the recognizer for both AFIs, so
// RFC 7606 Section 5.4 binds ipv4/mup and ipv6/mup in a real build.
// PREVENTS: the whole mechanism being present but unwired.
func TestRecognizerIsRegisteredForBothMUPFamilies(t *testing.T) {
	for _, fam := range []Family{IPv4MUP, IPv6MUP} {
		fn := nlritype.Get(fam)
		if fn == nil {
			t.Fatalf("the mup plugin must register its Section 5.4 recognizer for %s", fam)
		}
		if !fn(mupWireNLRI(byte(MUPArch3GPP5G), uint16(MUPT1ST), 0x00), false) {
			t.Errorf("%s: the registered recognizer must accept an implemented pair", fam)
		}
		if fn(mupWireNLRI(byte(MUPArch3GPP5G), 99, 0x00), false) {
			t.Errorf("%s: the registered recognizer must refuse an undefined route type", fam)
		}
	}
}
