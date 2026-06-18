// Design: docs/architecture/config/syntax.md — BGP config types and tree-to-map conversion

package bgpconfig

import (
	"net/netip"
	"strings"
)

const (
	configTrue    = "true"    // Config value for boolean true
	configFalse   = "false"   // Config value for boolean false
	configEnable  = "enable"  // Config value for enabled state
	configDisable = "disable" // Config value for disabled state
	configRequire = "require" // Config value for required state
	configSelf    = "self"    // Config value for next-hop self

	// DefaultHoldTime is the default hold time per RFC 4271 Section 10.
	DefaultHoldTime = 90
)

// FamilyMode represents the negotiation mode for an address family.
type FamilyMode int

const (
	// FamilyModeEnable advertises the family, accepts if peer doesn't support.
	// Strict on UPDATE: error if peer sends NLRI for non-negotiated family.
	FamilyModeEnable FamilyMode = iota
	// FamilyModeDisable does not advertise the family.
	FamilyModeDisable
	// FamilyModeRequire advertises the family, refuses session if peer doesn't support.
	FamilyModeRequire
	// FamilyModeIgnore advertises the family, accepts if peer doesn't support.
	// Lenient on UPDATE: skip NLRI for non-negotiated family instead of error.
	FamilyModeIgnore
)

// String returns the string representation of FamilyMode.
func (m FamilyMode) String() string {
	switch m {
	case FamilyModeEnable:
		return configEnable
	case FamilyModeDisable:
		return configDisable
	case FamilyModeRequire:
		return configRequire
	case FamilyModeIgnore:
		return "ignore"
	}
	return "unknown"
}

// ParseFamilyMode parses a string into a FamilyMode.
// Returns FamilyModeEnable for empty string or "true"/"enable".
func ParseFamilyMode(s string) FamilyMode {
	switch strings.ToLower(s) {
	case "", configTrue, configEnable:
		return FamilyModeEnable
	case configFalse, configDisable:
		return FamilyModeDisable
	case configRequire:
		return FamilyModeRequire
	case "ignore":
		return FamilyModeIgnore
	}
	return FamilyModeEnable
}

// StaticRouteConfig holds a static route.
type StaticRouteConfig struct {
	Prefix            netip.Prefix
	NextHop           string
	NextHopSelf       bool   // Use local address as next-hop
	Origin            string // igp, egp, incomplete
	LocalPreference   uint32
	MED               uint32
	Community         string
	ExtendedCommunity string
	LargeCommunity    string
	ASPath            string
	PathInformation   string   // path-id for add-path
	Label             string   // MPLS label (backward compat, single)
	Labels            []string // RFC 8277: MPLS label stack (multiple)
	RD                string   // Route Distinguisher
	Aggregator        string   // ASN:IP format
	AtomicAggregate   bool     // ATOMIC_AGGREGATE attribute
	Attribute         string   // Raw attribute hex: [ code flags value ]
	OriginatorID      string   // ORIGINATOR_ID (RFC 4456)
	ClusterList       string   // CLUSTER_LIST (RFC 4456)
	AIGP              string   // AIGP metric (RFC 7311)
	PrefixSID         string   // BGP Prefix-SID (RFC 8669) - can be number or "N, [(base,range),...]"

	// Split prefix into more-specific routes (e.g., "/25" splits /24 into two /25s)
	Split string
}

// FlowSpecRouteConfig holds a FlowSpec route configuration.
// RFC 8955 Section 4: NLRI contains match criteria (destination, source, protocol, ports, etc.)
// RFC 8955 Section 7: Actions are encoded as Extended Communities (rate-limit, redirect, etc.)
type FlowSpecRouteConfig struct {
	Name              string
	IsIPv6            bool
	RD                string              // for flow-vpn (SAFI 134)
	NLRI              map[string][]string // Match criteria (RFC 8955 Section 4)
	NextHop           string
	Community         string
	ExtendedCommunity string // Actions as extended communities (RFC 8955 Section 7)
	Attribute         string // Raw attribute hex: [ code flags value ]
}

// PluginRouteConfig holds a generic route produced by a plugin's config parser.
// Mirrors registry.PluginRoute but lives in the config layer.
type PluginRouteConfig struct {
	Family  string // "ipv4/sr-policy", etc.
	IsIPv6  bool
	NLRI    []byte // Pre-built NLRI wire bytes.
	NextHop string
	Attrs   []PluginRouteAttrConfig // Extra path attributes.

	// ASPath is the configured AS_PATH (built with ASN4 context at send time).
	ASPath []uint32
	// LocalPreference is the configured LOCAL_PREF (emitted only on iBGP; 0 = default 100).
	LocalPreference uint32
	// Group packs same-family same-attribute routes into one UPDATE (MVPN).
	Group bool
	// MapV4NextHop maps an IPv4 next-hop to IPv4-mapped IPv6 for IPv6 families.
	MapV4NextHop bool
}

// PluginRouteAttrConfig is a pre-built path attribute for a plugin route.
type PluginRouteAttrConfig struct {
	Code  uint8
	Flags uint8
	Value []byte
}
