//go:build linux

// Design: docs/features/interfaces.md -- what the address reconcile removes.
// Related: config_apply.go -- reconcileOnReadyWithJournal, the "remove extra addresses" pass.

package iface

import (
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestIntegrationReconcileKeepsLearnedAddresses applies a config with one static
// IPv4 on a veth unit, lets the unit learn three addresses the config never names,
// then applies an unrelated change and checks that all three survive.
//
// VALIDATES: the reconcile removes only addresses Ze would have installed from
// config: a DHCP lease, a SLAAC address and the kernel's IPv6 link-local stay on
// the unit across a commit that does not touch it, and a static address the new
// config dropped is still removed. A registered DHCPv4 lease with an infinite
// lifetime, which the kernel holds as permanent, also stays and keeps that
// lifetime, and a registered lease that is no longer installed is not put back.
// PREVENTS: every commit, vpp connect or owned-address registration stripping a
// DHCP-leased address, SLAAC addresses and fe80:: from a configured unit, and a
// reconcile resurrecting an expired lease as a permanent address.
//
// The leases are installed with ReplaceAddressWithLifetime, the exact call the
// DHCP client makes on an ACK (internal/plugins/iface/dhcp); the leased
// registration is RegisterLeasedAddresses, the call it makes before that. The SLAAC address
// is the same call over IPv6, which leaves IFA_F_PERMANENT clear as a Router
// Advertisement does (see slaac_integration_linux_test.go).
func TestIntegrationReconcileKeepsLearnedAddresses(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNetNS(t, func() {
		const (
			dev       = "ze0"
			peer      = "ze0p"
			static    = "10.90.0.1/24"
			dropped   = "10.90.0.2/24"
			lease     = "10.90.1.5/24"
			slaac     = "2001:db8:90::5/64"
			leaseSecs = 3600
			infinite  = "10.90.2.5/24"
			expired   = "10.90.3.5/24"
			owner     = "integration-dhcp"
			forever   = 0xffffffff
		)
		t.Cleanup(func() { UnregisterLeasedAddresses(owner) })
		b := GetBackend()
		previous := &ifaceConfig{
			Backend: "netlink",
			Veth: []vethEntry{{
				Name:  dev,
				Units: []unitEntry{{Addresses: []string{static, dropped}}},
				Peer:  peer,
			}},
		}
		t.Cleanup(func() { _ = DeleteInterface(dev) })
		t.Cleanup(func() { _ = DeleteInterface("ze1") })

		if errs := applyConfig(previous, nil, b); len(errs) > 0 {
			t.Fatalf("first apply: %v", errs)
		}
		// The kernel generates the link-local once the link has carrier,
		// which a veth has only when both ends are up.
		if err := SetAdminUp(peer); err != nil {
			t.Fatalf("SetAdminUp %s: %v", peer, err)
		}
		// The kernel's link-local can appear while the first apply brings the
		// veth up, so that apply's own reconcile is the first chance to lose it.
		linkLocal := waitForLinkLocal(t, dev)
		if linkLocal == "" {
			t.Errorf("no IPv6 link-local on %s after the first apply: the kernel generates one"+
				" when the link comes up, so the apply's reconcile removed it", dev)
		}

		if err := ReplaceAddressWithLifetime(dev, lease, leaseSecs, leaseSecs); err != nil {
			t.Fatalf("install DHCP-style lease %s: %v", lease, err)
		}
		if err := ReplaceAddressWithLifetime(dev, slaac, leaseSecs, leaseSecs/2); err != nil {
			t.Fatalf("install SLAAC-style address %s: %v", slaac, err)
		}
		// An infinite DHCPv4 lease (RFC 2131 Section 3.3, 0xffffffff), and a
		// registered lease whose address the client already removed.
		if err := RegisterLeasedAddresses(dev, owner, []string{infinite, expired}); err != nil {
			t.Fatalf("register leases: %v", err)
		}
		if err := ReplaceAddressWithLifetime(dev, infinite, forever, forever); err != nil {
			t.Fatalf("install infinite lease %s: %v", infinite, err)
		}

		// The unrelated change: a second dummy, and the static address dropped.
		current := &ifaceConfig{
			Backend: "netlink",
			Veth: []vethEntry{{
				Name:  dev,
				Units: []unitEntry{{Addresses: []string{static}}},
				Peer:  peer,
			}},
			Dummy: []ifaceEntry{{Name: "ze1"}},
		}
		if errs := applyConfig(current, previous, b); len(errs) > 0 {
			t.Fatalf("second apply: %v", errs)
		}

		requireAddress(t, dev, static)
		requireAddress(t, dev, lease)
		requireAddress(t, dev, slaac)
		requireAddress(t, dev, infinite)
		requireNoAddress(t, dev, expired)
		requireValidLifetime(t, dev, infinite, func(v int) bool { return v == forever }, "0xffffffff (infinite)")
		requireValidLifetime(t, dev, lease, func(v int) bool { return v > 0 && v <= leaseSecs }, "finite, at most the lease")
		if linkLocal != "" {
			requireAddress(t, dev, linkLocal)
		}
		requireNoAddress(t, dev, dropped)
	})
}

// waitForLinkLocal returns the IPv6 link-local address the kernel generates
// when the device comes up, or "" when none is present after three seconds.
// The kernel adds it asynchronously to the admin up, so the helper polls.
func waitForLinkLocal(t *testing.T, dev string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		link, err := netlink.LinkByName(dev)
		if err != nil {
			t.Fatalf("LinkByName %s: %v", dev, err)
		}
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			t.Fatalf("AddrList %s: %v", dev, err)
		}
		for i := range addrs {
			if addrs[i].IP.IsLinkLocalUnicast() {
				return addrs[i].IPNet.String()
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ""
}

// requireValidLifetime fails unless cidr on dev carries a valid lifetime ok
// accepts: the reconcile must neither re-add a lease as permanent nor change
// the lifetime its client installed.
func requireValidLifetime(t *testing.T, dev, cidr string, ok func(int) bool, want string) {
	t.Helper()
	link, err := netlink.LinkByName(dev)
	if err != nil {
		t.Fatalf("LinkByName %s: %v", dev, err)
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("AddrList %s: %v", dev, err)
	}
	for i := range addrs {
		if addrs[i].IPNet.String() != cidr {
			continue
		}
		if !ok(addrs[i].ValidLft) {
			t.Errorf("%s on %s: valid lifetime %d, want %s", cidr, dev, addrs[i].ValidLft, want)
		}
		return
	}
	t.Errorf("%s not on %s", cidr, dev)
}
