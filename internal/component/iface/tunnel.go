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

// ParseTunnelKind returns the TunnelKind for a YANG case name.
// Returns TunnelKindUnknown and false if the name is not recognized.
func ParseTunnelKind(name string) (TunnelKind, bool) {
	k, ok := tunnelKindByName[name]
	if !ok {
		return TunnelKindUnknown, false
	}
	return k, true
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

// greFamilyKinds enumerates the tunnel kinds that carry a GRE header
// (and therefore support the optional 32-bit key from RFC 2890).
var greFamilyKinds = map[TunnelKind]bool{
	TunnelKindGRE:       true,
	TunnelKindGRETap:    true,
	TunnelKindIP6GRE:    true,
	TunnelKindIP6GRETap: true,
}

// IsGREFamily reports whether the tunnel uses a GRE header.
func (k TunnelKind) IsGREFamily() bool {
	return greFamilyKinds[k]
}

// bridgeableKinds enumerates the L2 tunnel kinds that carry Ethernet frames
// and therefore support VLAN sub-interfaces and bridge port membership.
var bridgeableKinds = map[TunnelKind]bool{
	TunnelKindGRETap:    true,
	TunnelKindIP6GRETap: true,
}

// IsBridgeable reports whether the tunnel carries Ethernet frames (L2) and
// therefore can be a bridge port or carry VLAN sub-interfaces. Only gretap
// and ip6gretap qualify; the other six kinds are L3 and reject VLAN tagging.
func (k TunnelKind) IsBridgeable() bool {
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
// pointer fields and matches the existing ipv4Sysctl pattern in config.go.
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
}
