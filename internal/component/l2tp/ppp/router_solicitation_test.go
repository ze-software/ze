// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 carrier solicitation admission

package ppp

import (
	"net/netip"
	"testing"
)

// Goal: the carrier accepts reserved and unknown contents while refusing the
// invalid envelopes RFC 4861 Section 6.1.1 names. The checksum is the receive
// transport's responsibility; these tests supply a checksum-validated body.
func TestRouterSolicitationValidity(t *testing.T) {
	base := []byte{133, 0, 0, 0, 0, 0, 0, 0}
	unknown := append(append([]byte(nil), base...), 200, 1, 0, 0, 0, 0, 0, 0)
	unknown[4] = 0xde
	sourceLink := append(append([]byte(nil), base...), 1, 1, 2, 0, 0, 0, 0, 1)
	zeroOption := append(append([]byte(nil), base...), 200, 0)
	truncatedOption := append(append([]byte(nil), base...), 200, 1)
	wrongType := append([]byte(nil), base...)
	wrongType[0] = 134
	wrongCode := append([]byte(nil), base...)
	wrongCode[1] = 1
	global := netip.MustParseAddr("2001:db8::2")
	for _, tc := range []struct {
		name   string
		body   []byte
		source netip.Addr
		hop    int
		want   bool
	}{
		{"link-local", base, netip.MustParseAddr("fe80::2"), 255, true},
		{"global unicast", base, global, 255, true},
		{"unspecified without source option", base, netip.IPv6Unspecified(), 255, true},
		{"reserved and unknown option", unknown, global, 255, true},
		{"source option with unicast source", sourceLink, global, 255, true},
		{"unspecified with source option", sourceLink, netip.IPv6Unspecified(), 255, false},
		{"zero option length", zeroOption, global, 255, false},
		{"truncated option", truncatedOption, global, 255, false},
		{"short body", base[:7], global, 255, false},
		{"wrong type", wrongType, global, 255, false},
		{"wrong code", wrongCode, global, 255, false},
		{"forwarded packet", base, global, 254, false},
		{"missing source", base, netip.Addr{}, 255, false},
		{"IPv4 source", base, netip.MustParseAddr("192.0.2.2"), 255, false},
		{"multicast source", base, netip.MustParseAddr("ff02::2"), 255, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidRouterSolicitation(tc.body, tc.source, tc.hop); got != tc.want {
				t.Fatalf("validity = %v, want %v", got, tc.want)
			}
		})
	}
}
