// Design: docs/features/interfaces.md -- Tunnel interface specification
// Related: backend.go -- Backend.CreateTunnel uses TunnelSpec
// Related: config.go -- parseTunnelEntry/parseTunnelLeaves populate TunnelSpec from YANG

package iface

// TunnelKind discriminates between the supported tunnel encapsulation kinds.
// One TunnelKind value maps to exactly one Linux netlink interface kind.
// IPIP6 is special: it shares the ip6tnl Go type with IP6TNL, distinguished
// by the inner protocol (IPPROTO_IPIP for IPIP6, IPPROTO_IPV6 for IP6TNL).
type TunnelKind int

const (
	// TunnelKindUnknown is the zero value and represents an unset kind.
	TunnelKindUnknown TunnelKind = iota
	// TunnelKindGRE is GRE over IPv4 (RFC 2784, key extension RFC 2890). L3.
	TunnelKindGRE
	// TunnelKindGRETap is GRE over IPv4, L2 (Ethernet over GRE, bridgeable).
	TunnelKindGRETap
	// TunnelKindIP6GRE is GRE over IPv6, L3.
	TunnelKindIP6GRE
	// TunnelKindIP6GRETap is GRE over IPv6, L2 (bridgeable).
	TunnelKindIP6GRETap
	// TunnelKindIPIP is IPv4 in IPv4 (RFC 2003). No GRE header, no key.
	TunnelKindIPIP
	// TunnelKindSIT is IPv6 in IPv4 (6in4, RFC 4213).
	TunnelKindSIT
	// TunnelKindIP6Tnl is IPv6 in IPv6 (RFC 2473). Linux ip6tnl with Proto=IPV6.
	TunnelKindIP6Tnl
	// TunnelKindIPIP6 is IPv4 in IPv6. Linux ip6tnl with Proto=IPIP.
	TunnelKindIPIP6
	// TunnelKindVxlan is a VXLAN overlay (RFC 7348): L2 Ethernet frames
	// carried over a UDP/IPv4 underlay, discriminated by a 24-bit VNI.
	// Landed in both backends (netlink Vxlan link + VPP
	// VxlanAddDelTunnelV3); see tunnelKindNames and the vxlan YANG case.
	TunnelKindVxlan
)

// tunnelKindNames maps each valid TunnelKind to its YANG case name. Used by
// String() and (inverted) by ParseTunnelKind. TunnelKindUnknown is omitted
// because it has no valid YANG name; String() falls back to "unknown" for it.
var tunnelKindNames = map[TunnelKind]string{
	TunnelKindGRE:       "gre",
	TunnelKindGRETap:    "gretap",
	TunnelKindIP6GRE:    "ip6gre",
	TunnelKindIP6GRETap: "ip6gretap",
	TunnelKindIPIP:      "ipip",
	TunnelKindSIT:       "sit",
	TunnelKindIP6Tnl:    "ip6tnl",
	TunnelKindIPIP6:     "ipip6",
	TunnelKindVxlan:     "vxlan",
}

// tunnelKindByName is the inverse of tunnelKindNames, populated in init.
var tunnelKindByName = map[string]TunnelKind{}

func init() {
	for k, name := range tunnelKindNames {
		tunnelKindByName[name] = k
	}
}

// String returns the YANG case name for the tunnel kind.
func (k TunnelKind) String() string {
	if name, ok := tunnelKindNames[k]; ok {
		return name
	}
	return "unknown"
}

// parseTunnelKind returns the TunnelKind for a YANG case name.
// Returns TunnelKindUnknown and false if the name is not recognized.
func parseTunnelKind(name string) (TunnelKind, bool) {
	k, ok := tunnelKindByName[name]
	if !ok {
		return TunnelKindUnknown, false
	}
	return k, true
}

// kernelLinkTypes maps each tunnel kind to the netlink link type the kernel
// reports for a device of that kind. This is the value linkToInfo
// (internal/plugins/iface/netlink/show_linux.go) copies from netlink.Link.Type()
// into InterfaceInfo.Type, so it is what a read-back of an existing netdev can
// be compared against.
//
// ipip6 and ip6tnl share one entry: both are the ip6_tunnel driver, and the
// two differ only by the inner protocol, which the read-back drops. A device
// reported as "ip6tnl" can therefore be either of them, and no comparison over
// InterfaceInfo can say which.
//
// Related: kernelTunnelKinds (discover.go) maps the same kernel names the other
// way, for classifying discovered links into the single "tunnel" ze type.
var kernelLinkTypes = map[TunnelKind]string{
	TunnelKindGRE:       "gre",
	TunnelKindGRETap:    "gretap",
	TunnelKindIP6GRE:    "ip6gre",
	TunnelKindIP6GRETap: "ip6gretap",
	TunnelKindIPIP:      "ipip",
	TunnelKindSIT:       "sit",
	TunnelKindIP6Tnl:    "ip6tnl",
	TunnelKindIPIP6:     "ip6tnl",
	TunnelKindVxlan:     "vxlan",
}

// kernelLinkType returns the netlink link type the kernel reports for this
// kind. The second result is false for a kind with no entry, which the callers
// must treat as "cannot be identified" and fail closed on: an unknown kind
// names no device, so no existing device can be shown to be one.
func (k TunnelKind) kernelLinkType() (string, bool) {
	name, ok := kernelLinkTypes[k]
	return name, ok
}

// v6UnderlayKinds enumerates the tunnel kinds whose outer header is IPv6.
var v6UnderlayKinds = map[TunnelKind]bool{
	TunnelKindIP6GRE:    true,
	TunnelKindIP6GRETap: true,
	TunnelKindIP6Tnl:    true,
	TunnelKindIPIP6:     true,
}

// IsV6Underlay reports whether the tunnel uses an IPv6 outer header.
func (k TunnelKind) IsV6Underlay() bool {
	return v6UnderlayKinds[k]
}

// bridgeableKinds enumerates the L2 tunnel kinds that carry Ethernet frames
// and therefore support VLAN sub-interfaces and bridge port membership.
var bridgeableKinds = map[TunnelKind]bool{
	TunnelKindGRETap:    true,
	TunnelKindIP6GRETap: true,
}

// isBridgeable reports whether the tunnel carries Ethernet frames (L2) and
// therefore can be a bridge port or carry VLAN sub-interfaces. Only gretap
// and ip6gretap qualify; the other six kinds are L3 and reject VLAN tagging.
func (k TunnelKind) isBridgeable() bool {
	return bridgeableKinds[k]
}

// TunnelSpec carries the kind-specific parameters for creating a tunnel
// interface. Backends consume this struct via Backend.CreateTunnel.
//
// Source endpoint: exactly one of LocalAddress or LocalInterface must be
// non-empty. The YANG schema enforces this via a choice statement.
//
// Optional fields use a "set" sentinel where Go's zero value is a valid
// configured value: KeySet, TTLSet, TosSet, etc. This avoids the need for
// pointer fields and matches the existing ipv4Settings pattern in config.go.
type TunnelSpec struct {
	Kind            TunnelKind
	Name            string
	LocalAddress    string // empty if LocalInterface is set
	LocalInterface  string // empty if LocalAddress is set
	RemoteAddress   string // mandatory; v4 for v4-underlay kinds, v6 for v6-underlay
	Key             uint32 // GRE family only; valid only when KeySet
	KeySet          bool
	TTL             uint8 // gre/gretap/ipip/sit only; 0 = inherit (default)
	TTLSet          bool
	Tos             uint8 // gre/gretap/ipip/sit only
	TosSet          bool
	NoPMTUDiscovery bool  // gre/gretap/ipip/sit only
	HopLimit        uint8 // v6-underlay kinds only; default 64
	HopLimitSet     bool
	TClass          uint8 // v6-underlay kinds only
	TClassSet       bool
	EncapLimit      uint8 // ip6tnl/ipip6 only; default 4
	EncapLimitSet   bool
	VNI             uint32 // vxlan only; VXLAN Network Identifier (1..16777215)
	VNISet          bool
	Port            uint16 // vxlan only; UDP destination port; 0 = default 4789
	PortSet         bool
}
