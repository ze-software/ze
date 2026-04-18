// Design: docs/features/interfaces.md -- Tunnel netdev creation via netlink
// Overview: ifacenetlink.go -- package hub
// Related: backend_linux.go -- netlinkBackend type and Close()
// Related: manage_linux.go -- sibling Create* methods (Dummy, Veth, Bridge, VLAN)

//go:build linux

package ifacenetlink

import (
	"fmt"
	"math"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// CreateTunnel creates a tunnel netdev for one of the eight supported kinds.
//
// Kind dispatch:
//   - gre, ip6gre               -> netlink.Gretun (kind switches on Local v4/v6)
//   - gretap, ip6gretap         -> netlink.Gretap
//   - ipip                      -> netlink.Iptun
//   - sit                       -> netlink.Sittun
//   - ip6tnl                    -> netlink.Ip6tnl with Proto=IPPROTO_IPV6 (41)
//   - ipip6                     -> netlink.Ip6tnl with Proto=IPPROTO_IPIP (4)
//
// GRE key (RFC 2890): the vendored netlink lib auto-sets the GRE_KEY flag bit
// in IFlags/OFlags when IKey/OKey are non-zero, so we set both fields to the
// same uint32 to obtain symmetric key behavior. Key value 0 cannot be set
// because the lib treats 0 as "no key" -- documented in the YANG leaf and the
// spec deferral list.
//
// On LinkSetUp failure after a successful LinkAdd, the partial netdev is
// removed via LinkDel; failure of the rollback delete is logged so the user
// has a breadcrumb if a stale netdev is left behind.
func (b *netlinkBackend) CreateTunnel(spec iface.TunnelSpec) error {
	if err := iface.ValidateIfaceName(spec.Name); err != nil {
		return fmt.Errorf("iface: create tunnel %q: %w", spec.Name, err)
	}

	link, err := buildTunnelLink(spec)
	if err != nil {
		return fmt.Errorf("iface: create tunnel %q: %w", spec.Name, err)
	}

	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create tunnel %q (kind %s): %w", spec.Name, spec.Kind, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		if delErr := netlink.LinkDel(link); delErr != nil {
			loggerPtr.Load().Warn("iface: rollback delete after set-up failure",
				"name", spec.Name, "kind", spec.Kind, "err", delErr)
		}
		return fmt.Errorf("iface: set up tunnel %q: %w", spec.Name, err)
	}
	return nil
}

// buildTunnelLink converts a TunnelSpec into the matching vishvananda/netlink
// Go type. Returns an error for unknown kinds, malformed addresses, or
// kind/address-family mismatches (e.g. v6 address on a gre case).
func buildTunnelLink(spec iface.TunnelSpec) (netlink.Link, error) {
	local, err := parseTunnelLocal(spec)
	if err != nil {
		return nil, err
	}
	remote, err := parseTunnelRemote(spec)
	if err != nil {
		return nil, err
	}
	parentIndex, err := resolveLocalInterface(spec.LocalInterface)
	if err != nil {
		return nil, err
	}

	if greBuilders[spec.Kind] != nil {
		return greBuilders[spec.Kind](spec, spec.Name, local, remote, parentIndex), nil
	}
	if ipBuilders[spec.Kind] != nil {
		return ipBuilders[spec.Kind](spec, spec.Name, local, remote, parentIndex), nil
	}
	return nil, fmt.Errorf("unsupported tunnel kind %s", spec.Kind)
}

// linkAttrsForName builds a minimal LinkAttrs carrying only the interface
// name. Tunnel-specific OS attributes (mtu, mac) are applied via existing
// Backend methods after CreateTunnel returns.
func linkAttrsForName(name string) netlink.LinkAttrs {
	attrs := netlink.NewLinkAttrs()
	attrs.Name = name
	return attrs
}

// tunnelBuilder constructs a netlink.Link from the resolved TunnelSpec inputs.
type tunnelBuilder func(spec iface.TunnelSpec, name string, local, remote net.IP, parentIndex uint32) netlink.Link

// greBuilders maps the GRE-family kinds to their constructors.
// RFC 2784 Section 2.1 (header), RFC 2890 Section 2.1 (key extension).
var greBuilders = map[iface.TunnelKind]tunnelBuilder{
	iface.TunnelKindGRE:       buildGretun,
	iface.TunnelKindIP6GRE:    buildGretun,
	iface.TunnelKindGRETap:    buildGretap,
	iface.TunnelKindIP6GRETap: buildGretap,
}

// ipBuilders maps the non-GRE kinds to their constructors.
var ipBuilders = map[iface.TunnelKind]tunnelBuilder{
	iface.TunnelKindIPIP:   buildIptun,
	iface.TunnelKindSIT:    buildSittun,
	iface.TunnelKindIP6Tnl: buildIp6tnl,
	iface.TunnelKindIPIP6:  buildIp6tnl,
}

// buildGretun constructs a Gretun (gre or ip6gre) link.
// RFC 2784 Section 2.1: GRE header structure.
// RFC 2890 Section 2.1: Key field; vendored netlink auto-sets the GRE_KEY
// flag bit in IFlags/OFlags when IKey/OKey are non-zero.
func buildGretun(spec iface.TunnelSpec, name string, local, remote net.IP, parentIndex uint32) netlink.Link {
	link := &netlink.Gretun{
		LinkAttrs: linkAttrsForName(name),
		Local:     local,
		Remote:    remote,
		Link:      parentIndex,
	}
	if spec.KeySet {
		link.IKey = spec.Key
		link.OKey = spec.Key
	}
	if spec.TTLSet {
		link.Ttl = spec.TTL
	}
	if spec.TosSet {
		link.Tos = spec.Tos
	}
	if spec.HopLimitSet {
		link.Ttl = spec.HopLimit
	}
	if spec.TClassSet {
		link.Tos = spec.TClass
	}
	if spec.NoPMTUDiscovery {
		link.PMtuDisc = 0
	} else {
		link.PMtuDisc = 1
	}
	return link
}

// buildGretap constructs a Gretap (gretap or ip6gretap) link.
// RFC 2784: GRE base spec; protocol-type 0x6558 indicates Transparent Ethernet
// Bridging which the kernel selects automatically when kind=gretap.
func buildGretap(spec iface.TunnelSpec, name string, local, remote net.IP, parentIndex uint32) netlink.Link {
	link := &netlink.Gretap{
		LinkAttrs: linkAttrsForName(name),
		Local:     local,
		Remote:    remote,
		Link:      parentIndex,
	}
	if spec.KeySet {
		link.IKey = spec.Key
		link.OKey = spec.Key
	}
	if spec.TTLSet {
		link.Ttl = spec.TTL
	}
	if spec.TosSet {
		link.Tos = spec.Tos
	}
	if spec.HopLimitSet {
		link.Ttl = spec.HopLimit
	}
	if spec.TClassSet {
		link.Tos = spec.TClass
	}
	if spec.NoPMTUDiscovery {
		link.PMtuDisc = 0
	} else {
		link.PMtuDisc = 1
	}
	return link
}

// buildIptun constructs an Iptun (ipip) link. RFC 2003: outer IPv4 header
// with Protocol = 4. No GRE header, no key.
func buildIptun(spec iface.TunnelSpec, name string, local, remote net.IP, parentIndex uint32) netlink.Link {
	link := &netlink.Iptun{
		LinkAttrs: linkAttrsForName(name),
		Local:     local,
		Remote:    remote,
		Link:      parentIndex,
		Proto:     syscall.IPPROTO_IPIP,
	}
	if spec.TTLSet {
		link.Ttl = spec.TTL
	}
	if spec.TosSet {
		link.Tos = spec.Tos
	}
	if spec.NoPMTUDiscovery {
		link.PMtuDisc = 0
	} else {
		link.PMtuDisc = 1
	}
	return link
}

// buildSittun constructs a Sittun (sit / 6in4) link.
// RFC 4213 Section 3.5: outer IPv4 header with Protocol = 41 carrying IPv6.
func buildSittun(spec iface.TunnelSpec, name string, local, remote net.IP, parentIndex uint32) netlink.Link {
	link := &netlink.Sittun{
		LinkAttrs: linkAttrsForName(name),
		Local:     local,
		Remote:    remote,
		Link:      parentIndex,
		Proto:     syscall.IPPROTO_IPV6,
	}
	if spec.TTLSet {
		link.Ttl = spec.TTL
	}
	if spec.TosSet {
		link.Tos = spec.Tos
	}
	if spec.NoPMTUDiscovery {
		link.PMtuDisc = 0
	} else {
		link.PMtuDisc = 1
	}
	return link
}

// buildIp6tnl constructs an Ip6tnl link covering both ip6tnl (IPv6-in-IPv6)
// and ipip6 (IPv4-in-IPv6). RFC 2473.
//
// The two cases share the Go type but differ in the Proto field:
//   - ip6tnl -> Proto = IPPROTO_IPV6 (41)
//   - ipip6  -> Proto = IPPROTO_IPIP (4)
func buildIp6tnl(spec iface.TunnelSpec, name string, local, remote net.IP, parentIndex uint32) netlink.Link {
	link := &netlink.Ip6tnl{
		LinkAttrs: linkAttrsForName(name),
		Local:     local,
		Remote:    remote,
		Link:      parentIndex,
	}
	if spec.Kind == iface.TunnelKindIPIP6 {
		link.Proto = syscall.IPPROTO_IPIP
	} else {
		link.Proto = syscall.IPPROTO_IPV6
	}
	if spec.HopLimitSet {
		link.Ttl = spec.HopLimit
	}
	if spec.TClassSet {
		link.Tos = spec.TClass
	}
	if spec.EncapLimitSet {
		link.EncapLimit = spec.EncapLimit
	}
	return link
}

// parseTunnelLocal parses LocalAddress when set, returning nil when no
// local IP is configured (the spec may use a local interface instead, or
// neither for tunnel kinds where the kernel picks the source). Validates
// that the address family matches the kind.
func parseTunnelLocal(spec iface.TunnelSpec) (net.IP, error) {
	if spec.LocalAddress == "" {
		return nil, nil
	}
	ip := net.ParseIP(spec.LocalAddress)
	if ip == nil {
		return nil, fmt.Errorf("local ip %q is not a valid IP address", spec.LocalAddress)
	}
	if err := checkAddressFamily(spec.Kind, ip, "local ip"); err != nil {
		return nil, err
	}
	return ip, nil
}

// parseTunnelRemote parses RemoteAddress (mandatory) and validates the
// address family.
func parseTunnelRemote(spec iface.TunnelSpec) (net.IP, error) {
	if spec.RemoteAddress == "" {
		return nil, fmt.Errorf("remote ip is required for tunnel %q", spec.Name)
	}
	ip := net.ParseIP(spec.RemoteAddress)
	if ip == nil {
		return nil, fmt.Errorf("remote ip %q is not a valid IP address", spec.RemoteAddress)
	}
	if err := checkAddressFamily(spec.Kind, ip, "remote ip"); err != nil {
		return nil, err
	}
	return ip, nil
}

// checkAddressFamily verifies that ip is in the correct family for kind.
// v6-underlay kinds require an IPv6 address; v4-underlay kinds require IPv4.
func checkAddressFamily(kind iface.TunnelKind, ip net.IP, leafName string) error {
	if kind.IsV6Underlay() {
		if ip.To4() != nil {
			return fmt.Errorf("%s for %s tunnel must be IPv6, got IPv4 %q", leafName, kind, ip.String())
		}
		return nil
	}
	if ip.To4() == nil {
		return fmt.Errorf("%s for %s tunnel must be IPv4, got IPv6 %q", leafName, kind, ip.String())
	}
	return nil
}

// resolveLocalInterface looks up the parent interface index when the spec
// uses a local interface instead of a local ip. Returns 0 when the spec
// has no local interface set (ifindex 0 = no parent in netlink). The
// kernel uses uint32 for ifindex; the upper-bound check defends against
// any future change that would let the netlink lib return values too
// large to fit.
func resolveLocalInterface(name string) (uint32, error) {
	if name == "" {
		return 0, nil
	}
	link, err := netlink.LinkByName(name)
	if err != nil {
		return 0, fmt.Errorf("local interface %q: %w", name, err)
	}
	idx := link.Attrs().Index
	if idx < 0 || int64(idx) > int64(math.MaxUint32) {
		return 0, fmt.Errorf("local interface %q: index %d out of uint32 range", name, idx)
	}
	return uint32(idx), nil
}
