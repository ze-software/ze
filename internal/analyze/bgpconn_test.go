package analyze

import (
	"bytes"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestAnalyzeOpenUsesOnePacker proves AC-1q for ze analyze: bgpBuildOpen
// builds its Optional Parameters through capability.PackOptionalParams and
// its bytes are unchanged.
//
// VALIDATES: the OPEN body equals the bytes the inline packing wrote (one
// type-2 parameter of 18 octets: MP IPv4, MP IPv6, 4-octet AS) and
// round-trips through capability.ParseFromOptionalParams.
// PREVENTS: a byte change in ze analyze's OPEN, or a second packer.
func TestAnalyzeOpenUsesOnePacker(t *testing.T) {
	body, err := bgpBuildOpen(4200000001, 90, net.IP{192, 0, 2, 1})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		4, 0x5B, 0xA0, 0, 90, 192, 0, 2, 1, 20,
		2, 18,
		1, 4, 0, 1, 0, 1,
		1, 4, 0, 2, 0, 1,
		65, 4, 0xFA, 0x56, 0xEA, 0x01,
	}
	if !bytes.Equal(body, want) {
		t.Fatalf("body = % x\nwant   % x", body, want)
	}
	caps, err := capability.ParseFromOptionalParams(body[10:], false)
	if err != nil {
		t.Fatal(err)
	}
	if len(caps) != 3 {
		t.Fatalf("decoded %d capabilities, want 3", len(caps))
	}
}

// analyzePeerOpenBody is an OPEN body (no header) with My AS 23456 and params,
// framed by the one packer, with the RFC 9072 Section 2 envelope written by
// hand when the packer chose the extended form.
func analyzePeerOpenBody(t *testing.T, params ...capability.OptionalParam) []byte {
	t.Helper()
	optional, extended, err := capability.PackOptionalParams(params)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	body := []byte{4, 0x5b, 0xa0, 0, 90, 1, 2, 3, 4}
	if extended {
		body = append(body, 255, 255, byte(len(optional)>>8), byte(len(optional)))
	} else {
		body = append(body, byte(len(optional)))
	}
	return append(body, optional...)
}

// TestAnalyzePeerASReadsExtendedOpen proves ze analyze reads the peer's AS
// through the one Optional Parameters decoder in the framing the OPEN states.
//
// VALIDATES: the Four-octet AS capability is read from a base OPEN and from an
// RFC 9072 extended OPEN whose several type-2 parameters exceed 255 octets; an
// OPEN without it answers My AS; an OPEN the decoder refuses, and one shorter
// than its stated length, are errors rather than the My AS fallback.
// PREVENTS: a one-octet Optional Parameters walk that reads the extended
// envelope as 255 octets of parameters and misses the AS.
func TestAnalyzePeerASReadsExtendedOpen(t *testing.T) {
	asn4 := capability.RawParam(2, []byte{65, 4, 0xfa, 0x56, 0xea, 0x01})
	filler := capability.RawParam(2, append([]byte{0xee, 200}, make([]byte, 200)...))
	for _, tc := range []struct {
		name string
		body []byte
		want uint32
	}{
		{"base form", analyzePeerOpenBody(t, asn4), 4200000001},
		{"RFC 9072 form", analyzePeerOpenBody(t, filler, filler, asn4), 4200000001},
		{"no four-octet AS", analyzePeerOpenBody(t, capability.RawParam(2, []byte{1, 4, 0, 1, 0, 1})), 23456},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bgpPeerAS(tc.body)
			if err != nil {
				t.Fatalf("bgpPeerAS: %v", err)
			}
			if got != tc.want {
				t.Fatalf("AS %d, want %d", got, tc.want)
			}
		})
	}

	short := analyzePeerOpenBody(t, asn4)
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"a non-type-2 parameter", analyzePeerOpenBody(t, asn4, capability.RawParam(7, []byte{1}))},
		{"shorter than its stated length", short[:len(short)-1]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := bgpPeerAS(tc.body); err == nil {
				t.Fatalf("an unreadable OPEN answered AS %d", got)
			}
		})
	}
}
