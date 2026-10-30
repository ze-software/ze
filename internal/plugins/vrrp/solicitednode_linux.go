//go:build linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the Active router's macvlan hears its addresses' solicitations before it holds them
// Related: instance.go -- doInstallVIPs and doRemoveVIPs own a membership's lifetime
// Related: solicitednode_other.go -- the no-op off Linux

package vrrp

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"slices"
	"sync"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// solicitedNodeMembers maps an instance owner to the packet socket whose
// link-layer memberships hold its IPv6 virtual addresses' Solicited-Node groups
// on its virtual-MAC device. Closing the socket drops every membership it
// holds, so the socket is the membership's whole lifetime. Guarded by
// solicitedNodeMu: instances run on their own workers.
var (
	solicitedNodeMu      sync.Mutex
	solicitedNodeMembers = map[string]int{}
)

// joinSolicitedNode makes dev's receive filter admit the Solicited-Node group of
// every IPv6 address in vips, and returns once the filter carries them.
//
// The kernel joins an address's group itself, but only when the address is
// added, and the iface reconcile adds it after the promotion has returned
// (iface.RegisterOwnedAddresses is asynchronous). Until then a Neighbor
// Solicitation for the address reaches a macvlan whose filter drops it. The
// membership here is link-layer only (PACKET_MR_MULTICAST): it sends no MLD
// report and creates no IPv6 state, so the address stays the kernel's to answer
// for once it exists.
//
// PACKET_ADD_MEMBERSHIP adds the group to dev's multicast list at once, but the
// filter update is deferred: a macvlan has ndo_change_rx_flags, so
// __dev_set_rx_mode queues it to the netdev work queue (Linux 7.2,
// net/core/dev_addr_lists.c). syncReceiveFilter runs it before this returns.
//
// A second call for the same owner replaces the first membership, opening the
// new one before closing the old, so the groups common to both never leave the
// filter. A vips holding no IPv6 address leaves any membership the owner holds.
// The caller MUST call leaveSolicitedNode when the instance stops being Active.
// Safe for concurrent use.
//
// RFC 9568 Section 6.4.2: "Compute and join the Solicited-Node multicast address [RFC4291] for the IPv6 address(es) associated with the Virtual Router."
func joinSolicitedNode(instanceOwner, dev string, vips []netip.Addr) error {
	groups := solicitedNodeMACs(vips)
	if len(groups) == 0 {
		return leaveSolicitedNode(instanceOwner)
	}
	link, err := netlink.LinkByName(dev)
	if err != nil {
		return fmt.Errorf("join the Solicited-Node groups of the virtual addresses on %s: the device is not in the kernel (%w); the iface reconcile creates it, check `ip link show %s`", dev, err, dev)
	}
	fd, err := openSolicitedNodeSocket(link.Attrs().Index, groups)
	if err != nil {
		return fmt.Errorf("join the Solicited-Node groups of the virtual addresses on %s: %w", dev, err)
	}

	solicitedNodeMu.Lock()
	previous, held := solicitedNodeMembers[instanceOwner]
	solicitedNodeMembers[instanceOwner] = fd
	solicitedNodeMu.Unlock()

	var closeErr error
	if held {
		closeErr = closeSolicitedNodeSocket(previous)
	}
	syncErr := syncReceiveFilter(link, netlink.LinkByIndex, sendNoopNewLink)
	return errors.Join(syncErr, closeErr)
}

// leaveSolicitedNode drops the memberships joinSolicitedNode took for
// instanceOwner. It MUST be called once the instance stops being Active: a
// Backup does not answer for the addresses (RFC 9568 Section 6.4.2), so it has
// no reason to receive their solicitations. The macvlan keeps receiving a group
// while the kernel still holds the address, which the iface reconcile removes.
// Safe for concurrent use.
func leaveSolicitedNode(instanceOwner string) error {
	solicitedNodeMu.Lock()
	fd, held := solicitedNodeMembers[instanceOwner]
	delete(solicitedNodeMembers, instanceOwner)
	solicitedNodeMu.Unlock()

	// refuse-by-default: guards solicitedNodeMembers -- an instance that never became Active, or an IPv4 one, holds no membership, and leaving none is the answer
	if !held {
		return nil
	}
	return closeSolicitedNodeSocket(fd)
}

// solicitedNodeMACs returns the Ethernet multicast address of the
// Solicited-Node group of each IPv6 address in vips, in order. IPv4 addresses
// have no such group and are skipped.
//
// RFC 4291 Section 2.7.1: "A Solicited-Node multicast address is formed by taking the low-order 24 bits of an address (unicast or anycast) and appending those bits to the prefix FF02:0:0:0:0:1:FF00::/104 resulting in a multicast address in the range FF02:0:0:0:0:1:FF00:0000 to FF02:0:0:0:0:1:FFFF:FFFF"
// RFC 2464 Section 7: "An IPv6 packet with a multicast destination address DST, consisting of the sixteen octets DST[1] through DST[16], is transmitted to the Ethernet multicast address whose first two octets are the value 3333 hexadecimal and whose last four octets are the last four octets of DST."
func solicitedNodeMACs(vips []netip.Addr) [][6]byte {
	var groups [][6]byte
	for _, vip := range vips {
		if !vip.Is6() {
			continue
		}
		if vip.Is4In6() {
			continue
		}
		octets := vip.As16()
		groups = append(groups, [6]byte{0x33, 0x33, 0xff, octets[13], octets[14], octets[15]})
	}
	return groups
}

// openSolicitedNodeSocket opens a packet socket that receives nothing
// (protocol 0 registers no packet handler) and adds each group to the device
// at ifindex as a link-layer multicast membership.
func openSolicitedNodeSocket(ifindex int, groups [][6]byte) (int, error) {
	if ifindex <= 0 {
		return -1, fmt.Errorf("the kernel reported interface index %d, which no device holds", ifindex)
	}
	if ifindex > math.MaxInt32 {
		return -1, fmt.Errorf("the kernel reported interface index %d, beyond the 32-bit range a membership request carries", ifindex)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return -1, fmt.Errorf("open a packet socket: %w; ze needs CAP_NET_RAW", err)
	}
	for _, group := range groups {
		mreq := unix.PacketMreq{Ifindex: int32(ifindex), Type: unix.PACKET_MR_MULTICAST, Alen: 6}
		copy(mreq.Address[:], group[:])
		if err := unix.SetsockoptPacketMreq(fd, unix.SOL_PACKET, unix.PACKET_ADD_MEMBERSHIP, &mreq); err != nil {
			closeErr := closeSolicitedNodeSocket(fd)
			return -1, errors.Join(fmt.Errorf("add link-layer multicast membership %x: %w", group[:], err), closeErr)
		}
	}
	return fd, nil
}

// closeSolicitedNodeSocket closes a membership socket, which drops its groups.
func closeSolicitedNodeSocket(fd int) error {
	if err := unix.Close(fd); err != nil {
		return fmt.Errorf("close the Solicited-Node membership socket (descriptor %d): %w; Linux frees the descriptor even when close reports an error, and the groups go with the last reference to the socket, which `cat /proc/net/dev_mcast` shows", fd, err)
	}
	return nil
}

// receiveFilterDepthMax bounds the walk syncReceiveFilter takes down a device
// stack. The kernel refuses to stack a device on another when the stack would
// grow past MAX_NEST_DEV devices (`dev->lower_level + upper_dev->upper_level >
// MAX_NEST_DEV` in __netdev_upper_dev_link, net/core/dev.c), and MAX_NEST_DEV is
// 8 in include/linux/netdevice.h of the kernel.org stable tag v7.2.9. The
// macvlan and VLAN devices the walk follows register as upper devices, so the
// deepest stack the kernel builds of them is eight devices: the walk syncs all
// eight, and the eighth ends it, because it reports no link, its peer, or a
// link into another namespace. Only a ninth device means the devices changed
// under the walk.
const receiveFilterDepthMax = 8

// syncReceiveFilter makes the receive filters of link and of every device under
// it current before it returns, by sending each one RTM_NEWLINK that changes
// nothing. do_setlink ends in netif_rx_mode_sync, which runs a queued filter
// update inline under rtnl_lock (Linux 7.2, net/core/rtnetlink.c). The walk
// goes top down, following IFLA_LINK: each device's update syncs its groups
// onto the device below (macvlan_set_mac_lists for the macvlan,
// vlan_dev_set_rx_mode for a VLAN), whose own update is then queued in turn, so
// a macvlan on a VLAN on a VLAN needs three. It ends after a device that
// reports no link, after a device whose link is its peer rather than a device
// under it (linkIsPeer), in whichever namespace the peer is, and after a device
// it has already synced. A device under the stack in another namespace is an
// error: a request from this namespace cannot reach it, so its filter catches
// up only when the kernel's queued update runs.
//
// lookup fetches the device at an interface index and sendNoop sends the
// no-op: netlink.LinkByIndex and sendNoopNewLink in production, a fake device
// chain in a test.
func syncReceiveFilter(link netlink.Link, lookup func(index int) (netlink.Link, error), sendNoop func(index int) error) error {
	top := link.Attrs()
	if top.ParentIndex <= 0 {
		return fmt.Errorf("apply the receive filter of the parent of %s: the kernel reports no parent, so it is not the macvlan ze creates; check `ip -d link show %s`", top.Name, top.Name)
	}
	current := link
	synced := make([]int, 0, receiveFilterDepthMax)
	for range receiveFilterDepthMax {
		attrs := current.Attrs()
		if err := sendNoop(attrs.Index); err != nil {
			return fmt.Errorf("apply the receive filter of %s (index %d), under %s: %w", attrs.Name, attrs.Index, top.Name, err)
		}
		synced = append(synced, attrs.Index)
		if attrs.ParentIndex <= 0 {
			return nil
		}
		if linkIsPeer(current) {
			return nil
		}
		if attrs.NetNsID >= 0 {
			return fmt.Errorf("apply the receive filter under %s: the device below %s is in another network namespace (id %d), which a request from this namespace cannot reach, so its filter admits the groups only when the kernel's queued update runs; check `ip -d link show %s`", top.Name, attrs.Name, attrs.NetNsID, attrs.Name)
		}
		if slices.Contains(synced, attrs.ParentIndex) {
			return nil
		}
		lower, err := lookup(attrs.ParentIndex)
		if err != nil {
			return fmt.Errorf("apply the receive filter of the device below %s (index %d), under %s: %w; check `ip -d link show %s`", attrs.Name, attrs.ParentIndex, top.Name, err, attrs.Name)
		}
		current = lower
	}
	return fmt.Errorf("apply the receive filter under %s: the device stack is deeper than %d, which the kernel does not build, so the devices changed during the walk; check `ip -d link show`", top.Name, receiveFilterDepthMax)
}

// linkIsPeer reports whether link is one end of an Ethernet device pair, whose
// IFLA_LINK names the other end rather than a device under it: veth_get_iflink
// (drivers/net/veth.c) and netkit_get_iflink (drivers/net/netkit.c) both return
// the peer's index in the kernel.org stable tag v7.2.9. The peer receives what
// this end sends and keeps a receive filter of its own, which this end's groups
// never reach, so the walk has nothing to sync past this end.
func linkIsPeer(link netlink.Link) bool {
	switch link.(type) {
	case *netlink.Veth, *netlink.Netkit:
		return true
	}
	return false
}

// sendNoopNewLink sends RTM_NEWLINK for ifindex with no attribute and no flag
// change, and waits for the kernel's acknowledgement.
func sendNoopNewLink(ifindex int) error {
	if ifindex > math.MaxInt32 {
		return fmt.Errorf("interface index %d is beyond the 32-bit range RTM_NEWLINK carries", ifindex)
	}
	req := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_ACK)
	msg := nl.NewIfInfomsg(unix.AF_UNSPEC)
	msg.Index = int32(ifindex)
	req.AddData(msg)
	if _, err := req.Execute(unix.NETLINK_ROUTE, 0); err != nil {
		return fmt.Errorf("RTM_NEWLINK: %w", err)
	}
	return nil
}
