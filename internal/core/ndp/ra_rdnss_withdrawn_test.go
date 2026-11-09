package ndp

import (
	"encoding/hex"
	"net/netip"
	"testing"
)

// TestBuildRARDNSSWithdrawn: resolvers in RDNSSWithdrawn go in a second RDNSS
// option, after the live one, whose Lifetime is 0. The golden bytes are
// derived by hand from RFC 8106 Section 5.1: Type 25, Length 1 + 2n,
// Reserved 0, Lifetime, then each 16-octet address.
//
// VALIDATES: AC-8 and AC-15 (spec-home-cpe-23b) on the wire. RFC 8106
// Section 5.1: "A value of zero means that the RDNSS addresses MUST no
// longer be used."
// PREVENTS: a withdrawn resolver sent with the live lifetime, or merged into
// the live option.
func TestBuildRARDNSSWithdrawn(t *testing.T) {
	cfg := RAConfig{
		RDNSS:          []netip.Addr{netip.MustParseAddr("2001:db8:1::53")},
		RDNSSLifetime:  1800,
		RDNSSWithdrawn: []netip.Addr{netip.MustParseAddr("2001:db8::53")},
	}
	buf := make([]byte, RALen(cfg))
	if n := BuildRA(buf, 0, cfg); n != len(buf) {
		t.Fatalf("BuildRA wrote %d of %d octets", n, len(buf))
	}
	options := hex.EncodeToString(buf[raHeaderLen:])
	want := "1903000000000708" + "20010db8000100000000000000000053" +
		"1903000000000000" + "20010db8000000000000000000000053"
	if options != want {
		t.Fatalf("options\n got %s\nwant %s", options, want)
	}

	cfg.RDNSSWithdrawn = make([]netip.Addr, RDNSSMax+1)
	if n := BuildRA(make([]byte, RALen(cfg)), 0, cfg); n != 0 {
		t.Fatalf("BuildRA encoded %d octets for %d withdrawn resolvers, want refusal", n, RDNSSMax+1)
	}
}
