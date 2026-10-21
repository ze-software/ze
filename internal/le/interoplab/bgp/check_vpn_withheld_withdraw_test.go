// Design: docs/architecture/testing/interop.md -- typed scenario registry.
// Overview: check_vpn_withheld_withdraw.go -- frrCompatibilityWithdrawals.

package bgp

import "testing"

// TestFRRCompatibilityWithdrawals checks the log reader behind the
// bgp-vpn-withheld-withdraw-frr scenario counts only FRR's withdrawal decodes
// from ze whose label field is the Compatibility value 0x800000. Method: feed
// it a withdrawal carrying 524288, one carrying a carried-over label, an
// announcement, and a withdrawal from another peer.
func TestFRRCompatibilityWithdrawals(t *testing.T) {
	const neighbor = "172.30.0.2"
	const prefix = "10.10.0.0/24"
	received := "2026/10/21 22:09:08 BGP: [T1234-56789] 172.30.0.2(Unknown) rcvd "
	for _, tc := range []struct {
		name string
		line string
		want int
	}{
		{"compatibility", received + "UPDATE about RD 65004:40 10.10.0.0/24 label 524288 IPv4 vpn -- withdrawn", 1},
		// FRR 10.3.1's labeled-unicast withdrawal decode carries no label.
		{"no label field", received + "UPDATE about 10.10.0.0/24 IPv4 unicast -- withdrawn", 0},
		{"carried-over label", received + "UPDATE about RD 65004:40 10.10.0.0/24 label 200 IPv4 vpn -- withdrawn", 0},
		{"announcement", received + "RD 65004:40 10.10.0.0/24 label 524288 IPv4 vpn", 0},
		{"other peer", "BGP: [T1234-56789] 172.30.0.22(Unknown) rcvd UPDATE about RD 65004:40 10.10.0.0/24 label 524288 IPv4 vpn -- withdrawn", 0},
	} {
		if got := frrCompatibilityWithdrawals(tc.line+"\n", neighbor, prefix); got != tc.want {
			t.Errorf("%s: counted %d, want %d", tc.name, got, tc.want)
		}
	}
}
