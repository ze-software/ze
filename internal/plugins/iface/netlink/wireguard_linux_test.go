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
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

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

// TestConfigureWireguardDevice verifies the round-trip of listen-port,
// firewall mark, and persistent keepalive through
// ConfigureWireguardDevice -> kernel state -> GetWireguardDevice.
//
// VALIDATES: AC-13 (keepalive), AC-14 (listen-port), AC-15 (fwmark).
// PREVENTS: silent field drop in buildWireguardConfig or deviceToSpec.
func TestConfigureWireguardDevice(t *testing.T) {
	if !wireguardModuleAvailable() {
		t.Skip("requires wireguard kernel module")
	}

	withTunnelNetNS(t, func(b iface.Backend) {
		if err := b.CreateWireguardDevice("wg0"); err != nil {
			t.Fatalf("CreateWireguardDevice: %v", err)
		}

		priv, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("GeneratePrivateKey: %v", err)
		}
		pub, err := wgtypes.GenerateKey()
		if err != nil {
			t.Fatalf("GenerateKey peer: %v", err)
		}

		spec := iface.WireguardSpec{
			Name:          "wg0",
			PrivateKey:    priv,
			ListenPort:    51820,
			ListenPortSet: true,
			FirewallMark:  0x1234,
			Peers: []iface.WireguardPeerSpec{{
				Name:                "site2",
				PublicKey:           pub,
				AllowedIPs:          []string{"10.0.0.2/32"},
				PersistentKeepalive: 25,
			}},
		}
		if err := b.ConfigureWireguardDevice(spec); err != nil {
			t.Fatalf("ConfigureWireguardDevice: %v", err)
		}

		got, err := b.GetWireguardDevice("wg0")
		if err != nil {
			t.Fatalf("GetWireguardDevice: %v", err)
		}

		if got.ListenPort != 51820 {
			t.Errorf("listen-port = %d, want 51820", got.ListenPort)
		}
		if got.FirewallMark != 0x1234 {
			t.Errorf("fwmark = %#x, want 0x1234", got.FirewallMark)
		}
		if got.PrivateKey != priv {
			t.Errorf("private-key not round-tripped")
		}
		if len(got.Peers) != 1 {
			t.Fatalf("peers = %d, want 1", len(got.Peers))
		}
		if got.Peers[0].PublicKey != pub {
			t.Errorf("peer public-key not round-tripped")
		}
		if got.Peers[0].PersistentKeepalive != 25 {
			t.Errorf("keepalive = %d, want 25", got.Peers[0].PersistentKeepalive)
		}
		if len(got.Peers[0].AllowedIPs) != 1 || got.Peers[0].AllowedIPs[0] != "10.0.0.2/32" {
			t.Errorf("allowed-ips = %v, want [10.0.0.2/32]", got.Peers[0].AllowedIPs)
		}
	})
}

// TestConfigureWireguardDeviceBadAllowedIP verifies that a malformed
// allowed-ips CIDR is rejected by buildWireguardConfig before the call
// reaches wgctrl, so the kernel never sees a partially-applied peer.
//
// VALIDATES: allowed-ips is parsed defensively.
// PREVENTS: half-applied peer config leaking into the kernel.
func TestConfigureWireguardDeviceBadAllowedIP(t *testing.T) {
	if !wireguardModuleAvailable() {
		t.Skip("requires wireguard kernel module")
	}

	withTunnelNetNS(t, func(b iface.Backend) {
		if err := b.CreateWireguardDevice("wg0"); err != nil {
			t.Fatalf("CreateWireguardDevice: %v", err)
		}

		priv, _ := wgtypes.GeneratePrivateKey()
		pub, _ := wgtypes.GenerateKey()

		spec := iface.WireguardSpec{
			Name:       "wg0",
			PrivateKey: priv,
			Peers: []iface.WireguardPeerSpec{{
				Name:       "bad",
				PublicKey:  pub,
				AllowedIPs: []string{"not-a-cidr"},
			}},
		}
		err := b.ConfigureWireguardDevice(spec)
		if err == nil {
			t.Fatal("expected error for malformed allowed-ips")
		}
		if !strings.Contains(err.Error(), "not-a-cidr") {
			t.Errorf("error should name the offending cidr: %v", err)
		}
	})
}
