// VALIDATES: AC-5 (DHCP lease applied: address + default route; stale flushed)
// PREVENTS: stale foreign-NIC default route surviving ze.mac pinning

//go:build linux

package disk

import (
	"net"
	"testing"
)

func TestApplyLeaseAddsAddressAndRoute(t *testing.T) {
	var added []string
	var routed []string
	ops := &netlinkOps{
		addrReplace: func(ifName, cidr string) error { added = append(added, cidr); return nil },
		routeReplace: func(ifName, dst, gw string) error {
			routed = append(routed, dst+"->"+gw)
			return nil
		},
		addrFlush:  func(string) error { return nil },
		routeFlush: func(string) error { return nil },
		linkUp:     func(string) error { return nil },
	}

	lease := &dhcpLease{
		IP:     net.IPv4(10, 0, 0, 5),
		Mask:   net.CIDRMask(24, 32),
		Router: net.IPv4(10, 0, 0, 1),
		Iface:  "eth0",
	}

	if err := applyLease(ops, lease); err != nil {
		t.Fatalf("applyLease: %v", err)
	}
	if len(added) != 1 || added[0] != "10.0.0.5/24" {
		t.Fatalf("added = %v, want [10.0.0.5/24]", added)
	}
	if len(routed) != 1 || routed[0] != "0.0.0.0/0->10.0.0.1" {
		t.Fatalf("routed = %v, want [0.0.0.0/0->10.0.0.1]", routed)
	}
}

func TestFlushIfaceRemovesAddrAndRoute(t *testing.T) {
	flushedAddr := false
	flushedRoute := false
	ops := &netlinkOps{
		addrFlush:    func(string) error { flushedAddr = true; return nil },
		routeFlush:   func(string) error { flushedRoute = true; return nil },
		addrReplace:  func(string, string) error { return nil },
		routeReplace: func(string, string, string) error { return nil },
		linkUp:       func(string) error { return nil },
	}

	if err := sysFlushIface(ops, "eth0"); err != nil {
		t.Fatalf("flushIface: %v", err)
	}
	if !flushedAddr {
		t.Fatal("addrFlush not called")
	}
	if !flushedRoute {
		t.Fatal("routeFlush not called")
	}
}
