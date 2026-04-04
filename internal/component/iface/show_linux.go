// Design: docs/features/interfaces.md — Interface listing via netlink
// Overview: iface.go — shared types and topic constants

//go:build linux

package iface

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// ListInterfaces returns all OS network interfaces with their addresses and stats.
func ListInterfaces() ([]InterfaceInfo, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("iface: list interfaces: %w", err)
	}

	result := make([]InterfaceInfo, 0, len(links))
	for _, link := range links {
		info := linkToInfo(link)
		info.Addresses = addrList(link)
		result = append(result, info)
	}
	return result, nil
}

// GetInterface returns info for a single interface by name.
func GetInterface(name string) (*InterfaceInfo, error) {
	if err := validateIfaceName(name); err != nil {
		return nil, err
	}

	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("iface: get %q: %w", name, err)
	}

	info := linkToInfo(link)
	info.Addresses = addrList(link)

	s := link.Attrs().Statistics
	if s != nil {
		info.Stats = &InterfaceStats{
			RxBytes:   s.RxBytes,
			RxPackets: s.RxPackets,
			RxErrors:  s.RxErrors,
			RxDropped: s.RxDropped,
			TxBytes:   s.TxBytes,
			TxPackets: s.TxPackets,
			TxErrors:  s.TxErrors,
			TxDropped: s.TxDropped,
		}
	}

	return &info, nil
}

// linkToInfo converts a netlink.Link to InterfaceInfo.
// State logic matches isLinkUp() in monitor_linux.go: OperUp is definitive,
// OperUnknown with admin IFF_UP is treated as up (virtual interfaces),
// all other states are down.
func linkToInfo(link netlink.Link) InterfaceInfo {
	attrs := link.Attrs()
	state := "down"
	if attrs.OperState == netlink.OperUp {
		state = "up"
	} else if attrs.OperState == netlink.OperUnknown && (attrs.Flags&net.FlagUp != 0) {
		state = "up"
	}

	info := InterfaceInfo{
		Name:  attrs.Name,
		Index: attrs.Index,
		Type:  link.Type(),
		State: state,
		MTU:   attrs.MTU,
	}

	if len(attrs.HardwareAddr) > 0 {
		info.MAC = attrs.HardwareAddr.String()
	}

	if vlan, ok := link.(*netlink.Vlan); ok {
		info.VlanID = vlan.VlanId
		info.ParentIndex = attrs.ParentIndex
	}

	return info
}

// addrList returns all addresses on a link. Logs and returns nil on error.
func addrList(link netlink.Link) []AddrInfo {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		loggerPtr.Load().Warn("failed to list addresses", "interface", link.Attrs().Name, "err", err)
		return nil
	}

	result := make([]AddrInfo, 0, len(addrs))
	for _, a := range addrs {
		family := "ipv4"
		if a.IP.To4() == nil {
			family = "ipv6"
		}
		ones, _ := a.Mask.Size()
		result = append(result, AddrInfo{
			Address:      a.IP.String(),
			PrefixLength: ones,
			Family:       family,
		})
	}
	return result
}
