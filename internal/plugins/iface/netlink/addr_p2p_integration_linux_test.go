//go:build integration && linux

// Design: docs/features/interfaces.md -- exact point-to-point address replacement.
// Related: addr_primary_linux.go -- installed-pair lookup and cascade protection.

package ifacenetlink

import (
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestP2PKernelReplacement checks make-before-break through the real backend,
// then reads the kernel independently. Every case gets its own dummy link in
// the test's private namespace; no host address or route is changed.
func TestP2PKernelReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	for _, tc := range []struct {
		name, local, peer string
	}{
		{"local-only", "192.0.2.3/32", "192.0.2.1/32"},
		{"peer-only", "192.0.2.2/32", "192.0.2.9/32"},
		{"both", "192.0.2.3/32", "192.0.2.9/32"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend, err := newNetlinkBackend()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := backend.Close(); err != nil {
					t.Error(err)
				}
			})
			const device = "ze-p2p-proof"
			if err := backend.CreateDummy(device); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := backend.DeleteInterface(device); err != nil {
					t.Error(err)
				}
			})
			if err := backend.AddAddressP2P(device, "192.0.2.2/32", "192.0.2.1/32"); err != nil {
				t.Fatal(err)
			}
			if err := backend.AddAddressP2P(device, tc.local, tc.peer); err != nil {
				t.Fatal(err)
			}
			link, err := netlink.LinkByName(device)
			if err != nil {
				t.Fatal(err)
			}
			before, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err != nil {
				t.Fatal(err)
			}
			if len(before) != 2 {
				t.Fatalf("make-before-break installed %d addresses, want two: %v", len(before), before)
			}
			if err := backend.RemoveAddressP2P(device, "192.0.2.2/32", "192.0.2.1/32"); err != nil {
				t.Fatal(err)
			}
			after, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err != nil {
				t.Fatal(err)
			}
			if len(after) != 1 {
				t.Fatalf("replacement lost or old pair retained: %v", after)
			}
			local, ok := netip.AddrFromSlice(after[0].IP)
			if !ok {
				t.Fatalf("kernel returned invalid local address: %v", after)
			}
			if local.Unmap() != netip.MustParsePrefix(tc.local).Addr() {
				t.Fatalf("wrong surviving local: %v", after)
			}
			if after[0].Peer == nil {
				t.Fatalf("kernel lost peer: %v", after)
			}
			peer, ok := netip.AddrFromSlice(after[0].Peer.IP)
			if !ok {
				t.Fatalf("kernel returned invalid peer: %v", after)
			}
			if peer.Unmap() != netip.MustParsePrefix(tc.peer).Addr() {
				t.Fatalf("wrong surviving peer: %v", after)
			}
		})
	}
}
