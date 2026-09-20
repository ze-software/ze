package vpn

import (
	"testing"
)

// The section conf-addpath puts on the wire for ipv4 mpls-vpn, one NLRI:
//
//	00 00 00 0a     Path Identifier 10   (RFC 7911 Section 3)
//	70              112 bits follow: 24 label + 64 RD + 24 prefix
//	04 e3 01        label 20012, stack entry 320193, bottom of stack set
//	00 00 00 64 00 00 00 64   RD type 0, 100:100 (RFC 4364 Section 4.2)
//	0a 00 00        10.0.0.0/24
const (
	vpnAddPathHex = "0000000A" + "70" + "04E301" + "0000006400000064" + "0A0000"
	vpnPlainHex   = "70" + "04E301" + "0000006400000064" + "0A0000"
)

// TestDecodeNLRIHexReadsTheAddPathLayout is the discrimination test for the VPN
// decoder's half of the ADD-PATH chain.
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
//
// Hard-coding the flag false, which is what this decoder did until the flag
// reached it, makes the first octet of the Path Identifier the length octet.
// That reads as a length of zero and the parse fails, so the assertions below
// go RED on the route as well as on the identifier.
//
// VALIDATES: DecodeNLRIHex consumes the Path Identifier when addPath is set,
// publishes it as "path-id", and decodes the NLRI that follows it.
// PREVENTS: an ADD-PATH mpls-vpn route rendering as {"parsed":false,"raw":...}.
func TestDecodeNLRIHexReadsTheAddPathLayout(t *testing.T) {
	decoded, err := DecodeNLRIHex("ipv4/mpls-vpn", vpnAddPathHex, true)
	if err != nil {
		t.Fatalf("decode ADD-PATH mpls-vpn NLRI: %v", err)
	}

	route, isObject := decoded.(map[string]any)
	if !isObject {
		t.Fatalf("want one decoded route, got %T (%v)", decoded, decoded)
	}

	if route["prefix"] != "10.0.0.0/24" {
		t.Errorf("prefix = %v, want 10.0.0.0/24", route["prefix"])
	}
	if route["rd"] != "0:100:100" {
		t.Errorf("rd = %v, want 0:100:100", route["rd"])
	}
	pathID, held := route["path-id"].(uint32)
	if !held {
		t.Fatalf("path-id = %v (%T), want the uint32 10", route["path-id"], route["path-id"])
	}
	if pathID != 10 {
		t.Errorf("path-id = %d, want 10", pathID)
	}
}

// TestDecodeNLRIHexWithoutAddPathPublishesNoPathID holds the other polarity: the
// same NLRI with no Path Identifier in front of it decodes to the same route and
// names no identifier, so the flag is read rather than assumed either way.
func TestDecodeNLRIHexWithoutAddPathPublishesNoPathID(t *testing.T) {
	decoded, err := DecodeNLRIHex("ipv4/mpls-vpn", vpnPlainHex, false)
	if err != nil {
		t.Fatalf("decode mpls-vpn NLRI: %v", err)
	}

	route, isObject := decoded.(map[string]any)
	if !isObject {
		t.Fatalf("want one decoded route, got %T (%v)", decoded, decoded)
	}

	if route["prefix"] != "10.0.0.0/24" {
		t.Errorf("prefix = %v, want 10.0.0.0/24", route["prefix"])
	}
	if _, named := route["path-id"]; named {
		t.Errorf("path-id = %v, want no path-id key when ADD-PATH is not negotiated", route["path-id"])
	}
}

// TestDecodeNLRIHexRefusesATruncatedPathIdentifier holds the fail-closed rule: a
// section that ADD-PATH says carries a 4-octet Path Identifier and that holds
// fewer than four octets is malformed. The decoder says so, and it does not
// answer with a zero identifier a caller cannot tell from a real one.
func TestDecodeNLRIHexRefusesATruncatedPathIdentifier(t *testing.T) {
	decoded, err := DecodeNLRIHex("ipv4/mpls-vpn", "000000", true)
	if err != nil {
		return
	}

	route, isObject := decoded.(map[string]any)
	if !isObject {
		t.Fatalf("want an error or an unparsed route, got %T (%v)", decoded, decoded)
	}
	if route["parsed"] != false {
		t.Errorf("want parsed=false for three octets under an ADD-PATH layout, got %v", route)
	}
	if _, named := route["path-id"]; named {
		t.Errorf("want no path-id from a truncated identifier, got %v", route["path-id"])
	}
}

// vpnZeroPathHex is the same VPN NLRI under a Path Identifier of zero.
//
//	00 00 00 00     Path Identifier 0    (RFC 7911 Section 3)
//	70              112 bits follow: 24 label + 64 RD + 24 prefix
//	04 e3 01        label 20012, stack entry 320193, bottom of stack set
//	00 00 00 64 00 00 00 64   RD type 0, 100:100 (RFC 4364 Section 4.2)
//	0a 00 00        10.0.0.0/24
const vpnZeroPathHex = "00000000" + "70" + "04E301" + "0000006400000064" + "0A0000"

// TestDecodeNLRIHexPublishesAPathIdentifierOfZero pins that identifier zero is
// published like any other identifier.
//
// RFC 7911 Section 3 gives the Path Identifier four octets and reserves no
// value, so zero is an identifier a peer can legitimately send. vpnToJSON
// decided the member from `v.pathID != 0`, so a peer sending zero was published
// as a route that carried none, and a script could not tell the two apart.
//
// VALIDATES: an ADD-PATH VPN route with identifier zero carries "path-id": 0.
// PREVENTS: the zero identifier disappearing from the JSON a script reads.
func TestDecodeNLRIHexPublishesAPathIdentifierOfZero(t *testing.T) {
	decoded, err := DecodeNLRIHex("ipv4/mpls-vpn", vpnZeroPathHex, true)
	if err != nil {
		t.Fatalf("decode ADD-PATH mpls-vpn NLRI: %v", err)
	}

	route, isObject := decoded.(map[string]any)
	if !isObject {
		t.Fatalf("want one decoded route, got %T (%v)", decoded, decoded)
	}
	if route["prefix"] != "10.0.0.0/24" {
		t.Errorf("prefix = %v, want 10.0.0.0/24", route["prefix"])
	}
	pathID, held := route["path-id"].(uint32)
	if !held {
		t.Fatalf("path-id = %v (%T), want the uint32 0", route["path-id"], route["path-id"])
	}
	if pathID != 0 {
		t.Errorf("path-id = %d, want 0", pathID)
	}
}

// TestDecodeNLRIHexWithoutAddPathOmitsTheZeroIdentifier is the other polarity:
// the same payload read without the negotiation carries no identifier at all,
// so no "path-id" member is published.
//
// VALIDATES: the member is absent when the section carried no identifier.
// PREVENTS: a route with no Path Identifier publishing "path-id": 0, which
// would be indistinguishable from a real identifier of zero.
func TestDecodeNLRIHexWithoutAddPathOmitsTheZeroIdentifier(t *testing.T) {
	decoded, err := DecodeNLRIHex("ipv4/mpls-vpn", vpnPlainHex, false)
	if err != nil {
		t.Fatalf("decode mpls-vpn NLRI: %v", err)
	}

	route, isObject := decoded.(map[string]any)
	if !isObject {
		t.Fatalf("want one decoded route, got %T (%v)", decoded, decoded)
	}
	if _, held := route["path-id"]; held {
		t.Errorf("path-id = %v, want no member at all", route["path-id"])
	}
}
