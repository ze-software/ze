// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Overview: ifacenetlink.go -- package hub
// Related: owned_bridge_linux.go -- the bridge a link's host end joins

//go:build linux

package ifacenetlink

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

// CreateOwnedLink creates a veth pair with its peer end directly inside
// spec.Namespace and configures both ends.
//
// One RTM_NEWLINK carries the peer's name, hardware address, MTU and target
// namespace (IFLA_NET_NS_FD), so the peer end is never visible in the host
// namespace and needs no temporary name there. The host end is then marked,
// joined to the bridge and set up from the host namespace. The peer end is
// configured through a netlink socket opened in its namespace: the vendored
// library enters the namespace on a locked thread only to open the socket and
// returns the thread at once, so no thread of ze stays in that namespace.
//
// On any failure the host end is deleted, which deletes the pair, so a failed
// call leaves no part of the link.
func (b *netlinkBackend) CreateOwnedLink(spec iface.OwnedLinkSpec) error {
	if !qosMapMu.TryLock() {
		return iface.ErrNativeAccessPending
	}
	defer qosMapMu.Unlock()
	if err := b.nativeVLANs.legacyAdmissionLocked(); err != nil {
		return err
	}
	if err := validateOwnedLinkSpec(spec); err != nil {
		return err
	}
	bridge, err := netlink.LinkByName(spec.Bridge)
	if err != nil {
		return fmt.Errorf("iface: create owned link %q: bridge %q: %w", spec.HostEnd, spec.Bridge, err)
	}
	if bridge.Type() != linkTypeBridge {
		return fmt.Errorf("iface: create owned link %q: %q is a %s, not a bridge", spec.HostEnd, spec.Bridge, bridge.Type())
	}
	attrs := netlink.NewLinkAttrs()
	attrs.Name = spec.HostEnd
	attrs.MTU = spec.MTU
	veth := &netlink.Veth{
		LinkAttrs:        attrs,
		PeerName:         spec.PeerName,
		PeerHardwareAddr: spec.PeerMAC,
		PeerMTU:          uint32(spec.MTU), //nolint:gosec // validateOwnedLinkSpec bounds the MTU to [68, 16000]
		PeerNamespace:    netlink.NsFd(int(spec.Namespace.Fd())),
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("iface: create owned link %q (peer %q): %w", spec.HostEnd, spec.PeerName, err)
	}
	rollback := func(stage string, cause error) error {
		return fmt.Errorf("iface: %s owned link %q: %w", stage, spec.HostEnd, b.rollbackNativeCreate(veth, spec.HostEnd, "veth", stage, cause))
	}
	// The kernel ignores IFLA_IFALIAS in RTM_NEWLINK (Linux limitation
	// LL-RTNL-NEWLINK-NO-ALIAS, docs/architecture/linux-limitations.md).
	if err := netlink.LinkSetAlias(veth, spec.Alias); err != nil {
		return rollback("set alias on", err)
	}
	if err := netlink.LinkSetMaster(veth, bridge); err != nil {
		return rollback("join bridge "+spec.Bridge+" with", err)
	}
	if err := netlink.LinkSetUp(veth); err != nil {
		return rollback("set up", err)
	}
	if err := configureOwnedPeer(spec); err != nil {
		return rollback("configure the peer end of", err)
	}
	return nil
}

// validateOwnedLinkSpec refuses a spec the kernel would refuse later, so no
// pair is made for it.
func validateOwnedLinkSpec(spec iface.OwnedLinkSpec) error {
	if err := iface.ValidateIfaceName(spec.HostEnd); err != nil {
		return fmt.Errorf("create owned link %q: %w", spec.HostEnd, err)
	}
	if err := iface.ValidateIfaceName(spec.PeerName); err != nil {
		return fmt.Errorf("create owned link %q: peer: %w", spec.HostEnd, err)
	}
	if err := validateMTU(spec.MTU); err != nil {
		return fmt.Errorf("create owned link %q: %w", spec.HostEnd, err)
	}
	if spec.Namespace == nil {
		return fmt.Errorf("iface: create owned link %q: no network namespace for the peer end", spec.HostEnd)
	}
	return nil
}

// configureOwnedPeer addresses the peer end, sets it up and installs one
// default route per gateway, in that order: a route through a gateway needs
// the address that reaches it and a link that is up. IPv6 addresses carry
// IFA_F_NODAD, because the owner guarantees their uniqueness and a program
// inside the namespace may bind them at once.
func configureOwnedPeer(spec iface.OwnedLinkSpec) error {
	handle, err := netlink.NewHandleAt(netns.NsHandle(int(spec.Namespace.Fd())), unix.NETLINK_ROUTE)
	if err != nil {
		return fmt.Errorf("open a netlink socket in the peer's namespace: %w", err)
	}
	defer handle.Close()
	peer, err := handle.LinkByName(spec.PeerName)
	if err != nil {
		return fmt.Errorf("find peer %q in its namespace: %w", spec.PeerName, err)
	}
	for _, prefix := range spec.PeerAddresses {
		address := &netlink.Addr{IPNet: ownedPrefixIPNet(prefix)}
		if prefix.Addr().Is6() {
			address.Flags = unix.IFA_F_NODAD
		}
		if err := handle.AddrAdd(peer, address); err != nil {
			return fmt.Errorf("add %s to peer %q: %w", prefix, spec.PeerName, err)
		}
	}
	if err := handle.LinkSetUp(peer); err != nil {
		return fmt.Errorf("set up peer %q: %w", spec.PeerName, err)
	}
	for _, gateway := range spec.PeerGateways {
		route := &netlink.Route{LinkIndex: peer.Attrs().Index, Gw: net.IP(gateway.AsSlice())}
		if err := handle.RouteAdd(route); err != nil {
			return fmt.Errorf("add default route via %s on peer %q: %w", gateway, spec.PeerName, err)
		}
	}
	return nil
}

// ownedPrefixIPNet is prefix as the library's address form: the address with
// its host bits kept, which is the address installed, and the prefix's mask.
func ownedPrefixIPNet(prefix netip.Prefix) *net.IPNet {
	return &net.IPNet{
		IP:   net.IP(prefix.Addr().AsSlice()),
		Mask: net.CIDRMask(prefix.Bits(), prefix.Addr().BitLen()),
	}
}
