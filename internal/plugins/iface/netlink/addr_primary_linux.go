// Design: docs/features/interfaces.md -- Interface management via netlink
// Overview: addr_primary.go -- the OS-independent primary/secondary policy
// Related: manage_linux.go -- RemoveAddress drives this remover

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// errP2PPairAbsent is produced only after a complete, successful address dump
// proves the exact requested pair absent. A failed lookup is never absence.
var errP2PPairAbsent = errors.New("point-to-point address pair is not installed")

// netlinkAddrRemover is the kernel-backed addrRemover. List resolves the
// caller's local CIDR to its installed address, including the peer metadata
// needed by AddrDel to match a point-to-point address.
type netlinkAddrRemover struct {
	link      netlink.Link
	addr      *netlink.Addr
	exactPair bool
}

// List returns every address on dev, both families. flushedByDelete filters to
// IPv4 itself; listing both keeps the snapshot an honest picture of the device.
func (r *netlinkAddrRemover) List(dev string, target *deviceAddress) ([]deviceAddress, error) {
	list, err := netlink.AddrList(r.link, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("iface: remove address on %q: list addresses: %w", dev, err)
	}
	return r.resolve(dev, list, target)
}

// resolve binds Delete and the cascade guard to the same selected identity.
// The caller MUST supply the complete address dump from List.
func (r *netlinkAddrRemover) resolve(dev string, list []netlink.Addr, target *deviceAddress) ([]deviceAddress, error) {
	resolved, existing, err := resolveAddressRemoval(r.addr, list, r.exactPair)
	if err != nil {
		return nil, fmt.Errorf("iface: remove address on %q: %w", dev, err)
	}
	identity, ok := toDeviceAddress(resolved)
	if !ok {
		return nil, fmt.Errorf("iface: remove address on %q: selected address is not representable", dev)
	}
	*target = identity
	r.addr = resolved
	return existing, nil
}

// resolveAddressRemoval MUST resolve an exact pair before its caller can delete.
// Linux inet_rtm_deladdr matches peer subnets, not exact peer addresses, so an
// absent pair MUST NOT be sent to AddrDel. Generic local-only removal retains
// its existing lookup contract. Returned addresses refer to the owned dump.
func resolveAddressRemoval(request *netlink.Addr, list []netlink.Addr, exactPair bool) (*netlink.Addr, []deviceAddress, error) {
	target, ok := toDeviceAddress(request)
	if !ok {
		return nil, nil, fmt.Errorf("requested address is not representable")
	}
	existing := make([]deviceAddress, 0, len(list))
	var resolved *netlink.Addr
	for i := range list {
		current, ok := toDeviceAddress(&list[i])
		if !ok {
			return nil, nil, fmt.Errorf("installed address is not representable")
		}
		existing = append(existing, current)
		if resolved != nil {
			continue
		}
		if request.Label != "" {
			if request.Label != list[i].Label {
				continue
			}
		}
		if exactPair {
			if current.Prefix != target.Prefix {
				continue
			}
			if current.Peer != target.Peer {
				continue
			}
		} else if !request.Equal(list[i]) {
			continue
		}
		resolved = &list[i]
	}
	if resolved != nil {
		return resolved, existing, nil
	}
	if exactPair {
		return nil, nil, fmt.Errorf("local %s peer %s: %w", target.Prefix, target.Peer, errP2PPairAbsent)
	}
	return request, existing, nil
}

func (r *netlinkAddrRemover) Delete(dev string, target deviceAddress) error {
	if err := netlink.AddrDel(r.link, r.addr); err != nil {
		if r.exactPair {
			if target.Prefix.Addr().Is4() {
				// Linux inet_rtm_deladdr returns EADDRNOTAVAIL when the
				// address disappeared after List. ENOENT is not this proof.
				if errors.Is(err, unix.EADDRNOTAVAIL) {
					return nil
				}
			}
		}
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
// P2P addresses arrive with a host mask on the local side. Preserve the peer
// prefix separately: Linux uses that prefix for primary/secondary grouping.
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
	var peer netip.Prefix
	if addr.Peer != nil {
		ip, ok := netip.AddrFromSlice(addr.Peer.IP)
		if !ok {
			return deviceAddress{}, false
		}
		bits, size := addr.Peer.Mask.Size()
		if size == 0 {
			return deviceAddress{}, false
		}
		peer = netip.PrefixFrom(ip.Unmap(), bits)
		if !peer.IsValid() {
			return deviceAddress{}, false
		}
		if peer.Addr().Is4() != prefix.Addr().Is4() {
			return deviceAddress{}, false
		}
		// Match parseAddr: IPv4 equal endpoints collapse to the peer prefix.
		// A distinct peer (and IPv6 IFA_LOCAL) gives the local a host mask.
		prefix = netip.PrefixFrom(prefix.Addr(), prefix.Addr().BitLen())
		if prefix.Addr().Is4() {
			if prefix.Addr() == peer.Addr() {
				prefix = peer
				peer = netip.Prefix{}
			}
		}
	}
	return deviceAddress{
		Prefix:    prefix,
		Peer:      peer,
		Secondary: addr.Flags&unix.IFA_F_SECONDARY != 0,
	}, true
}
