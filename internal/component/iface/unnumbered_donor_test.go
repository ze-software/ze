// VALIDATES: gap #55 owed item 3, iface side. An unnumbered unit's borrowed /32
// is withdrawn when the donor goes down or loses the address, restored when the
// donor recovers, and left alone when the backend cannot report addresses.
// METHOD: reconcileUnnumbered over a parsed config and a backend whose device
// list the test sets, recording the address calls it makes.
package iface

import (
	"errors"
	"net/netip"
	"slices"
	"testing"
)

// donorBackend answers ListInterfaces from infos and records address calls.
// Every other Backend method is unused here and panics through the nil embed.
type donorBackend struct {
	Backend
	infos   []InterfaceInfo
	added   []string
	removed []string
}

func (b *donorBackend) ListInterfaces() ([]InterfaceInfo, error) { return b.infos, nil }

func (b *donorBackend) AddAddress(name, cidr string) error {
	b.added = append(b.added, name+" "+cidr)
	return nil
}

func (b *donorBackend) RemoveAddress(name, cidr string) error {
	b.removed = append(b.removed, name+" "+cidr)
	return nil
}

func donorInfo(name, state string, addrs ...AddrInfo) InterfaceInfo {
	return InterfaceInfo{Name: name, OsName: name, State: state, Addresses: addrs, AddressesRead: true}
}

func TestUnnumberedDonorReconcile(t *testing.T) {
	cfg, err := parseIfaceConfig(`{"interface": {
		"dummy": {"dummy0": {"unit": {"0": {"ipv4": {"address": ["10.255.0.1/32"]}}}}},
		"ethernet": {"eth0": {"unit": {"0": {"ipv4": {"unnumbered": "dummy0"}}}}}
	}}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	donorAddr := AddrInfo{Address: "10.255.0.1", PrefixLength: 32, Family: "ipv4"}
	borrowed := AddrInfo{Address: "10.255.0.1", PrefixLength: 32, Family: "ipv4"}
	cases := []struct {
		name    string
		infos   []InterfaceInfo
		added   []string
		removed []string
	}{
		{
			name:  "donor up with address, /32 present: nothing to do",
			infos: []InterfaceInfo{donorInfo("dummy0", LinkStateUp, donorAddr), donorInfo("eth0", LinkStateUp, borrowed)},
		},
		{
			name:    "donor down: /32 withdrawn",
			infos:   []InterfaceInfo{donorInfo("dummy0", LinkStateDown, donorAddr), donorInfo("eth0", LinkStateUp, borrowed)},
			removed: []string{"eth0 10.255.0.1/32"},
		},
		{
			name:    "donor lost the address: /32 withdrawn",
			infos:   []InterfaceInfo{donorInfo("dummy0", LinkStateUp), donorInfo("eth0", LinkStateUp, borrowed)},
			removed: []string{"eth0 10.255.0.1/32"},
		},
		{
			name:  "donor recovered: /32 restored",
			infos: []InterfaceInfo{donorInfo("dummy0", LinkStateUp, donorAddr), donorInfo("eth0", LinkStateUp)},
			added: []string{"eth0 10.255.0.1/32"},
		},
		{
			name:  "borrowing device absent: nothing to do",
			infos: []InterfaceInfo{donorInfo("dummy0", LinkStateUp, donorAddr)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := &donorBackend{infos: tc.infos}
			if errs := reconcileUnnumbered(cfg, b); len(errs) != 0 {
				t.Fatalf("reconcile errors: %v", errs)
			}
			if !slices.Equal(b.added, tc.added) || !slices.Equal(b.removed, tc.removed) {
				t.Fatalf("added %v removed %v, want added %v removed %v", b.added, b.removed, tc.added, tc.removed)
			}
		})
	}

	// The borrowed /32 itself never proves the donor is alive.
	if donorLive([]InterfaceInfo{donorInfo("eth0", LinkStateUp, borrowed)}, netip.MustParseAddr("10.255.0.1"), cfg.unnumberedDevices()) {
		t.Fatal("the borrower's own /32 was taken as a live donor")
	}

	// A backend that did not read addresses gives no answer, and nothing moves.
	unread := donorInfo("eth0", LinkStateUp)
	unread.AddressesRead = false
	b := &donorBackend{infos: []InterfaceInfo{donorInfo("dummy0", LinkStateDown), unread}}
	errs := reconcileUnnumbered(cfg, b)
	if len(errs) != 1 || !errors.Is(errs[0], errUnnumberedAddressesUnread) {
		t.Fatalf("errors %v, want errUnnumberedAddressesUnread", errs)
	}
	if len(b.added)+len(b.removed) != 0 {
		t.Fatalf("addresses moved without an address read: added %v removed %v", b.added, b.removed)
	}
}
