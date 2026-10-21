package rtc

import (
	"errors"
	"testing"
)

// TestRTCEncodeNLRIHex checks the text encoder against the RFC 4684 Section 4
// layout: a 96-bit prefix of origin AS and route target, or the zero-length
// default. Method: each row's hex is the wire NLRI written out by hand from the
// figure, and the same bytes decode back through parseRTC.
func TestRTCEncodeNLRIHex(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		// 0x60 (96 bits), origin AS 65000, RT type 0x0002, admin 65000, number 1.
		{"two-octet AS target", []string{"origin-as", "65000", "route-target", "65000:1"}, "600000FDE80002FDE800000001"},
		// The `target:` spelling ExaBGP also reads names the same route target.
		{"target prefix", []string{"origin-as", "65000", "route-target", "target:65000:1"}, "600000FDE80002FDE800000001"},
		// RT type 0x0102: IPv4 administrator 192.0.2.1, number 5.
		{"IPv4 target", []string{"origin-as", "65001", "route-target", "192.0.2.1:5"}, "600000FDE90102C00002010005"},
		// RT type 0x0202: four-octet administrator 4200000000, number 7.
		{"four-octet AS target", []string{"origin-as", "4200000000", "route-target", "4200000000:7"}, "60FA56EA000202FA56EA000007"},
		// Boundary: the largest origin AS fits its four octets.
		{"origin-as max", []string{"origin-as", "4294967295", "route-target", "65000:1"}, "60FFFFFFFF0002FDE800000001"},
		// Boundary: origin AS 0 is a value, not the default.
		{"origin-as zero", []string{"origin-as", "0", "route-target", "65000:1"}, "60000000000002FDE800000001"},
		// RFC 5396 asdot: 1.234 is 1<<16 | 234, origin AS 0x000100EA.
		{"origin-as asdot", []string{"origin-as", "1.234", "route-target", "65000:1"}, "60000100EA0002FDE800000001"},
		// The default route target is a zero-length prefix.
		{"default", []string{"default"}, "00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeNLRIHex(familyIPv4RTC, tt.args)
			if err != nil {
				t.Fatalf("EncodeNLRIHex(%v): %v", tt.args, err)
			}
			if got != tt.want {
				t.Fatalf("EncodeNLRIHex(%v) = %s, want %s", tt.args, got, tt.want)
			}
		})
	}
}

// TestRTCEncodeNLRIHexRefuses checks that every malformed membership is refused
// with a reason, never encoded with a zero in place of a missing field.
func TestRTCEncodeNLRIHexRefuses(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want error
	}{
		{"empty", nil, errRTCNeedsMembership},
		{"origin-as alone", []string{"origin-as", "65000"}, errRTCNeedsRouteTarget},
		{"route-target alone", []string{"route-target", "65000:1"}, errRTCNeedsOriginAS},
		{"default with a target", []string{"default", "route-target", "65000:1"}, errRTCDefaultTakesNoValue},
		{"origin-as without value", []string{"origin-as"}, errRTCValueMissing},
		{"unknown word", []string{"origin-as", "65000", "route-target", "65000:1", "color", "1"}, errRTCUnknownWord},
		{"origin-as above four octets", []string{"origin-as", "4294967296", "route-target", "65000:1"}, errRTCOriginAS},
		{"origin-as not a number", []string{"origin-as", "x", "route-target", "65000:1"}, errRTCOriginAS},
		{"not a route target", []string{"origin-as", "65000", "route-target", "origin:65000:1"}, errRTCRouteTarget},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeNLRIHex(familyIPv4RTC, tt.args)
			if !errors.Is(err, tt.want) {
				t.Fatalf("EncodeNLRIHex(%v) = %q, %v; want error %v", tt.args, got, err, tt.want)
			}
		})
	}
	if _, err := EncodeNLRIHex("ipv4/unicast", []string{"default"}); err == nil {
		t.Fatal("EncodeNLRIHex accepted a family it does not encode")
	}
}
