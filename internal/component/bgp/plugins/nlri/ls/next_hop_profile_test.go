// Design: docs/architecture/wire/nlri-bgpls.md -- LS next-hop admission
// Related: next_hop_wire_test.go -- literal field and NLRI fixtures

package ls_test

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// TestLSNextHopProfileAdmission checks framed widths independently of address
// roles: LS72 native IPv4 is twelve octets, while its IPv6 pair is forty-eight.
func TestLSNextHopProfileAdmission(t *testing.T) {
	t.Parallel()
	for _, fam := range []family.Family{ls.BGPLSFamily, ls.BGPLSVPNFamily} {
		profile := attribute.MPNextHopProfile(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI))
		cases := []struct {
			name   string
			first  string
			second string
			valid  bool
		}{
			{"native IPv4", "192.0.2.1", "", true},
			{"IPv6 global", "2001:db8::1", "", true},
			{"IPv6 pair", "2001:db8::1", "fe80::2", true},
			{"mapped single", "::ffff:192.0.2.1", "", false},
			{"mapped pair", "::ffff:192.0.2.1", "fe80::2", false},
			{"unspecified pair", "::", "fe80::2", false},
			{"two globals", "2001:db8::1", "2001:db8::2", false},
			{"IPv4 pair", "192.0.2.1", "fe80::2", false},
			{"loopback", "::1", "", false},
		}
		for _, tc := range cases {
			t.Run(fam.String()+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				first := netip.MustParseAddr(tc.first)
				var second netip.Addr
				if tc.second != "" {
					second = netip.MustParseAddr(tc.second)
				}
				err := message.ValidateFamilyNextHop(fam, first, second)
				if (err == nil) != tc.valid {
					t.Fatalf("admission error=%v, valid=%v", err, tc.valid)
				}
				if tc.valid {
					width := profile.NextHopLen(first, second)
					if err := message.ValidateMPNextHop(profile, width+1, first, second); err == nil {
						t.Fatalf("invalid framed width %d accepted", width+1)
					}
				}
			})
		}
	}
}

// TestLSNextHopReceivePairFraming observes the real RFC7606 receiver. The
// negotiated capability-77 pair check must decode LS72's two RD slots, and
// remain inactive without that capability. Width errors still reset first.
func TestLSNextHopReceivePairFraming(t *testing.T) {
	t.Parallel()
	for _, wireCase := range lsNextHopWireCases() {
		if len(wireCase.nextHops) != 2 {
			continue
		}
		t.Run(wireCase.name, func(t *testing.T) {
			t.Parallel()
			for _, negotiated := range []bool{false, true} {
				for _, malformed := range []bool{false, true} {
					field := append([]byte(nil), wireCase.wantNextHop...)
					if malformed {
						// Change only the second address to global IPv6. This
						// offset comes from the independent literal wire oracle.
						field[len(field)-16] = 0x20
					}
					attrs := lsReceiveNextHopAttrs(wireCase, field)
					result := message.ValidateUpdateRFC7606Peer(attrs, false, true, true, nil, negotiated, false)
					want := message.RFC7606ActionNone
					if negotiated {
						if malformed {
							want = message.RFC7606ActionTreatAsWithdraw
						}
					}
					if result.Action != want {
						t.Fatalf("negotiated=%v malformed=%v: action=%v want=%v: %s",
							negotiated, malformed, result.Action, want, result.Description)
					}
				}
			}
			if wireCase.family == ls.BGPLSVPNFamily {
				attrs := lsReceiveNextHopAttrs(wireCase, make([]byte, 40))
				result := message.ValidateUpdateRFC7606Peer(attrs, false, true, true, nil, true, false)
				if result.Action != message.RFC7606ActionSessionReset {
					t.Fatalf("40-octet action=%v, want reset: %s", result.Action, result.Description)
				}
			}
		})
	}
}

// lsReceiveNextHopAttrs supplies mandatory attributes and a complete literal
// MP_REACH, without encoding the next-hop field through the implementation.
func lsReceiveNextHopAttrs(tc lsNextHopWireCase, field []byte) []byte {
	value := make([]byte, 5+len(field)+len(tc.nlri))
	value[0], value[1] = 0x40, 0x04
	value[2], value[3] = byte(tc.family.SAFI), byte(len(field))
	copy(value[4:], field)
	copy(value[5+len(field):], tc.nlri)
	attrs := []byte{
		0x40, 1, 1, 0,
		0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9,
		0x80, 14, byte(len(value)),
	}
	return append(attrs, value...)
}
