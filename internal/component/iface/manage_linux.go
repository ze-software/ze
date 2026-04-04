// Design: plan/spec-iface-0-umbrella.md — Interface management via netlink
// Overview: iface.go — shared types and topic constants
// Related: bridge_linux.go — bridge-specific management (ports, STP)

package iface

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// linkTypeBridge is the netlink link type string for bridge interfaces.
const linkTypeBridge = "bridge"

// VLAN ID range per IEEE 802.1Q.
const (
	minVLANID = 1
	maxVLANID = 4094
)

// MTU limits. 68 is the minimum for IPv4 (RFC 791). 16000 is a practical
// upper bound for common virtual/physical NICs (jumbo frames).
const (
	minMTU = 68
	maxMTU = 16000
)

// validateVLANID checks that id is in the valid 802.1Q range [1, 4094].
func validateVLANID(id int) error {
	if id < minVLANID || id > maxVLANID {
		return fmt.Errorf("iface: vlan id %d not in [%d, %d]",
			id, minVLANID, maxVLANID)
	}
	return nil
}

// validateMTU checks that mtu is in the supported range [68, 16000].
func validateMTU(mtu int) error {
	if mtu < minMTU || mtu > maxMTU {
		return fmt.Errorf("iface: mtu %d not in [%d, %d]",
			mtu, minMTU, maxMTU)
	}
	return nil
}

// CreateDummy creates a dummy interface and brings it up.
func CreateDummy(name string) error {
	if err := validateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create dummy %q: %w", name, err)
	}

	link := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{Name: name},
	}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create dummy %q: %w", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		_ = netlink.LinkDel(link) // best-effort cleanup
		return fmt.Errorf("iface: set up dummy %q: %w", name, err)
	}
	return nil
}

// CreateVeth creates a veth pair and brings both ends up.
func CreateVeth(name, peerName string) error {
	if err := validateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create veth %q: %w", name, err)
	}
	if err := validateIfaceName(peerName); err != nil {
		return fmt.Errorf("iface: create veth peer %q: %w", peerName, err)
	}

	link := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name},
		PeerName:  peerName,
	}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create veth %q/%q: %w", name, peerName, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		_ = netlink.LinkDel(link) // best-effort cleanup
		return fmt.Errorf("iface: set up veth %q: %w", name, err)
	}

	peer, err := netlink.LinkByName(peerName)
	if err != nil {
		_ = netlink.LinkDel(link) // best-effort cleanup
		return fmt.Errorf("iface: lookup veth peer %q: %w", peerName, err)
	}
	if err := netlink.LinkSetUp(peer); err != nil {
		_ = netlink.LinkDel(link) // best-effort cleanup (removes both ends)
		return fmt.Errorf("iface: set up veth peer %q: %w", peerName, err)
	}
	return nil
}

// CreateBridge creates a bridge interface and brings it up.
func CreateBridge(name string) error {
	if err := validateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create bridge %q: %w", name, err)
	}

	link := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{Name: name},
	}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create bridge %q: %w", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		_ = netlink.LinkDel(link) // best-effort cleanup
		return fmt.Errorf("iface: set up bridge %q: %w", name, err)
	}
	return nil
}

// CreateVLAN creates a VLAN subinterface named "<parentName>.<vlanID>"
// on the given parent interface and brings it up.
func CreateVLAN(parentName string, vlanID int) error {
	if err := validateIfaceName(parentName); err != nil {
		return fmt.Errorf("iface: create vlan on %q: %w", parentName, err)
	}
	if err := validateVLANID(vlanID); err != nil {
		return fmt.Errorf("iface: create vlan on %q: %w", parentName, err)
	}

	parent, err := netlink.LinkByName(parentName)
	if err != nil {
		return fmt.Errorf("iface: create vlan: parent %q not found: %w", parentName, err)
	}

	vlanName := fmt.Sprintf("%s.%d", parentName, vlanID)
	if err := validateIfaceName(vlanName); err != nil {
		return fmt.Errorf("iface: create vlan: composed name too long: %w", err)
	}
	vlan := &netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        vlanName,
			ParentIndex: parent.Attrs().Index,
		},
		VlanId: vlanID,
	}
	if err := netlink.LinkAdd(vlan); err != nil {
		return fmt.Errorf("iface: create vlan %q: %w", vlanName, err)
	}
	if err := netlink.LinkSetUp(vlan); err != nil {
		_ = netlink.LinkDel(vlan) // best-effort cleanup
		return fmt.Errorf("iface: set up vlan %q: %w", vlanName, err)
	}
	return nil
}

// DeleteInterface removes an interface by name.
func DeleteInterface(name string) error {
	if err := validateIfaceName(name); err != nil {
		return fmt.Errorf("iface: delete %q: %w", name, err)
	}

	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("iface: delete %q: not found: %w", name, err)
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("iface: delete %q: %w", name, err)
	}
	return nil
}

// AddAddress adds an IP address in CIDR notation to the named interface.
func AddAddress(ifaceName, cidr string) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: add address on %q: %w", ifaceName, err)
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("iface: add address %q on %q: %w", cidr, ifaceName, err)
	}

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: add address on %q: not found: %w", ifaceName, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("iface: add address %q on %q: %w", cidr, ifaceName, err)
	}
	return nil
}

// RemoveAddress removes an IP address in CIDR notation from the named interface.
func RemoveAddress(ifaceName, cidr string) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: remove address on %q: %w", ifaceName, err)
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("iface: remove address %q on %q: %w", cidr, ifaceName, err)
	}

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: remove address on %q: not found: %w", ifaceName, err)
	}
	if err := netlink.AddrDel(link, addr); err != nil {
		return fmt.Errorf("iface: remove address %q on %q: %w", cidr, ifaceName, err)
	}
	return nil
}

// SetMTU sets the MTU on the named interface.
func SetMTU(ifaceName string, mtu int) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: set mtu on %q: %w", ifaceName, err)
	}
	if err := validateMTU(mtu); err != nil {
		return fmt.Errorf("iface: set mtu on %q: %w", ifaceName, err)
	}

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: set mtu on %q: not found: %w", ifaceName, err)
	}

	if err := netlink.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("iface: set mtu %d on %q: %w", mtu, ifaceName, err)
	}
	return nil
}

// SetAdminUp brings the named interface administratively up.
func SetAdminUp(ifaceName string) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: set up %q: %w", ifaceName, err)
	}

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: set up %q: not found: %w", ifaceName, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("iface: set up %q: %w", ifaceName, err)
	}
	return nil
}

// SetAdminDown brings the named interface administratively down.
func SetAdminDown(ifaceName string) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: set down %q: %w", ifaceName, err)
	}

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: set down %q: not found: %w", ifaceName, err)
	}
	if err := netlink.LinkSetDown(link); err != nil {
		return fmt.Errorf("iface: set down %q: %w", ifaceName, err)
	}
	return nil
}

// SetMACAddress sets the hardware (MAC) address on the named interface.
// The interface should be down before changing its MAC address on most drivers.
func SetMACAddress(ifaceName, mac string) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: set mac on %q: %w", ifaceName, err)
	}

	hw, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("iface: set mac %q on %q: %w", mac, ifaceName, err)
	}

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: set mac on %q: not found: %w", ifaceName, err)
	}
	if err := netlink.LinkSetHardwareAddr(link, hw); err != nil {
		return fmt.Errorf("iface: set mac %q on %q: %w", mac, ifaceName, err)
	}
	return nil
}

// GetMACAddress returns the current hardware (MAC) address of the named interface.
func GetMACAddress(ifaceName string) (string, error) {
	if err := validateIfaceName(ifaceName); err != nil {
		return "", fmt.Errorf("iface: get mac on %q: %w", ifaceName, err)
	}

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("iface: get mac on %q: not found: %w", ifaceName, err)
	}
	hw := link.Attrs().HardwareAddr
	if len(hw) == 0 {
		return "", nil
	}
	return hw.String(), nil
}

// GetStats returns the current traffic counters for the named interface.
func GetStats(ifaceName string) (*InterfaceStats, error) {
	if err := validateIfaceName(ifaceName); err != nil {
		return nil, fmt.Errorf("iface: get stats on %q: %w", ifaceName, err)
	}

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("iface: get stats on %q: not found: %w", ifaceName, err)
	}

	s := link.Attrs().Statistics
	if s == nil {
		return &InterfaceStats{}, nil
	}

	return &InterfaceStats{
		RxBytes:   s.RxBytes,
		RxPackets: s.RxPackets,
		RxErrors:  s.RxErrors,
		RxDropped: s.RxDropped,
		TxBytes:   s.TxBytes,
		TxPackets: s.TxPackets,
		TxErrors:  s.TxErrors,
		TxDropped: s.TxDropped,
	}, nil
}
