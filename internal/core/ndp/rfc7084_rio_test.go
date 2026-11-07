package ndp

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

// TestRFC7084L3RouteInformationOption checks the Route Information option a CE
// router sends for its delegated prefix. Method: encode an RA with one route
// of each Length class (/0, /56, /96) and read every field back at the offsets
// RFC 4191 Section 2.3 gives, after the 16-octet header and one PIO.
func TestRFC7084L3RouteInformationOption(t *testing.T) {
	cases := []struct {
		name   string
		prefix netip.Prefix
		length int // octets
	}{
		{"default route has no Prefix field", netip.MustParsePrefix("::/0"), 8},
		{"delegated /56 fits Length 2", netip.MustParsePrefix("2001:db8:aa00::/56"), 16},
		{"a /96 needs Length 3", netip.MustParsePrefix("2001:db8::/96"), 24},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := RAConfig{
				RouterLifetime: 0,
				Prefixes: []PrefixInfo{{
					Prefix: netip.MustParsePrefix("2001:db8:aa01::/64"), OnLink: true, Autonomous: true,
					ValidLifetime: 100, PreferredLifetime: 50,
				}},
				Routes: []RouteInfo{{Prefix: tc.prefix, Lifetime: 3600}},
			}
			buf := make([]byte, RALen(cfg))
			n := BuildRA(buf, 0, cfg)
			if n != raHeaderLen+prefixOptionLen+tc.length {
				t.Fatalf("encoded %d octets, want %d", n, raHeaderLen+prefixOptionLen+tc.length)
			}
			opt := buf[raHeaderLen+prefixOptionLen : n]
			if opt[0] != OptRouteInformation {
				t.Fatalf("type %d, want %d", opt[0], OptRouteInformation)
			}
			if int(opt[1])*8 != tc.length {
				t.Fatalf("Length %d units, want %d octets", opt[1], tc.length)
			}
			if int(opt[2]) != tc.prefix.Bits() {
				t.Fatalf("Prefix Length %d, want %d", opt[2], tc.prefix.Bits())
			}
			if opt[3] != 0 {
				t.Fatalf("Resvd/Prf/Resvd octet %#x, want 0 (medium preference, reserved zero)", opt[3])
			}
			if got := binary.BigEndian.Uint32(opt[4:8]); got != 3600 {
				t.Fatalf("Route Lifetime %d, want 3600", got)
			}
			want := tc.prefix.Addr().As16()
			for i := 8; i < tc.length; i++ {
				if opt[i] != want[i-8] {
					t.Fatalf("Prefix octet %d = %#x, want %#x", i-8, opt[i], want[i-8])
				}
			}
		})
	}
}

// TestRouteInformationMasksHostBits checks that bits past the prefix length go
// out as zero (RFC 4191 Section 2.3), whatever the caller passed.
func TestRouteInformationMasksHostBits(t *testing.T) {
	p := netip.PrefixFrom(netip.MustParseAddr("2001:db8:aa00:ff::1"), 56)
	cfg := RAConfig{Routes: []RouteInfo{{Prefix: p, Lifetime: 1}}}
	buf := make([]byte, RALen(cfg))
	n := BuildRA(buf, 0, cfg)
	opt := buf[raHeaderLen:n]
	if opt[8+7] != 0 {
		t.Fatalf("octet 7 of the Prefix is %#x, want 0: host bits past /56 leaked", opt[8+7])
	}
}
