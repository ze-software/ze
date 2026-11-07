//go:build integration && linux

// Design: docs/features/interfaces.md -- Idempotent exact P2P removal.
// Related: addr_primary_linux.go -- Narrow absence handling at lookup and delete.

package ifacenetlink

import (
	"errors"
	"runtime"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestP2PAbsentPairPreservesUnrelated uses the real public backend and kernel
// dump to check an absent same-subnet peer cannot delete either installed pair.
func TestP2PAbsentPairPreservesUnrelated(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	b := &netlinkBackend{}
	if err := b.CreateDummy("p2p-absence"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.DeleteInterface("p2p-absence"); err != nil {
			t.Error(err)
		}
	})
	for _, pair := range [][2]string{
		{"198.51.100.2/32", "192.0.2.1/24"},
		{"203.0.113.2/32", "203.0.113.1/32"},
	} {
		if err := b.AddAddressP2P("p2p-absence", pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	link, err := netlink.LinkByName("p2p-absence")
	if err != nil {
		t.Fatal(err)
	}
	before, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 {
		t.Fatalf("initial pairs = %v", before)
	}
	if err := b.RemoveAddressP2P("p2p-absence", "198.51.100.2/32", "192.0.2.9/24"); err != nil {
		t.Fatalf("proven absence failed: %v", err)
	}
	after, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("surviving pairs = %v, want %v", after, before)
	}
	for _, want := range before {
		found := false
		for _, got := range after {
			if !got.Equal(want) {
				continue
			}
			if got.PeerEqual(want) {
				found = true
			}
		}
		if !found {
			t.Errorf("lost installed pair %v peer %v", want.IPNet, want.Peer)
		}
	}
}

// TestP2PRemovalGenuineErrors exercises public parse/lookup failures and an
// actual kernel permission failure, then checks the installed pair survived.
func TestP2PRemovalGenuineErrors(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	b := &netlinkBackend{}
	for _, tc := range []struct{ name, device, local, peer string }{
		{"missing-link", "p2p-missing", "192.0.2.2/32", "192.0.2.1/32"},
		{"bad-local", "lo", "invalid", "192.0.2.1/32"},
		{"bad-peer", "lo", "192.0.2.2/32", "invalid"},
		{"mixed-family", "lo", "192.0.2.2/32", "2001:db8::1/128"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := b.RemoveAddressP2P(tc.device, tc.local, tc.peer)
			if err == nil {
				t.Fatal("genuine error was accepted as absence")
			}
			if errors.Is(err, errP2PPairAbsent) {
				t.Fatalf("genuine error became absence: %v", err)
			}
		})
	}
	if err := b.CreateDummy("p2p-errors"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.DeleteInterface("p2p-errors"); err != nil {
			t.Error(err)
		}
	})
	if err := b.AddAddressP2P("p2p-errors", "192.0.2.2/32", "192.0.2.1/32"); err != nil {
		t.Fatal(err)
	}
	t.Run("permission", func(t *testing.T) {
		// Capabilities are thread-local. Keep the synchronous netlink call on
		// this thread and restore its effective set before returning it to Go.
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
		err := b.RemoveAddressP2P("p2p-errors", "192.0.2.2/32", "192.0.2.1/32")
		if !errors.Is(err, unix.EPERM) {
			t.Fatalf("permission failure = %v, want EPERM", err)
		}
	})
	link, err := netlink.LinkByName("p2p-errors")
	if err != nil {
		t.Fatal(err)
	}
	addresses, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 1 {
		t.Fatalf("permission failure changed installed pairs: %v", addresses)
	}
	want := exactRemovalPair(t, "192.0.2.2/32", "192.0.2.1/32")
	if !addresses[0].Equal(*want) {
		t.Fatal("permission failure changed local address")
	}
	if !addresses[0].PeerEqual(*want) {
		t.Fatal("permission failure changed peer address")
	}
}

// TestP2PDeleteAbsenceIsNarrow exercises the post-dump disappearance path using
// actual AddrDel errors, distinguishing exact IPv4 absence from generic deletion.
func TestP2PDeleteAbsenceIsNarrow(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	link, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	request := exactRemovalPair(t, "192.0.2.2/32", "192.0.2.1/32")
	target, ok := toDeviceAddress(request)
	if !ok {
		t.Fatal("valid pair rejected")
	}
	remover := &netlinkAddrRemover{link: link, addr: request, exactPair: true}
	b := &netlinkBackend{}
	if err := b.AddAddressP2P("lo", "192.0.2.2/32", "192.0.2.1/32"); err != nil {
		t.Fatal(err)
	}
	if err := remover.Delete("lo", target); err != nil {
		t.Fatalf("installed exact pair deletion failed: %v", err)
	}
	if err := remover.Delete("lo", target); err != nil {
		t.Fatalf("exact IPv4 disappearance failed: %v", err)
	}
	remover.exactPair = false
	if err := remover.Delete("lo", target); !errors.Is(err, unix.EADDRNOTAVAIL) {
		t.Fatalf("generic deletion error = %v, want EADDRNOTAVAIL", err)
	}
	remover.exactPair = true
	remover.link = &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Index: 2147483647}}
	if err := remover.Delete("missing", target); !errors.Is(err, unix.ENODEV) {
		t.Fatalf("missing device error = %v, want ENODEV", err)
	}
	remover.link = link
	remover.addr = exactRemovalPair(t, "2001:db8::2/128", "2001:db8::1/128")
	target, ok = toDeviceAddress(remover.addr)
	if !ok {
		t.Fatal("valid IPv6 pair rejected")
	}
	if err := remover.Delete("lo", target); err == nil {
		t.Fatal("IPv6 deletion error was swallowed by the IPv4 absence exception")
	}
}
