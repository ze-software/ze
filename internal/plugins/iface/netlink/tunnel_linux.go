// Design: docs/features/interfaces.md -- Tunnel netdev creation via netlink
// Overview: ifacenetlink.go -- package hub
// Related: backend_linux.go -- netlinkBackend type and Close()
// Related: manage_linux.go -- sibling Create* methods (Dummy, Veth, Bridge, VLAN)
// RFC: rfc/short/rfc2784.md
// RFC: rfc/short/rfc2890.md
// RFC: rfc/short/rfc2003.md
// RFC: rfc/short/rfc2473.md

//go:build linux

package ifacenetlink

import (
	"fmt"
	"math"
	"net"
	"net/netip"
	"syscall"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
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
// GRE key (RFC 2890): the vendored netlink library auto-sets the GRE_KEY flag bit
// in IFlags/OFlags when IKey/OKey are non-zero, so we set both fields to the
// same uint32 to obtain symmetric key behavior. Key value 0 cannot be set
// because the lib treats 0 as "no key" -- documented in the YANG leaf and the
// spec deferral list.
//
// On LinkSetUp failure after a successful LinkAdd, the partial netdev is
// removed via LinkDel; failure of the rollback delete is logged so the user
// has a breadcrumb if a stale netdev is left behind.
func (b *netlinkBackend) CreateTunnel(spec iface.TunnelSpec) error {
	if !qosMapMu.TryLock() {
		return iface.ErrNativeAccessPending
	}
	defer qosMapMu.Unlock()
	if err := b.nativeVLANs.legacyAdmissionLocked(); err != nil {
		return err
	}
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
		return fmt.Errorf("iface: set up tunnel %q: %w", spec.Name, b.rollbackNativeCreate(link, spec.Name, spec.Kind.String(), "set up", err))
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

	if spec.Kind == iface.TunnelKindVxlan {
		return buildVxlan(spec, local, remote)
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
// RFC 2890 Section 2.1: Key field; the vendored netlink library auto-sets the GRE_KEY
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

// buildVxlan constructs a netlink.Vxlan link. VXLAN carries L2 Ethernet frames
// over a UDP/IPv4 underlay keyed by a 24-bit VNI; Group is the unicast (or
// multicast) remote endpoint and the port defaults to the IANA value 4789.
// VNI is validated here as well as in the VPP backend so both halves of the
// new kind reject an out-of-range identifier identically.
func buildVxlan(spec iface.TunnelSpec, local, remote net.IP) (netlink.Link, error) {
	if spec.LocalInterface != "" {
		return nil, fmt.Errorf("vxlan %q: local-interface source not supported; set local ip", spec.Name)
	}
	if !spec.VNISet || spec.VNI == 0 {
		return nil, fmt.Errorf("vxlan %q: vni is required (1..16777215)", spec.Name)
	}
	if spec.VNI > 16777215 {
		return nil, fmt.Errorf("vxlan %q: vni %d out of range (1..16777215)", spec.Name, spec.VNI)
	}
	port := 4789
	if spec.PortSet && spec.Port != 0 {
		port = int(spec.Port)
	}
	return &netlink.Vxlan{
		LinkAttrs: linkAttrsForName(spec.Name),
		VxlanId:   int(spec.VNI),
		SrcAddr:   local,
		Group:     remote,
		Port:      port,
	}, nil
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
	// RFC 4213 Section 3.6. Config verify already refused an unparsable or
	// unspecified remote through the same check; this is its pair on the
	// backend path, for a spec that reached CreateTunnel another way.
	if err := spec.CheckRemote(); err != nil {
		return nil, err
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
// has no local interface set (ifindex 0 = no parent in netlink messages). The
// kernel uses uint32 for ifindex; the upper-bound check defends against
// any future change that would let the netlink library return values too
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

// GetTunnelInfo reads live encapsulation parameters for a non-IPsec tunnel
// netdev via rtnetlink. For GRE/GRETap it reads OFlags directly (avoiding the
// vendored netlink parseGretapData IKey/OKey swap) and inspects Remote before
// Local to determine outer address family (avoiding Local.To4() == nil when
// local-interface is used). For IPv6 GRE it issues one RTM_GETLINK to read
// IFLA_GRE_FLAGS so IPv6EncapLimit reflects IP6_TNL_F_IGN_ENCAP_LIMIT exactly.
func (b *netlinkBackend) GetTunnelInfo(name string) (iface.TunnelInfo, error) {
	if err := iface.ValidateIfaceName(name); err != nil {
		return iface.TunnelInfo{}, fmt.Errorf("tunnel info %q: %w", name, err)
	}
	link, err := netlink.LinkByName(name)
	if err != nil {
		return iface.TunnelInfo{}, fmt.Errorf("iface: tunnel info %q: %w", name, err)
	}

	switch t := link.(type) {
	case *netlink.Gretun:
		info, err := greTunnelInfo(iface.TunnelKindGRE, iface.TunnelKindIP6GRE,
			t.Index, t.Local, t.Remote, t.OFlags, t.EncapType, t.FlowBased)
		if err != nil {
			return iface.TunnelInfo{}, fmt.Errorf("iface: tunnel info %q: %w", name, err)
		}
		return info, nil
	case *netlink.Gretap:
		info, err := greTunnelInfo(iface.TunnelKindGRETap, iface.TunnelKindIP6GRETap,
			t.Index, t.Local, t.Remote, t.OFlags, t.EncapType, t.FlowBased)
		if err != nil {
			return iface.TunnelInfo{}, fmt.Errorf("iface: tunnel info %q: %w", name, err)
		}
		return info, nil
	case *netlink.Vxlan:
		if t.VxlanId < 0 {
			return iface.TunnelInfo{}, fmt.Errorf("iface: tunnel info %q: negative vxlan id %d", name, t.VxlanId)
		}
		if int64(t.VxlanId) > math.MaxUint32 {
			return iface.TunnelInfo{}, fmt.Errorf("iface: tunnel info %q: vxlan id %d overflows uint32", name, t.VxlanId)
		}
		local, remote, err := tunnelEndpointAddrs(t.SrcAddr, t.Group)
		if err != nil {
			return iface.TunnelInfo{}, fmt.Errorf("iface: tunnel info %q: %w", name, err)
		}
		return iface.TunnelInfo{
			Kind:      iface.TunnelKindVxlan,
			Local:     local,
			Remote:    remote,
			FlowBased: t.FlowBased,
			VNI:       uint32(t.VxlanId),
		}, nil
	case *netlink.Iptun:
		local, remote, err := tunnelEndpointAddrs(t.Local, t.Remote)
		if err != nil {
			return iface.TunnelInfo{}, fmt.Errorf("iface: tunnel info %q: %w", name, err)
		}
		return iface.TunnelInfo{
			Kind:           iface.TunnelKindIPIP,
			Local:          local,
			Remote:         remote,
			OuterEncapType: t.EncapType,
			FlowBased:      t.FlowBased,
		}, nil
	case *netlink.Sittun:
		local, remote, err := tunnelEndpointAddrs(t.Local, t.Remote)
		if err != nil {
			return iface.TunnelInfo{}, fmt.Errorf("iface: tunnel info %q: %w", name, err)
		}
		return iface.TunnelInfo{
			Kind:           iface.TunnelKindSIT,
			Local:          local,
			Remote:         remote,
			OuterEncapType: t.EncapType,
		}, nil
	case *netlink.Ip6tnl:
		local, remote, err := tunnelEndpointAddrs(t.Local, t.Remote)
		if err != nil {
			return iface.TunnelInfo{}, fmt.Errorf("iface: tunnel info %q: %w", name, err)
		}
		kind := iface.TunnelKindIP6Tnl
		if t.Proto == syscall.IPPROTO_IPIP {
			kind = iface.TunnelKindIPIP6
		}
		return iface.TunnelInfo{
			Kind:           kind,
			Local:          local,
			Remote:         remote,
			IPv6EncapLimit: t.Flags&uint32(netlink.IP6_TNL_F_IGN_ENCAP_LIMIT) == 0,
			OuterEncapType: t.EncapType,
			FlowBased:      t.FlowBased,
		}, nil
	default:
		return iface.TunnelInfo{}, fmt.Errorf("iface: tunnel info %q: not a tunnel interface (type %q)", name, link.Type())
	}
}

// netIPToAddr converts an optional net.IP from netlink into an unmapped
// netip.Addr. An empty slice represents an omitted endpoint attribute; a
// non-empty slice that is neither 4 nor 16 bytes is refused as an error.
func netIPToAddr(ip net.IP) (netip.Addr, error) {
	if len(ip) == 0 {
		return netip.Addr{}, nil
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}, fmt.Errorf("malformed kernel IP length %d", len(ip))
	}
	return addr.Unmap(), nil
}

// tunnelEndpointAddrs decodes a tunnel's local and remote netlink IP slices.
func tunnelEndpointAddrs(localIP, remoteIP net.IP) (local, remote netip.Addr, err error) {
	local, err = netIPToAddr(localIP)
	if err != nil {
		return netip.Addr{}, netip.Addr{}, fmt.Errorf("local endpoint: %w", err)
	}
	remote, err = netIPToAddr(remoteIP)
	if err != nil {
		return netip.Addr{}, netip.Addr{}, fmt.Errorf("remote endpoint: %w", err)
	}
	return local, remote, nil
}

// greTunnelInfo builds TunnelInfo for both L3 (Gretun) and L2 (Gretap) links.
// Underlay family prefers Remote when valid and falls back to Local so an IPv4
// tunnel using local-interface (unset Local) is never misclassified as IPv6.
func greTunnelInfo(kindV4, kindV6 iface.TunnelKind, index int, local, remote net.IP, oflags, encapType uint16, flowBased bool) (iface.TunnelInfo, error) {
	localAddr, remoteAddr, err := tunnelEndpointAddrs(local, remote)
	if err != nil {
		return iface.TunnelInfo{}, err
	}
	var isV6 bool
	if remoteAddr.IsValid() {
		isV6 = remoteAddr.Is6()
	} else if localAddr.IsValid() {
		isV6 = localAddr.Is6()
	}
	kind := kindV4
	if isV6 {
		kind = kindV6
	}
	info := iface.TunnelInfo{
		Kind:           kind,
		Local:          localAddr,
		Remote:         remoteAddr,
		GREKey:         oflags&uint16(nl.GRE_KEY) != 0,
		GREChecksum:    oflags&uint16(nl.GRE_CSUM) != 0,
		GRESequence:    oflags&uint16(nl.GRE_SEQ) != 0,
		OuterEncapType: encapType,
		FlowBased:      flowBased,
	}
	if isV6 && !flowBased {
		encapLimit, err := readIP6GREEncapLimit(index)
		if err != nil {
			return iface.TunnelInfo{}, err
		}
		info.IPv6EncapLimit = encapLimit
	}
	return info, nil
}

// readIP6GREEncapLimit issues one RTM_GETLINK for index and reads
// IFLA_GRE_FLAGS from IFLA_LINKINFO -> IFLA_INFO_DATA. It returns true when
// IP6_TNL_F_IGN_ENCAP_LIMIT is clear (meaning Linux emits the 8-octet RFC 2473
// Section 4.1.1 Tunnel Encapsulation Limit Destination Options header).
func readIP6GREEncapLimit(index int) (bool, error) {
	if index <= 0 {
		return false, fmt.Errorf("read ip6gre flags: invalid ifindex %d", index)
	}
	if int64(index) > math.MaxInt32 {
		return false, fmt.Errorf("read ip6gre flags: ifindex %d overflows int32", index)
	}
	req := nl.NewNetlinkRequest(unix.RTM_GETLINK, 0)
	msg := nl.NewIfInfomsg(unix.AF_UNSPEC)
	msg.Index = int32(index)
	req.AddData(msg)
	answers, err := req.Execute(unix.NETLINK_ROUTE, unix.RTM_NEWLINK)
	if err != nil {
		return false, fmt.Errorf("read ip6gre flags (ifindex %d): %w", index, err)
	}
	if len(answers) != 1 {
		return false, fmt.Errorf("read ip6gre flags (ifindex %d): got %d netlink answers, want 1", index, len(answers))
	}
	if len(answers[0]) < unix.SizeofIfInfomsg {
		return false, fmt.Errorf("read ip6gre flags (ifindex %d): truncated ifinfomsg (%d bytes)", index, len(answers[0]))
	}
	return ip6GREEncapLimitFromAttrs(answers[0][unix.SizeofIfInfomsg:])
}

// ip6GREEncapLimitFromAttrs locates IFLA_LINKINFO -> IFLA_INFO_DATA ->
// IFLA_GRE_FLAGS inside the attribute bytes following struct ifinfomsg.
func ip6GREEncapLimitFromAttrs(attrs []byte) (bool, error) {
	top, err := nl.ParseRouteAttr(attrs)
	if err != nil {
		return false, fmt.Errorf("parse link attributes: %w", err)
	}
	for _, attr := range top {
		if attr.Attr.Type != unix.IFLA_LINKINFO {
			continue
		}
		return ip6GREEncapLimitFromLinkInfo(attr.Value)
	}
	return false, fmt.Errorf("ip6gre link carries no IFLA_LINKINFO")
}

// ip6GREEncapLimitFromLinkInfo parses IFLA_GRE_FLAGS out of IFLA_LINKINFO.
func ip6GREEncapLimitFromLinkInfo(linkInfo []byte) (bool, error) {
	entries, err := nl.ParseRouteAttr(linkInfo)
	if err != nil {
		return false, fmt.Errorf("parse IFLA_LINKINFO: %w", err)
	}
	for _, entry := range entries {
		if entry.Attr.Type != nl.IFLA_INFO_DATA {
			continue
		}
		dataAttrs, err := nl.ParseRouteAttr(entry.Value)
		if err != nil {
			return false, fmt.Errorf("parse IFLA_INFO_DATA: %w", err)
		}
		for _, attr := range dataAttrs {
			if attr.Attr.Type != nl.IFLA_GRE_FLAGS {
				continue
			}
			if len(attr.Value) != 4 {
				return false, fmt.Errorf("IFLA_GRE_FLAGS has %d bytes, want 4", len(attr.Value))
			}
			flags := nl.NativeEndian().Uint32(attr.Value)
			return flags&uint32(netlink.IP6_TNL_F_IGN_ENCAP_LIMIT) == 0, nil
		}
		return false, fmt.Errorf("ip6gre IFLA_INFO_DATA carries no IFLA_GRE_FLAGS")
	}
	return false, fmt.Errorf("ip6gre IFLA_LINKINFO carries no IFLA_INFO_DATA")
}
