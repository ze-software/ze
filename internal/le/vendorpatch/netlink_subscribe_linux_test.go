//go:build linux

// Design: docs/features/interfaces.md -- existing addresses on the monitor subscription

package vendorpatch

import (
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// TestAddressSubscriptionDumpsWithoutNetAdmin reads the real loopback address
// through a multicast subscription with ListExisting, after removing effective
// CAP_NET_ADMIN from the subscribing thread. A dump must reach the kernel by
// unicast, not be sent to the subscribed multicast groups. The latter returns
// EPERM for an ordinary user and aborts the interface plugin's startup.
func TestAddressSubscriptionDumpsWithoutNetAdmin(t *testing.T) {
	link, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	updates := make(chan netlink.AddrUpdate, 64)
	done := make(chan struct{})
	defer close(done)
	if err := subscribeAddressesWithoutNetAdmin(t, updates, done); err != nil {
		t.Fatalf("subscribe and dump without CAP_NET_ADMIN: %v", err)
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case update, open := <-updates:
			if !open {
				t.Fatal("address subscription closed before dumping loopback")
			}
			if update.LinkIndex == link.Attrs().Index {
				if update.NewAddr {
					if update.LinkAddress.IP.IsLoopback() {
						return
					}
				}
			}
		case <-deadline.C:
			t.Fatal("address subscription did not dump the existing loopback address")
		}
	}
}

// subscribeAddressesWithoutNetAdmin changes only this locked thread's effective
// capability set and restores it before returning the thread to the runtime.
func subscribeAddressesWithoutNetAdmin(t *testing.T, updates chan<- netlink.AddrUpdate, done <-chan struct{}) error {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var original [2]unix.CapUserData
	if err := unix.Capget(&header, &original[0]); err != nil {
		t.Fatal(err)
	}
	restricted := original
	restricted[0].Effective &^= 1 << unix.CAP_NET_ADMIN
	if err := unix.Capset(&header, &restricted[0]); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Capset(&header, &original[0]); err != nil {
			t.Fatal(err)
		}
	}()
	return netlink.AddrSubscribeWithOptions(updates, done, netlink.AddrSubscribeOptions{ListExisting: true})
}
