// Design: plan/spec-iface-0-umbrella.md — Interface management via netlink
// Overview: iface.go — shared types and topic constants

package iface

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

// Interface name length limits (Linux kernel IFNAMSIZ = 16, including NUL).
const (
	minIfaceNameLen = 1
	maxIfaceNameLen = 15
)

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

// validateIfaceName checks that name is a valid Linux interface name.
func validateIfaceName(name string) error {
	n := len(name)
	if n < minIfaceNameLen || n > maxIfaceNameLen {
		return fmt.Errorf("iface: name %q length %d not in [%d, %d]",
			name, n, minIfaceNameLen, maxIfaceNameLen)
	}
	return nil
}

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
		return fmt.Errorf("iface: set up veth %q: %w", name, err)
	}

	peer, err := netlink.LinkByName(peerName)
	if err != nil {
		return fmt.Errorf("iface: lookup veth peer %q: %w", peerName, err)
	}
	if err := netlink.LinkSetUp(peer); err != nil {
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
