// Design: docs/features/interfaces.md -- WireGuard netdev creation via netlink
// Overview: ifacenetlink.go -- package hub
// Related: backend_linux.go -- netlinkBackend type and Close()
// Related: tunnel_linux.go -- sibling Create* implementation (tunnel)

//go:build linux

package ifacenetlink

import (
	"fmt"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// CreateWireguardDevice creates a WireGuard netdev with the given name.
// The vendored netlink library exposes a Wireguard{} LinkType that carries
// only LinkAttrs (name, MTU, MAC, ...) -- all WireGuard-specific config
// (private-key, listen-port, fwmark, peers) goes through the wgctrl
// genetlink protocol in a later phase, not through rtnetlink.
//
// On LinkSetUp failure after a successful LinkAdd, the partial netdev is
// removed via LinkDel so the operator does not end up with a half-created
// interface after a transient error; rollback delete failures are logged.
func (b *netlinkBackend) CreateWireguardDevice(name string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create wireguard %q: %w", name, err)
	}

	link := &netlink.Wireguard{
		LinkAttrs: netlink.LinkAttrs{Name: name},
	}

	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create wireguard %q: %w", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		if delErr := netlink.LinkDel(link); delErr != nil {
			loggerPtr.Load().Warn("iface: rollback delete after set-up failure",
				"name", name, "kind", "wireguard", "err", delErr)
		}
		return fmt.Errorf("iface: set up wireguard %q: %w", name, err)
	}
	return nil
}
