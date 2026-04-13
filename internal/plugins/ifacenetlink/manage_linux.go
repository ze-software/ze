// Design: docs/features/interfaces.md -- Interface management via netlink
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
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

func validateVLANID(id int) error {
	if id < minVLANID || id > maxVLANID {
		return fmt.Errorf("iface: vlan id %d not in [%d, %d]", id, minVLANID, maxVLANID)
	}
	return nil
}

func validateMTU(mtu int) error {
	if mtu < minMTU || mtu > maxMTU {
		return fmt.Errorf("iface: mtu %d not in [%d, %d]", mtu, minMTU, maxMTU)
	}
	return nil
}

func (b *netlinkBackend) CreateDummy(name string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create dummy %q: %w", name, err)
	}
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create dummy %q: %w", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		_ = netlink.LinkDel(link)
		return fmt.Errorf("iface: set up dummy %q: %w", name, err)
	}
	return nil
}

func (b *netlinkBackend) CreateVeth(name, peerName string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create veth %q: %w", name, err)
	}
	if err := iface.ValidateIfaceName(peerName); err != nil {
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
		_ = netlink.LinkDel(link)
		return fmt.Errorf("iface: set up veth %q: %w", name, err)
	}
	peer, err := netlink.LinkByName(peerName)
	if err != nil {
		_ = netlink.LinkDel(link)
		return fmt.Errorf("iface: lookup veth peer %q: %w", peerName, err)
	}
	if err := netlink.LinkSetUp(peer); err != nil {
		_ = netlink.LinkDel(link)
		return fmt.Errorf("iface: set up veth peer %q: %w", peerName, err)
	}
	return nil
}

func (b *netlinkBackend) CreateBridge(name string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create bridge %q: %w", name, err)
	}
	link := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: name}}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create bridge %q: %w", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		_ = netlink.LinkDel(link)
		return fmt.Errorf("iface: set up bridge %q: %w", name, err)
	}
	return nil
}

func (b *netlinkBackend) CreateVLAN(parentName string, vlanID int) error {
	if err := iface.ValidateIfaceName(parentName); err != nil {
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
	if err := iface.ValidateIfaceName(vlanName); err != nil {
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
		_ = netlink.LinkDel(vlan)
		return fmt.Errorf("iface: set up vlan %q: %w", vlanName, err)
	}
	return nil
}

func (b *netlinkBackend) DeleteInterface(name string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
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

func (b *netlinkBackend) AddAddress(ifaceName, cidr string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
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

func (b *netlinkBackend) RemoveAddress(ifaceName, cidr string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
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

func (b *netlinkBackend) ReplaceAddressWithLifetime(ifaceName, cidr string, validLft, preferredLft int) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: replace address on %q: %w", ifaceName, err)
	}
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("iface: replace address %q on %q: %w", cidr, ifaceName, err)
	}
	addr.ValidLft = validLft
	addr.PreferedLft = preferredLft
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: replace address on %q: not found: %w", ifaceName, err)
	}
	if err := netlink.AddrReplace(link, addr); err != nil {
		return fmt.Errorf("iface: replace address %q on %q: %w", cidr, ifaceName, err)
	}
	return nil
}

func (b *netlinkBackend) AddRoute(ifaceName, destCIDR, gateway string, metric int) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: add route on %q: %w", ifaceName, err)
	}
	dst, err := netlink.ParseIPNet(destCIDR)
	if err != nil {
		return fmt.Errorf("iface: add route dest %q: %w", destCIDR, err)
	}
	gw := net.ParseIP(gateway)
	if gw == nil {
		return fmt.Errorf("iface: add route gateway %q: invalid IP", gateway)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: add route on %q: not found: %w", ifaceName, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Gw:        gw,
		Priority:  metric,
	}
	if err := netlink.RouteReplace(route); err != nil {
		return fmt.Errorf("iface: add route %s via %s on %q (metric %d): %w", destCIDR, gateway, ifaceName, metric, err)
	}
	return nil
}

func (b *netlinkBackend) RemoveRoute(ifaceName, destCIDR, gateway string, metric int) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: remove route on %q: %w", ifaceName, err)
	}
	dst, err := netlink.ParseIPNet(destCIDR)
	if err != nil {
		return fmt.Errorf("iface: remove route dest %q: %w", destCIDR, err)
	}
	gw := net.ParseIP(gateway)
	if gw == nil {
		return fmt.Errorf("iface: remove route gateway %q: invalid IP", gateway)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: remove route on %q: not found: %w", ifaceName, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Gw:        gw,
		Priority:  metric,
	}
	if err := netlink.RouteDel(route); err != nil {
		// ESRCH (no such route) is expected when cleaning up after a
		// failed install or double-remove. Not an error on the teardown path.
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return fmt.Errorf("iface: remove route %s via %s metric %d on %q: %w", destCIDR, gateway, metric, ifaceName, err)
	}
	return nil
}

func (b *netlinkBackend) SetMTU(ifaceName string, mtu int) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
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

func (b *netlinkBackend) SetAdminUp(ifaceName string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
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

func (b *netlinkBackend) SetAdminDown(ifaceName string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
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

func (b *netlinkBackend) SetMACAddress(ifaceName, mac string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
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

func (b *netlinkBackend) GetMACAddress(ifaceName string) (string, error) {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
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

func (b *netlinkBackend) GetStats(ifaceName string) (*iface.InterfaceStats, error) {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return nil, fmt.Errorf("iface: get stats on %q: %w", ifaceName, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("iface: get stats on %q: not found: %w", ifaceName, err)
	}
	s := link.Attrs().Statistics
	if s == nil {
		return &iface.InterfaceStats{}, nil
	}
	return &iface.InterfaceStats{
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
