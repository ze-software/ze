//go:build linux

// Design: docs/guide/firewall.md -- the destination-local match
// Related: lower_linux.go -- lowerDestinationLocalMatch under test

package firewallnft

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
)

// TestLowerLocalDestination reads the exact expression sequence of the
// destination-local match: the nft form of `fib daddr type local`, a fib
// lookup keyed on the destination address that stores the route type, then a
// comparison with RTN_LOCAL in host byte order, the order nft_fib_eval stores.
// A lookup keyed on the source, or one that stores the output interface,
// matches other packets, and so does a comparison in network byte order on a
// little-endian host.
func TestLowerLocalDestination(t *testing.T) {
	got, err := lowerMatch(nil, firewall.MatchDestinationLocal{})
	if err != nil {
		t.Fatal(err)
	}
	rtnLocal := make([]byte, 4)
	binary.NativeEndian.PutUint32(rtnLocal, unix.RTN_LOCAL)
	want := []expr.Any{
		&expr.Fib{Register: 1, FlagDADDR: true, ResultADDRTYPE: true},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: rtnLocal},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("destination-local match = %#v, want %#v", got, want)
	}
}
