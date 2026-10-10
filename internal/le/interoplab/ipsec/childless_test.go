// Design: docs/architecture/testing/interop.md -- childless IKE SA scenario verdicts.
// Related: checkers.go -- checkChildlessIKESAZeResponder reads strongSwan's trace.
package ipsec

import "testing"

// TestSwanSentNewChildWithKE holds the responder checker to strongSwan's trace of a
// new PFS Child SA: the payload list is read as a set, a rekey (N(REKEY_SA)) or a
// request without KE does not count, and the lines read are answered for the error.
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, seen := swanSentNewChildWithKE(tc.logs)
			if got != tc.want {
				t.Fatalf("swanSentNewChildWithKE = %v, want %v (requests read %q)", got, tc.want, seen)
			}
		})
	}
	if _, seen := swanSentNewChildWithKE("x generating CREATE_CHILD_SA request 4 [ N(REKEY_SA) SA No TSi TSr ]\n"); len(seen) != 1 {
		t.Fatalf("requests read = %q, want the one rekey line", seen)
	}
}
