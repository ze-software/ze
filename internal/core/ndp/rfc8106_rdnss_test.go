// Design: docs/architecture/core-design.md -- shared IPv6 Neighbor Discovery encoding
// Related: rdnss.go -- ParseRDNSS, the received RDNSS option parser

package ndp

import (
	"errors"
	"net/netip"
	"slices"
	"testing"
)

// buildRDNSSOption encodes one RDNSS option with BuildRA's own option writer,
// so the parser is proven against the bytes Ze's sender puts on the wire.
func buildRDNSSOption(t *testing.T, lifetime uint32, servers ...netip.Addr) []byte {
	t.Helper()
	cfg := RAConfig{RDNSS: servers, RDNSSLifetime: lifetime}
	buf := make([]byte, RALen(cfg))
	end := BuildRA(buf, 0, cfg)
	if end != len(buf) {
		t.Fatalf("BuildRA wrote %d octets, want %d", end, len(buf))
	}
	// The RA header is 16 octets; the RDNSS option is the only option.
	return buf[16:end]
}

// TestParseRDNSSOption checks that a well-formed option yields its Lifetime
// and its addresses in option order. Method: round trip through BuildRA.
//
// RFC requirement: RFC8106-5.3.1-1 negative -- a received RDNSS option with a valid Length and unicast addresses is not discarded: its Lifetime and its addresses, in option order, are returned.
func TestParseRDNSSOption(t *testing.T) {
	a := netip.MustParseAddr("2001:db8::53")
	b := netip.MustParseAddr("2001:db8::54")
	got, err := ParseRDNSS(buildRDNSSOption(t, 600, a, b))
	if err != nil {
		t.Fatal(err)
	}
	if got.Lifetime != 600 {
		t.Fatalf("Lifetime %d, want 600", got.Lifetime)
	}
	if !slices.Equal(got.Servers, []netip.Addr{a, b}) {
		t.Fatalf("Servers %v, want [%v %v]", got.Servers, a, b)
	}
	if got.Infinite() {
		t.Fatal("Lifetime 600 reported infinite")
	}
	inf, err := ParseRDNSS(buildRDNSSOption(t, LifetimeInfinity, a))
	if err != nil {
		t.Fatal(err)
	}
	if !inf.Infinite() {
		t.Fatal("Lifetime 0xffffffff not reported infinite")
	}
}

// TestParseRDNSSOptionRefusesBadLength checks RFC 8106 Section 5.3.1: an
// option whose Length is below 3, or whose (Length - 1) is odd, is invalid,
// and the error names the Length so the discard can be logged with it.
// Method: corrupt the Length octet of a valid option, and pad the bytes so
// only the Length check can refuse.
//
// RFC requirement: RFC8106-5.3.1-1 positive --a received RDNSS option whose Length is below 3 or whose (Length - 1) is not a multiple of 2 is refused with an error naming that Length, so no address of it is used.
func TestParseRDNSSOptionRefusesBadLength(t *testing.T) {
	valid := buildRDNSSOption(t, 600,
		netip.MustParseAddr("2001:db8::53"), netip.MustParseAddr("2001:db8::54"))
	for _, length := range []uint8{0, 1, 2, 4} {
		option := slices.Clone(valid)
		option[1] = length
		_, err := ParseRDNSS(option)
		var lengthErr *RDNSSLengthError
		if !errors.As(err, &lengthErr) {
			t.Fatalf("Length %d: error %v, want *RDNSSLengthError", length, err)
		}
		if lengthErr.Length != length {
			t.Fatalf("Length %d: error names Length %d", length, lengthErr.Length)
		}
	}
	// Length 5 with the bytes present is the two-address option itself.
	if _, err := ParseRDNSS(valid); err != nil {
		t.Fatalf("Length 5: %v", err)
	}
}

// TestParseRDNSSOptionRefusesNonUnicast checks the address half of RFC 8106
// Section 5.3.1's validity check: "the addresses should be unicast
// addresses". A multicast or unspecified address makes the option invalid.
func TestParseRDNSSOptionRefusesNonUnicast(t *testing.T) {
	for _, text := range []string{"ff02::1", "::"} {
		option := buildRDNSSOption(t, 600, netip.MustParseAddr("2001:db8::53"), netip.MustParseAddr(text))
		_, err := ParseRDNSS(option)
		if _, ok := errors.AsType[*RDNSSAddressError](err); !ok {
			t.Fatalf("%s: error %v, want *RDNSSAddressError", text, err)
		}
	}
}

// TestParseRDNSSOptionRefusesTruncation checks that an option shorter than
// its Length says, or not an RDNSS option at all, is an error, never a read
// past the slice.
func TestParseRDNSSOptionRefusesTruncation(t *testing.T) {
	valid := buildRDNSSOption(t, 600, netip.MustParseAddr("2001:db8::53"))
	for name, option := range map[string][]byte{
		"empty":      nil,
		"one octet":  valid[:1],
		"short body": valid[:len(valid)-1],
	} {
		if _, err := ParseRDNSS(option); err == nil {
			t.Fatalf("%s: no error", name)
		}
	}
	other := slices.Clone(valid)
	other[0] = 3
	if _, err := ParseRDNSS(other); err == nil {
		t.Fatal("option type 3 parsed as RDNSS")
	}
}

// FuzzParseRDNSS checks that no input panics the parser and that every
// accepted option holds only unicast addresses.
func FuzzParseRDNSS(f *testing.F) {
	f.Add([]byte{OptRDNSS, 3, 0, 0, 0, 0, 2, 88, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x53})
	f.Add([]byte{OptRDNSS, 2, 0, 0, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, option []byte) {
		got, err := ParseRDNSS(option)
		if err != nil {
			return
		}
		for _, server := range got.Servers {
			if server.IsMulticast() || server.IsUnspecified() {
				t.Fatalf("accepted non-unicast %v", server)
			}
		}
	})
}
