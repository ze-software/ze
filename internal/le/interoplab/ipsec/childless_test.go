// Design: docs/architecture/testing/interop.md -- childless IKE SA scenario verdicts.
// Related: checkers.go -- both childless checkers read strongSwan's trace.
package ipsec

import "testing"

// TestSwanSentNewChildWithKE holds the responder checker to strongSwan's trace of a
// new PFS Child SA: the payload list is read as a set, a rekey (N(REKEY_SA)) or a
// request without KE does not count, a request strongSwan received does not count as
// one it sent, and the lines read are answered for the error.
func TestSwanSentNewChildWithKE(t *testing.T) {
	cases := []struct {
		name string
		logs string
		want bool
	}{
		{"exact", "13[ENC] generating CREATE_CHILD_SA request 2 [ SA No KE TSi TSr ]\n", true},
		{"notifies around", "13[ENC] generating CREATE_CHILD_SA request 2 [ N(ESP_TFC_PAD_N) SA No KE TSi TSr N(NON_FIRST_FRAG) ]\n", true},
		{"rekey", "13[ENC] generating CREATE_CHILD_SA request 2 [ N(REKEY_SA) SA No KE TSi TSr ]\n", false},
		{"no KE", "13[ENC] generating CREATE_CHILD_SA request 2 [ SA No TSi TSr ]\n", false},
		{"response only", "13[ENC] parsed CREATE_CHILD_SA response 2 [ SA No KE TSi TSr ]\n", false},
		{"received, not sent", "11[ENC] parsed CREATE_CHILD_SA request 6 [ SA No KE TSi TSr ]\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, seen := swanNewChildWithKE(tc.logs, swanCreateChildRequest)
			if got != tc.want {
				t.Fatalf("sent: swanNewChildWithKE = %v, want %v (requests read %q)", got, tc.want, seen)
			}
		})
	}
	if _, seen := swanNewChildWithKE("x generating CREATE_CHILD_SA request 4 [ N(REKEY_SA) SA No TSi TSr ]\n", swanCreateChildRequest); len(seen) != 1 {
		t.Fatalf("requests read = %q, want the one rekey line", seen)
	}
}

// TestSwanReceivedNewChildWithKE holds the initiator checker to strongSwan's trace of
// the CREATE_CHILD_SA request it received from Ze: a new Child SA with KE counts, one
// without KE does not, and a request strongSwan sent does not count as one it received.
func TestSwanReceivedNewChildWithKE(t *testing.T) {
	received := []struct {
		name string
		logs string
		want bool
	}{
		{"received with KE", "11[ENC] parsed CREATE_CHILD_SA request 6 [ SA No KE TSi TSr ]\n", true},
		{"received without KE", "06[ENC] parsed CREATE_CHILD_SA request 3 [ SA No TSi TSr ]\n", false},
		{"sent, not received", "13[ENC] generating CREATE_CHILD_SA request 2 [ SA No KE TSi TSr ]\n", false},
	}
	for _, tc := range received {
		t.Run(tc.name, func(t *testing.T) {
			got, seen := swanNewChildWithKE(tc.logs, swanParsedChildRequest)
			if got != tc.want {
				t.Fatalf("received: swanNewChildWithKE = %v, want %v (requests read %q)", got, tc.want, seen)
			}
		})
	}
}
