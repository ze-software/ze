// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- Make-before-break addresses.
// Related: addr_primary_linux.go -- Kernel address conversion.

//go:build linux

package ifacenetlink

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/vishvananda/netlink"
)

// TestP2PPrimaryRemovalPreservesNewLocal checks the kernel's same-peer subnet
// rule through the actual netlink-address conversion and deletion policy.
func TestP2PPrimaryRemovalPreservesNewLocal(t *testing.T) {
	convert := func(local string) deviceAddress {
		t.Helper()
		addr, err := netlink.ParseAddr(local + "/32")
		if err != nil {
			t.Fatal(err)
		}
		peer, err := netlink.ParseAddr("192.0.2.1/32")
		if err != nil {
			t.Fatal(err)
		}
		addr.Peer = peer.IPNet
		converted, ok := toDeviceAddress(addr)
		if !ok {
			t.Fatal("valid P2P address rejected")
		}
		return converted
	}
	old := convert("192.0.2.2")
	newer := convert("192.0.2.3")
	doomed := flushedByDelete(old, []deviceAddress{old, newer})
	if !slices.Equal(doomed, []netip.Prefix{netip.MustParsePrefix("192.0.2.3/32")}) {
		t.Fatalf("addresses requiring promotion = %v, want new local 192.0.2.3/32", doomed)
	}
}
