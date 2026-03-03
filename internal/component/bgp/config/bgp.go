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

	// MUP route types for SRv6 Mobile User Plane.
	routeTypeMUPISD  = "mup-isd"
	routeTypeMUPDSD  = "mup-dsd"
	routeTypeMUPT1ST = "mup-t1st"
	routeTypeMUPT2ST = "mup-t2st"

	// Common field names.
	fieldSource = "source"
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
	PrefixSID         string   // BGP Prefix-SID (RFC 8669) - can be number or "N, [(base,range),...]"

	// Split prefix into more-specific routes (e.g., "/25" splits /24 into two /25s)
	Split string
}

// MVPNRouteConfig holds an MVPN route configuration.
type MVPNRouteConfig struct {
	RouteType         string // shared-join, source-join, source-ad
	IsIPv6            bool
	RD                string
	SourceAS          uint32
	Source            string // source IP or RP IP
	Group             string // multicast group
	NextHop           string
	Origin            string
	LocalPreference   uint32
	MED               uint32
	ExtendedCommunity string
	OriginatorID      string // RFC 4456 route reflector
	ClusterList       string // RFC 4456 route reflector
}

// VPLSRouteConfig holds a VPLS route configuration.
type VPLSRouteConfig struct {
	Name              string
	RD                string
	Endpoint          uint16
	Base              uint32
	Offset            uint16
	Size              uint16
	NextHop           string
	Origin            string
	LocalPreference   uint32
	MED               uint32
	ASPath            string
	Community         string
	ExtendedCommunity string
	OriginatorID      string
	ClusterList       string
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

// MUPRouteConfig holds a MUP route configuration.
type MUPRouteConfig struct {
	RouteType         string // mup-isd, mup-dsd, mup-t1st, mup-t2st
	IsIPv6            bool
	Prefix            string
	Address           string
	RD                string
	TEID              string
	QFI               uint8
	Endpoint          string
	Source            string // T1ST source address
	NextHop           string
	ExtendedCommunity string
	PrefixSID         string
}
