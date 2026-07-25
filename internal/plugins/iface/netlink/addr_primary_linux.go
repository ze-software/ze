// Design: docs/features/interfaces.md -- Interface management via netlink
// Overview: addr_primary.go -- the OS-independent primary/secondary policy
// Related: manage_linux.go -- RemoveAddress drives this remover

//go:build linux

package ifacenetlink

import (
	"fmt"
	"net/netip"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// netlinkAddrRemover is the kernel-backed addrRemover. It carries the already
// parsed *netlink.Addr so the delete uses exactly the address the caller
// validated, rather than re-parsing a rendered prefix.
type netlinkAddrRemover struct {
	link netlink.Link
	addr *netlink.Addr
}

// List returns every address on dev, both families. flushedByDelete filters to
// IPv4 itself; listing both keeps the snapshot an honest picture of the device.
func (r *netlinkAddrRemover) List(dev string) ([]deviceAddress, error) {
	list, err := netlink.AddrList(r.link, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("iface: remove address on %q: list addresses: %w", dev, err)
	}
	existing := make([]deviceAddress, 0, len(list))
	for i := range list {
		current, ok := toDeviceAddress(&list[i])
		if !ok {
			continue
		}
		existing = append(existing, current)
	}
	return existing, nil
}

func (r *netlinkAddrRemover) Delete(dev string, target deviceAddress) error {
	if err := netlink.AddrDel(r.link, r.addr); err != nil {
		return fmt.Errorf("iface: remove address %q on %q: %w", target.Prefix, dev, err)
	}
	return nil
}

// toDeviceAddress converts a netlink address into the OS-independent shape the
// primary/secondary policy works on. Reports false for an address whose IP or
// mask cannot be represented as a netip.Prefix.
//
// Secondary is best-effort by construction and is only ever trusted as a
// POSITIVE signal (see flushedByDelete). Two reasons it cannot be trusted as a
// negative one:
//
//   - netlink fills Flags only from the IFA_FLAGS attribute, never from the
//     ifa_flags header byte, so an address whose attribute did not arrive reads
//     as not-secondary.
//   - IFA_F_SECONDARY and IFA_F_TEMPORARY are the SAME bit (0x1,
//     vendor/golang.org/x/sys/unix/zerrors_linux.go). On IPv6 it therefore means
//     "RFC 4941 temporary address", something else entirely. Only the Is4 guards
//     in flushedByDelete keep that from being misread; do not relax them.
//
// P2P addresses (AddAddressP2P) arrive with the local address under a forced
// /32 mask, so they are modeled as host routes and never match a wider
// sibling. That under-reports in theory; a PPP device carries one address, so
// there is no sibling to lose.
func toDeviceAddress(addr *netlink.Addr) (deviceAddress, bool) {
	if addr == nil || addr.IPNet == nil {
		return deviceAddress{}, false
	}
	ip, ok := netip.AddrFromSlice(addr.IP)
	if !ok {
		return deviceAddress{}, false
	}
	bits, size := addr.Mask.Size()
	if size == 0 {
		// A nil or non-contiguous mask. PrefixFrom would happily produce a
		// valid-looking /0 that matches every address on the device, so reject
		// it rather than let a zero value pass as an answer.
		return deviceAddress{}, false
	}
	prefix := netip.PrefixFrom(ip.Unmap(), bits)
	if !prefix.IsValid() {
		return deviceAddress{}, false
	}
	return deviceAddress{
		Prefix:    prefix,
		Secondary: addr.Flags&unix.IFA_F_SECONDARY != 0,
	}, true
}
