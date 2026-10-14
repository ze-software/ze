// Design: docs/features/srv6.md -- Prefix-SID transmission rules
// Overview: session_prefix_sid.go -- clearTransmittedLabelIndex, the family-gated egress clear

package reactor

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// TestClearTransmittedLabelIndexOnlyOnLabeledUnicast drives the egress clear
// directly over UPDATE bodies whose Prefix-SID holds a 7-octet Label-Index TLV
// with non-zero Reserved and Flags.
//
// VALIDATES: the clear runs on IPv4 and IPv6 labeled unicast, with attribute 40
// before and after MP_REACH_NLRI, and leaves the body octet-equal on IPv4 VPN,
// IPv4 unicast (no MP_REACH_NLRI) and when MP_REACH_NLRI is too short to name a
// family; no case allocates.
// PREVENTS: Ze rewriting a TLV it does not recognize on the family in hand.
//
// RFC requirement: RFC8669-3.1-3 positive -- on IPv4 and IPv6 labeled unicast a transmitted Label-Index TLV leaves with its Reserved octet cleared, whichever of attribute 40 and MP_REACH_NLRI comes first.
// RFC requirement: RFC8669-3.1-5 positive -- on IPv4 and IPv6 labeled unicast a transmitted Label-Index TLV leaves with its Flags cleared.
// RFC requirement: RFC8669-3-1 positive -- on IPv4 VPN, where the Label-Index TLV is unrecognized, the TLV leaves octet-equal to what was received, non-zero Reserved and Flags included.
// RFC requirement: RFC8669-3.1-2 positive -- on IPv4 unicast (no MP_REACH_NLRI) the Label-Index TLV is ignored: the body leaves octet-equal.
func TestClearTransmittedLabelIndexOnlyOnLabeledUnicast(t *testing.T) {
	const (
		origin   = "40010100"
		asPath   = "4002060201" + "0000fde9"
		dirty    = "c0280a" + "010007" + "a5" + "5aff" + "00000064"
		cleared  = "c0280a" + "010007" + "00" + "0000" + "00000064"
		lu4      = "800e10" + "000104" + "04c0000263" + "00" + "30000641c63364"
		lu6      = "800e21" + "000204" + "1020010db8000000000000000000000001" + "00" + "5800064120010db800010000"
		vpn4     = "800e20" + "000180" + "0c0000000000000000c0000263" + "00" + "70000641" + "00010000fde900000001" + "c63364"
		nextHop  = "400304c0000263"
		shortMP  = "800e02" + "0001"
		ipv4NLRI = "18c63364"
	)
	body := func(attrs, nlri string) []byte {
		raw, err := hex.DecodeString(attrs + nlri)
		if err != nil {
			t.Fatal(err)
		}
		n := len(attrs) / 2
		return append([]byte{0, 0, byte(n >> 8), byte(n)}, raw...)
	}
	cases := []struct {
		name       string
		in, wanted []byte
	}{
		{"ipv4 labeled unicast, prefix-sid after", body(origin+asPath+lu4+dirty, ""), body(origin+asPath+lu4+cleared, "")},
		{"ipv4 labeled unicast, prefix-sid first", body(origin+asPath+dirty+lu4, ""), body(origin+asPath+cleared+lu4, "")},
		{"ipv6 labeled unicast", body(origin+asPath+lu6+dirty, ""), body(origin+asPath+lu6+cleared, "")},
		{"ipv4 vpn", body(origin+asPath+dirty+vpn4, ""), body(origin+asPath+dirty+vpn4, "")},
		{"ipv4 unicast", body(origin+asPath+nextHop+dirty, ipv4NLRI), body(origin+asPath+nextHop+dirty, ipv4NLRI)},
		{"mp_reach too short", body(origin+asPath+shortMP+dirty, ""), body(origin+asPath+shortMP+dirty, "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bytes.Clone(tc.in)
			clearTransmittedLabelIndex(got)
			if !bytes.Equal(got, tc.wanted) {
				t.Fatalf("body = %x, want %x", got, tc.wanted)
			}
			scratch := make([]byte, len(tc.in))
			allocs := testing.AllocsPerRun(50, func() {
				copy(scratch, tc.in)
				clearTransmittedLabelIndex(scratch)
			})
			if allocs != 0 {
				t.Fatalf("clearTransmittedLabelIndex allocated %v times", allocs)
			}
		})
	}
}
