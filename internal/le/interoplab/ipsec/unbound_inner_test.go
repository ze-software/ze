// VALIDATES: innerOutPolicyTunnels counts one answer per outbound ESP policy to the
// inner address, from the text `ip xfrm policy` prints, and ignores the inbound and
// forward policies of the same tunnel and every policy to another address.
// PREVENTS: unbound-inner-address-uniqueness passing with two outbound policies to
// 10.1.2.3/32 because a set keyed by the selector pair collapsed them into one.
package ipsec

import (
	"slices"
	"testing"
)

const unboundInnerPolicies = `src 10.20.0.1/32 dst 10.1.2.3/32
	dir out priority 2000 ptype main
	tmpl src 172.28.0.2 dst 172.28.0.7
		proto esp spi 0xc1a2b3c4 reqid 1 mode tunnel
src 10.1.2.3/32 dst 10.20.0.1/32
	dir in priority 2000 ptype main
	tmpl src 172.28.0.7 dst 172.28.0.2
		proto esp reqid 1 mode tunnel
src 10.1.2.3/32 dst 10.20.0.1/32
	dir fwd priority 2000 ptype main
	tmpl src 172.28.0.7 dst 172.28.0.2
		proto esp reqid 1 mode tunnel
src 10.20.0.1/32 dst 10.1.2.3/32
	dir out priority 2000 ptype main
	tmpl src 172.28.0.2 dst 172.28.0.9
		proto esp spi 0xd1a2b3c4 reqid 2 mode tunnel
src 10.20.0.1/32 dst 10.9.9.9/32
	dir out priority 2000 ptype main
	tmpl src 172.28.0.2 dst 172.28.0.7
		proto esp reqid 3 mode tunnel
src 0.0.0.0/0 dst 0.0.0.0/0
	socket out priority 0 ptype main
`

func TestInnerOutPolicyTunnels(t *testing.T) {
	got := innerOutPolicyTunnels(unboundInnerPolicies, "10.1.2.3/32")
	want := []string{"172.28.0.7", "172.28.0.9"}
	if !slices.Equal(got, want) {
		t.Fatalf("innerOutPolicyTunnels = %v, want %v", got, want)
	}
	if got := innerOutPolicyTunnels(unboundInnerPolicies, "10.1.2.4/32"); len(got) != 0 {
		t.Fatalf("innerOutPolicyTunnels for an address no policy names = %v, want none", got)
	}
}
