// Design: docs/architecture/core-design.md -- classify pipeline tests

//go:build linux

package firewallvpp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

func TestClassifyMaskMatchProtocol(t *testing.T) {
	matches := []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}}
	mask, match, err := classifyMaskMatch(matches)
	if err != nil {
		t.Fatal(err)
	}
	if mask[9] != 0xff {
		t.Errorf("protocol mask byte 9 = %#x, want 0xff", mask[9])
	}
	if match[9] != 6 {
		t.Errorf("protocol match byte 9 = %d, want 6 (TCP)", match[9])
	}
}

func TestClassifyMaskMatchSrcAddr(t *testing.T) {
	matches := []firewall.Match{firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("10.0.0.0/24")}}
	mask, match, err := classifyMaskMatch(matches)
	if err != nil {
		t.Fatal(err)
	}
	if mask[12] != 0xff || mask[13] != 0xff || mask[14] != 0xff || mask[15] != 0x00 {
		t.Errorf("source mask bytes 12-15 = %x %x %x %x, want ff ff ff 00", mask[12], mask[13], mask[14], mask[15])
	}
	if match[12] != 10 || match[13] != 0 || match[14] != 0 || match[15] != 0 {
		t.Errorf("source match bytes 12-15 = %d %d %d %d, want 10 0 0 0", match[12], match[13], match[14], match[15])
	}
}

func TestClassifyMaskMatchDstPort(t *testing.T) {
	matches := []firewall.Match{firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}}}
	mask, match, err := classifyMaskMatch(matches)
	if err != nil {
		t.Fatal(err)
	}
	if mask[22] != 0xff || mask[23] != 0xff {
		t.Errorf("dst port mask bytes 22-23 = %x %x, want ff ff", mask[22], mask[23])
	}
	port := uint16(match[22])<<8 | uint16(match[23])
	if port != 80 {
		t.Errorf("dst port match = %d, want 80", port)
	}
}

func TestClassifyMaskMatchCombined(t *testing.T) {
	matches := []firewall.Match{
		firewall.MatchProtocol{Protocol: "tcp"},
		firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("192.168.1.0/24")},
		firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 443, Hi: 443}}},
	}
	mask, match, err := classifyMaskMatch(matches)
	if err != nil {
		t.Fatal(err)
	}
	if mask[9] != 0xff {
		t.Error("protocol mask not set")
	}
	if match[9] != 6 {
		t.Error("protocol not TCP")
	}
	if mask[12] != 0xff || mask[13] != 0xff || mask[14] != 0xff {
		t.Error("source address mask not set for /24")
	}
	port := uint16(match[22])<<8 | uint16(match[23])
	if port != 443 {
		t.Errorf("dst port = %d, want 443", port)
	}
}

func TestClassifyMaskMatchEmpty(t *testing.T) {
	mask, match, err := classifyMaskMatch(nil)
	if err != nil {
		t.Fatal(err)
	}
	allZero := true
	for _, b := range mask {
		if b != 0 {
			allZero = false
			break
		}
	}
	if !allZero {
		t.Error("empty matches should produce all-zero mask")
	}
	for _, b := range match {
		if b != 0 {
			t.Error("empty matches should produce all-zero match")
			break
		}
	}
}

func TestPrefixToMask(t *testing.T) {
	cases := []struct {
		bits int
		want [4]byte
	}{
		{0, [4]byte{0, 0, 0, 0}},
		{8, [4]byte{0xff, 0, 0, 0}},
		{16, [4]byte{0xff, 0xff, 0, 0}},
		{24, [4]byte{0xff, 0xff, 0xff, 0}},
		{32, [4]byte{0xff, 0xff, 0xff, 0xff}},
		{25, [4]byte{0xff, 0xff, 0xff, 0x80}},
	}
	for _, tc := range cases {
		got := prefixToMask(tc.bits)
		if got != tc.want {
			t.Errorf("prefixToMask(%d) = %x, want %x", tc.bits, got, tc.want)
		}
	}
}

func TestLimitToKbps(t *testing.T) {
	cases := []struct {
		name string
		lim  firewall.Limit
		want uint32
	}{
		{"bytes-per-second", firewall.Limit{Rate: 1000, Unit: "second", Dimension: firewall.RateDimensionBytes}, 8},
		{"packets-per-second", firewall.Limit{Rate: 1000, Unit: "second"}, 1000},
		{"per-minute", firewall.Limit{Rate: 6000, Unit: "minute"}, 100},
		{"minimum-1", firewall.Limit{Rate: 1, Unit: "day"}, 1},
		{"bytes-per-minute", firewall.Limit{Rate: 120000, Unit: "minute", Dimension: firewall.RateDimensionBytes}, 16},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := limitToRate(&tc.lim)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("limitToRate = %d, want %d", got, tc.want)
			}
		})
	}
}
