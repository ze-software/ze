// Design: docs/features/interfaces.md -- Bridge interface management
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// bridgeSysfsRoot is the base path for bridge sysfs writes.
// Tests override this to a temporary directory.
var bridgeSysfsRoot = "/sys/class/net"

func (b *netlinkBackend) BridgeAddPort(bridgeName, portName string) error {
	if err := iface.ValidateIfaceName(bridgeName); err != nil {
		return fmt.Errorf("iface: bridge add port: bridge: %w", err)
	}
	if err := iface.ValidateIfaceName(portName); err != nil {
		return fmt.Errorf("iface: bridge add port: port: %w", err)
	}
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("iface: bridge %q not found: %w", bridgeName, err)
	}
	if br.Type() != linkTypeBridge {
		return fmt.Errorf("iface: %q is not a bridge (type %q)", bridgeName, br.Type())
	}
	port, err := netlink.LinkByName(portName)
	if err != nil {
		return fmt.Errorf("iface: bridge port %q not found: %w", portName, err)
	}
	if err := netlink.LinkSetMaster(port, br); err != nil {
		return fmt.Errorf("iface: add port %q to bridge %q: %w", portName, bridgeName, err)
	}
	return nil
}

func (b *netlinkBackend) BridgeDelPort(portName string) error {
	if err := iface.ValidateIfaceName(portName); err != nil {
		return fmt.Errorf("iface: bridge del port: %w", err)
	}
	port, err := netlink.LinkByName(portName)
	if err != nil {
		return fmt.Errorf("iface: bridge del port %q: not found: %w", portName, err)
	}
	if err := netlink.LinkSetNoMaster(port); err != nil {
		return fmt.Errorf("iface: remove port %q from bridge: %w", portName, err)
	}
	return nil
}

func (b *netlinkBackend) BridgeSetSTP(bridgeName string, enabled bool) error {
	if err := iface.ValidateIfaceName(bridgeName); err != nil {
		return fmt.Errorf("iface: bridge stp: %w", err)
	}
	val := "0"
	if enabled {
		val = "1"
	}
	path := filepath.Join(bridgeSysfsRoot, bridgeName, "bridge", "stp_state")
	if err := os.WriteFile(path, []byte(val), 0o600); err != nil {
		return fmt.Errorf("iface: bridge stp on %q: %w", bridgeName, err)
	}
	return nil
}
