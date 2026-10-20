// Design: docs/features/interfaces.md -- plugin-owned macvlan devices via netlink
// Overview: ifacenetlink.go -- package hub
// Related: backend_linux.go -- netlinkBackend type and Close()
// Related: tunnel_linux.go -- sibling Create* implementation (LinkAdd + rollback)
// Related: manage_linux.go -- DeleteInterface (used to delete owned macvlans)

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

// CreateMacvlanDevice creates a bridge-mode macvlan netdev on spec.Parent
// carrying spec.MAC, marked with the owned-device alias spec.Alias
// ("ze:owned:<owner>"), and brings it admin-up. The MTU is inherited
// explicitly from the parent (kernels inherit it at create anyway, but setting
// it makes the value deterministic and independent of that behavior).
//
// The device derives no IPv6 link-local of its own: its IPv6 address
// generation mode is set to none before it comes up (setMacvlanAddrGenNone).
//
// Sequence: LinkAdd, LinkSetAlias, addr_gen_mode, LinkSetUp. The vendored netlink lib does
// serialize IFLA_IFALIAS inside LinkAdd's RTM_NEWLINK, but the kernel does NOT
// apply it on create -- QEMU readback proved the alias comes back empty (spec
// A-2 broken; see the spec's Mistake Log). So the ownership alias is asserted
// explicitly with LinkSetAlias right after create; if that fails the device is
// deleted and the call errors, so no UNMARKED device is ever left behind by a
// completed call. A crash exactly between LinkAdd and LinkSetAlias can leave an
// unmarked device carrying a registered name; the reconcile pass adopts and
// re-marks that case (config_apply.go reconcileOwnedDevices drift path). On
// LinkSetUp failure the partial netdev is likewise removed via LinkDel
// (rollback), matching the tunnel/wireguard pattern.
func (b *netlinkBackend) CreateMacvlanDevice(spec iface.MacvlanSpec) error {
	if err := iface.ValidateIfaceName(spec.Name); err != nil {
		return fmt.Errorf("iface: create macvlan %q: %w", spec.Name, err)
	}
	parent, err := netlink.LinkByName(spec.Parent)
	if err != nil {
		return fmt.Errorf("iface: create macvlan %q: parent %q: %w", spec.Name, spec.Parent, err)
	}
	link, err := buildMacvlanLink(spec, parent.Attrs().Index, parent.Attrs().MTU)
	if err != nil {
		return fmt.Errorf("iface: create macvlan %q: %w", spec.Name, err)
	}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create macvlan %q on %q: %w", spec.Name, spec.Parent, err)
	}
	rollback := func(stage string, cause error) error {
		if delErr := netlink.LinkDel(link); delErr != nil {
			logger().Warn("iface: rollback delete after create failure",
				"name", spec.Name, "kind", "macvlan", "stage", stage, "err", delErr)
		}
		return fmt.Errorf("iface: %s macvlan %q: %w", stage, spec.Name, cause)
	}
	if spec.Alias != "" {
		if err := netlink.LinkSetAlias(link, spec.Alias); err != nil {
			return rollback("set alias on", err)
		}
	}
	// RFC 9568 Section 7.4
	if err := setMacvlanAddrGenNone(link); err != nil {
		return rollback("suppress the IPv6 link-local of", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return rollback("set up", err)
	}
	return nil
}

// addrGenModeNone is IN6_ADDR_GEN_MODE_NONE from include/uapi/linux/if_link.h:
// the kernel generates no IPv6 interface identifier for the device.
const addrGenModeNone = 1

// setMacvlanAddrGenNone stops the kernel deriving an IPv6 link-local for an
// owned macvlan. The device carries a MAC its owner chose, which for VRRP is
// the Virtual Router MAC every router of the group shares, and Linux's default
// mode builds the link-local's Modified EUI-64 identifier from that MAC. The
// owner installs whatever IPv6 address the device needs (VRRP: the configured
// virtual link-local). It MUST run before LinkSetUp, because the kernel adds
// the link-local when the device comes up and a later mode change does not
// remove it.
//
// EAFNOSUPPORT means the kernel has no IPv6 stack (booted ipv6.disable=1), so
// no identifier can be derived and there is nothing to suppress.
func setMacvlanAddrGenNone(link netlink.Link) error {
	// RFC 9568 Section 7.4: "The Virtual Router MAC MUST NOT be used for the Net_Iface parameter used in the Interface Identifier (IID) derivation algorithms in [RFC7217] and [RFC8981]."
	// RFC 5798 Section 7.4: "They MUST NOT use the virtual router MAC address to create the Modified Extended Unique Identifier (EUI)-64 identifiers."
	err := netlink.LinkSetIP6AddrGenMode(link, addrGenModeNone)
	if errors.Is(err, unix.EAFNOSUPPORT) {
		return nil
	}
	return err
}

// buildMacvlanLink is the pure spec -> netlink.Macvlan translation: the delivery
// mode from spec.Mode, parent index and MTU carried, and the MAC + ownership
// alias placed in LinkAttrs. LinkAdd serializes IFLA_ADDRESS + IFLA_IFALIAS in
// one message, but the kernel only honors the MAC at create -- the alias is
// ignored (QEMU readback evidence, spec A-2), so CreateMacvlanDevice
// re-asserts it with LinkSetAlias; keeping it in LinkAttrs is harmless and
// documents intent for kernels that do apply it. No netlink calls here, so it
// is unit-testable without a kernel. Returns an error only when the MAC does
// not parse (defense in depth; the registry validated the caller spec already).
func buildMacvlanLink(spec iface.MacvlanSpec, parentIndex, mtu int) (*netlink.Macvlan, error) {
	mac, err := net.ParseMAC(spec.MAC)
	if err != nil {
		return nil, fmt.Errorf("mac %q: %w", spec.MAC, err)
	}
	attrs := netlink.NewLinkAttrs()
	attrs.Name = spec.Name
	attrs.ParentIndex = parentIndex
	attrs.HardwareAddr = mac
	attrs.Alias = spec.Alias
	attrs.MTU = mtu
	return &netlink.Macvlan{LinkAttrs: attrs, Mode: macvlanMode(spec.Mode)}, nil
}

// macvlanMode maps the iface delivery mode to the netlink constant. Bridge is
// the zero value / default.
func macvlanMode(mode iface.MacvlanMode) netlink.MacvlanMode {
	if mode == iface.MacvlanModePrivate {
		return netlink.MACVLAN_MODE_PRIVATE
	}
	return netlink.MACVLAN_MODE_BRIDGE
}
