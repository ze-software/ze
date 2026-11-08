//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the tunnel-source guard passes a packet entering a Child SA.

package firewallnft

import (
	"reflect"
	"testing"

	"github.com/google/nftables/expr"

	"github.com/ze-software/ze/internal/component/firewall"
)

// VALIDATES: MatchIPsecTransform lowers to nft `rt ipsec exists`: the rt expression
// with key NFT_RT_XFRM (4) into register 1, compared equal to the one byte 1.
// Method: lower the match and compare the exact expression sequence.
// PREVENTS: a guard term that names the transform but lowers to another rt key (the
// classid or a next hop), or that a missing case refuses at publish.
func TestLowerIPsecTransformMatch(t *testing.T) {
	got, err := lowerMatch(nil, firewall.MatchIPsecTransform{})
	if err != nil {
		t.Fatal(err)
	}
	want := []expr.Any{
		&expr.Rt{Register: 1, Key: expr.RtKey(4)},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ipsec transform match = %#v, want %#v", got, want)
	}
}
