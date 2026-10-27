//go:build linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- distinguish unicast VMAC traffic from multicast control traffic.

package firewallnft

import (
	"reflect"
	"testing"

	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
)

// TestLowerUnicastMatch reads the exact expression sequence: one byte of kernel
// packet-type metadata, not an IP destination or protocol exception. HOST is
// unicast; MULTICAST and BROADCAST must not satisfy the comparison.
func TestLowerUnicastMatch(t *testing.T) {
	got, err := lowerMatch(nil, firewall.MatchUnicast{})
	if err != nil {
		t.Fatal(err)
	}
	want := []expr.Any{
		&expr.Meta{Key: expr.MetaKeyPKTTYPE, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{unix.PACKET_HOST}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unicast match = %#v, want %#v", got, want)
	}
}
