// Design: docs/features/interfaces.md -- Interface listing via netlink
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

func (b *netlinkBackend) ListInterfaces() ([]iface.InterfaceInfo, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("iface: list interfaces: %w", err)
	}
	result := make([]iface.InterfaceInfo, 0, len(links))
	for _, link := range links {
		info := linkToInfo(link)
		info.Addresses = addrList(link)
		result = append(result, info)
	}
	return result, nil
}

func (b *netlinkBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	if err := iface.ValidateIfaceName(name); err != nil {
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
		info.Stats = &iface.InterfaceStats{
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

func linkToInfo(link netlink.Link) iface.InterfaceInfo {
	attrs := link.Attrs()
	state := "down"
	if attrs.OperState == netlink.OperUp {
		state = "up"
	} else if attrs.OperState == netlink.OperUnknown && (attrs.Flags&net.FlagUp != 0) {
		state = "up"
	}
	info := iface.InterfaceInfo{
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

func addrList(link netlink.Link) []iface.AddrInfo {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		loggerPtr.Load().Warn("failed to list addresses", "interface", link.Attrs().Name, "err", err)
		return nil
	}
	result := make([]iface.AddrInfo, 0, len(addrs))
	for _, a := range addrs {
		fam := "ipv4" //nolint:goconst // AFI label; see ifacenetlink.go for siblings
		if a.IP.To4() == nil {
			fam = "ipv6" //nolint:goconst // AFI label; see ifacenetlink.go for siblings
		}
		ones, _ := a.Mask.Size()
		result = append(result, iface.AddrInfo{
			Address:      a.IP.String(),
			PrefixLength: ones,
			Family:       fam,
		})
	}
	return result
}
