// Design: docs/architecture/cli/plugin-modes.md -- provision interface auto-config

//go:build linux

package provision

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

func ensureAddress(ifaceName, cidr string) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("interface %s: %w", ifaceName, err)
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("parse address %s: %w", cidr, err)
	}

	if link.Attrs().Flags&net.FlagUp == 0 {
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("bring up %s: %w", ifaceName, err)
		}
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("add %s to %s: %w", cidr, ifaceName, err)
	}

	return nil
}

func removeAddress(ifaceName, cidr string) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("interface %s: %w", ifaceName, err)
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("parse address %s: %w", cidr, err)
	}

	if err := netlink.AddrDel(link, addr); err != nil {
		return fmt.Errorf("remove %s from %s: %w", cidr, ifaceName, err)
	}

	return nil
}
