// WireGuard netlink integration tests. Each subtest creates a fresh netns,
// calls CreateWireguardDevice via the iface.Backend interface, and verifies
// the resulting netdev with netlink.LinkByName so the kind and admin state
// are checked against kernel state.
//
// Build tags require both `integration` and `linux`. The runner skips when
// CAP_NET_ADMIN is unavailable so unprivileged CI hosts pass cleanly, and
// also skips when the wireguard kernel module is not loaded (LinkAdd fails
// with "operation not supported" on a kernel without the module).

//go:build integration && linux

package ifacenetlink

import (
	"os"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// wireguardModuleAvailable reports whether the running kernel has the
// wireguard module loaded (or built in). Without the module, LinkAdd fails
// with EOPNOTSUPP and the test cannot meaningfully exercise the create
// path, so we skip rather than fail.
func wireguardModuleAvailable() bool {
	if _, err := os.Stat("/sys/module/wireguard"); err == nil {
		return true
	}
	return false
}

// TestCreateWireguardDevice verifies that a wireguard netdev is created
// with the expected name, kind, and admin state.
//
// VALIDATES: Phase 6 -- netlink backend creates a wireguard netdev.
// PREVENTS: silent regression if netlink.Wireguard{} is replaced or the
// LinkAdd call path is refactored without preserving kind semantics.
func TestCreateWireguardDevice(t *testing.T) {
	if !wireguardModuleAvailable() {
		t.Skip("requires wireguard kernel module")
	}

	withTunnelNetNS(t, func(b iface.Backend) {
		if err := b.CreateWireguardDevice("wg0"); err != nil {
			t.Fatalf("CreateWireguardDevice: %v", err)
		}

		link, err := netlink.LinkByName("wg0")
		if err != nil {
			t.Fatalf("LinkByName wg0: %v", err)
		}
		if link.Type() != "wireguard" {
			t.Errorf("link type = %q, want wireguard", link.Type())
		}
		if link.Attrs().Flags&1 == 0 {
			// IFF_UP is bit 0; CreateWireguardDevice calls LinkSetUp after LinkAdd.
			t.Errorf("wg0 not administratively up: flags=%#x", link.Attrs().Flags)
		}
	})
}

// TestCreateWireguardDeviceInvalidName verifies that an invalid iface name
// is rejected before any netlink call is made, matching the CreateTunnel
// behavior and preventing malformed names from reaching the kernel.
//
// VALIDATES: name validation guards the CreateWireguardDevice entry point.
// PREVENTS: unchecked strings flowing into LinkAdd.
func TestCreateWireguardDeviceInvalidName(t *testing.T) {
	if !wireguardModuleAvailable() {
		t.Skip("requires wireguard kernel module")
	}

	withTunnelNetNS(t, func(b iface.Backend) {
		// 20 characters exceeds Linux's IFNAMSIZ=16 limit.
		err := b.CreateWireguardDevice("wg-name-way-too-long")
		if err == nil {
			t.Fatal("expected error for invalid name, got nil")
		}
		if !strings.Contains(err.Error(), "wg-name-way-too-long") {
			t.Errorf("error does not name the offending iface: %v", err)
		}
	})
}

// TestCreateAndDeleteWireguardDevice verifies the create+delete round-trip
// that Phase 4 reconciliation will use to remove wireguard netdevs not in
// config.
//
// VALIDATES: the netdev created by CreateWireguardDevice is cleanly
// removable via DeleteInterface (same path as every other iface kind).
// PREVENTS: stranded netdevs after a config reload removes a wireguard
// block.
func TestCreateAndDeleteWireguardDevice(t *testing.T) {
	if !wireguardModuleAvailable() {
		t.Skip("requires wireguard kernel module")
	}

	withTunnelNetNS(t, func(b iface.Backend) {
		if err := b.CreateWireguardDevice("wg1"); err != nil {
			t.Fatalf("CreateWireguardDevice: %v", err)
		}
		if err := b.DeleteInterface("wg1"); err != nil {
			t.Fatalf("DeleteInterface: %v", err)
		}
		if _, err := netlink.LinkByName("wg1"); err == nil {
			t.Errorf("wg1 still present after DeleteInterface")
		}
	})
}
