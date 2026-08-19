// Design: docs/architecture/core-design.md — shared BGP types
//
// Package types defines BGP-specific route types and constants.
//
// These types are used across the codebase for route announcement,
// withdrawal, and update processing. They were extracted from
// internal/plugin/types.go to separate BGP-specific concerns from
// generic plugin infrastructure.
package types

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// AFI name constants for API use.
// These match the string representations used in commands and JSON output.
const (
	AFINameIPv4  = "ipv4"
	AFINameIPv6  = "ipv6"
	AFINameL2VPN = "l2vpn"
)

// SAFI name constants for API use.
// These match the string representations used in commands and JSON output.
const (
	SAFINameUnicast   = "unicast"
	SAFINameMulticast = "multicast"
	SAFINameMPLSVPN   = "mpls-vpn"
	SAFINameNLRIMPLS  = "nlri-mpls" // ExaBGP name for labeled-unicast
	SAFINameFlowSpec  = "flowspec"
	SAFINameEVPN      = "evpn"
	SAFINameMUP       = "mup" // Mobile User Plane (SAFI 85)
)

// TransactionResult holds the result of a commit or rollback operation.
type TransactionResult struct {
	RoutesAnnounced int      // Routes announced (on commit)
	RoutesWithdrawn int      // Routes withdrawn (on commit)
	RoutesDiscarded int      // Routes discarded (on rollback)
	UpdatesSent     int      // Number of UPDATE messages sent
	Families        []string // Address families with EOR sent
	TransactionID   string   // Transaction label
}

// LargeCommunity is an alias for attribute.LargeCommunity (RFC 8092).
type LargeCommunity = attribute.LargeCommunity

// RouteSpec specifies a route for announcement.
// Supports optional BGP path attributes that override iBGP defaults.
//
// IMMUTABILITY: RouteSpec and Wire must not be mutated after being passed
// to any reactor method. The reactor stores shallow copies for efficiency;
// mutation would corrupt internal state.
type RouteSpec struct {
	Prefix  netip.Prefix
	NextHop RouteNextHop              // Encapsulates next-hop policy (explicit or self)
	Wire    *attribute.AttributesWire // Path attributes in wire format
}

// FlowSpecRoute specifies a FlowSpec route for announcement.
type FlowSpecRoute struct {
	Family       family.AFI      // family.AFIIPv4 or family.AFIIPv6
	DestPrefix   *netip.Prefix   // Destination prefix match
	SourcePrefix *netip.Prefix   // Source prefix match
	Protocols    []uint8         // IP protocol numbers
	Ports        []uint16        // Port numbers (src or dst)
	DestPorts    []uint16        // Destination ports
	SourcePorts  []uint16        // Source ports
	Actions      FlowSpecActions // Traffic actions
}

// FlowSpecActions specifies what to do with matching traffic.
//
// Each numeric action carries a companion flag, because zero is a value an
// operator can mean rather than the absence of one: RFC 8955 Section 7.1 gives a
// traffic-rate of 0 the meaning "discard all traffic for this flow", and DSCP 0
// is CS0. Reading the number alone dropped `then rate-limit 0` and `then mark 0`
// on the way to the wire, turning a discard rule into the default accept.
//
// The encoder emits an action when its flag is set OR its value is non-zero, so
// a caller that fills the value alone is still encoded and the flag only ever
// adds. That is the shape the flag exists for: it says "0 was asked for", which
// is the one thing the value cannot say.
type FlowSpecActions struct {
	Accept              bool   // Accept traffic (default)
	Discard             bool   // Drop traffic
	RateLimit           uint32 // Rate limit in bytes/sec
	RateLimitSet        bool   // A bytes/sec rate limit was requested, 0 included
	RateLimitPackets    uint32 // Rate limit in packets/sec
	RateLimitPacketsSet bool   // A packets/sec rate limit was requested, 0 included
	Redirect            string // Redirect target (RT or IP)
	MarkDSCP            uint8  // DSCP marking value
	MarkDSCPSet         bool   // A DSCP marking was requested, 0 (CS0) included
}

// VPLSRoute specifies a VPLS route for announcement.
type VPLSRoute struct {
	RD            string // Route distinguisher (e.g., "65000:100")
	VEBlockOffset uint16 // VE block offset
	VEBlockSize   uint16 // VE block size
	LabelBase     uint32 // Base MPLS label
	NextHop       netip.Addr
}

// L2VPNRoute specifies an L2VPN/EVPN route for announcement.
type L2VPNRoute struct {
	RouteType   string // "mac-ip", "ip-prefix", "multicast", "ethernet-segment", "ethernet-ad"
	RD          string // Route distinguisher
	EthernetTag uint32 // Ethernet Tag ID

	// For mac-ip (Type 2)
	MAC string     // MAC address (e.g., "00:11:22:33:44:55")
	IP  netip.Addr // Optional IP address
	ESI string     // Ethernet Segment Identifier

	// For ip-prefix (Type 5)
	Prefix  netip.Prefix // IP prefix
	Gateway netip.Addr   // Gateway IP

	// Labels
	Label1 uint32 // First MPLS label
	Label2 uint32 // Second MPLS label (optional)

	// Next-hop
	NextHop netip.Addr
}

// L3VPNRoute specifies an L3VPN (MPLS VPN) route for announcement.
// Supports VPNv4 (AFI=1, SAFI=128) and VPNv6 (AFI=2, SAFI=128) per RFC 4364.
type L3VPNRoute struct {
	Prefix  netip.Prefix              // IP prefix
	NextHop netip.Addr                // Next-hop address
	RD      string                    // Route Distinguisher (e.g., "100:100" or "1.2.3.4:100")
	Labels  []uint32                  // MPLS label stack (supports multiple labels per RFC 3032)
	RT      string                    // Route Target (extended community, optional)
	Wire    *attribute.AttributesWire // Path attributes in wire format
}

// LabeledUnicastRoute specifies an MPLS labeled unicast route (SAFI 4).
// This is unicast routing with MPLS labels but without VPN semantics (no RD/RT).
// RFC 8277: Using BGP to Bind MPLS Labels to Address Prefixes.
// RFC 7911: ADD-PATH support via PathID field.
type LabeledUnicastRoute struct {
	Prefix  netip.Prefix              // IP prefix
	NextHop netip.Addr                // Next-hop address
	Labels  []uint32                  // MPLS label stack
	PathID  uint32                    // ADD-PATH path identifier (RFC 7911), 0 means not set
	Wire    *attribute.AttributesWire // Path attributes in wire format
}

// MUPRouteSpec specifies a MUP route for announcement (SAFI 85).
// Per draft-mpmz-bess-mup-safi for Mobile User Plane.
type MUPRouteSpec struct {
	RouteType    string                    // mup-isd, mup-dsd, mup-t1st, mup-t2st
	IsIPv6       bool                      // AFI: false=IPv4, true=IPv6
	Prefix       string                    // For ISD, T1ST (e.g., "10.0.1.0/24")
	Address      string                    // For DSD, T2ST (e.g., "10.0.0.1")
	RD           string                    // Route Distinguisher
	TEID         string                    // Tunnel Endpoint ID (for T1ST/T2ST)
	QFI          uint8                     // QoS Flow Identifier
	Endpoint     string                    // GTP endpoint address
	Source       string                    // Source address (optional)
	NextHop      string                    // Next-hop address (IPv6 for SRv6)
	ExtCommunity string                    // Extended communities (e.g., "[target:10:10]")
	PrefixSID    string                    // SRv6 Prefix SID (e.g., "l3-service 2001:db8::1 0x48 [64,24,16,0,0,0]")
	Wire         *attribute.AttributesWire // Path attributes in wire format
}

// NLRIGroup represents a group of NLRIs sharing the same attributes.
// Used by ParseUpdateText to capture attribute snapshots per NLRI section.
type NLRIGroup struct {
	Family       family.Family             // Address family (AFI/SAFI)
	Announce     []nlri.NLRI               // NLRIs to announce
	Withdraw     []nlri.NLRI               // NLRIs to withdraw
	Wire         *attribute.AttributesWire // Path attributes in wire format
	NextHop      RouteNextHop              // Encapsulates next-hop policy (explicit or self)
	WatchdogName string                    // Watchdog pool name for routes (empty = none)
	// OriginAS, when nonzero, originates the route as a virtual router with this
	// AS: the reactor synthesizes AS_PATH [OriginAS] for iBGP peers and prepends
	// the local AS for eBGP peers ([localAS, OriginAS]) -- the normal export
	// rule, unlike a verbatim `as-path`. Zero uses the legacy default (empty
	// iBGP / [localAS] eBGP). Set by the `origin-as` update-text token.
	OriginAS uint32
}

// UpdateTextResult is the parsed result of an update text command.
type UpdateTextResult struct {
	Groups       []NLRIGroup
	WatchdogName string
	EORFamilies  []family.Family // EOR markers to send (RFC 4724)
}

// NLRIBatch represents a batch of NLRIs with shared attributes.
// Used for efficient UPDATE message generation - reactor builds wire format
// and splits into multiple messages if exceeding peer's max size.
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI/MP_UNREACH_NLRI for non-IPv4-unicast families.
type NLRIBatch struct {
	Family  family.Family             // AFI/SAFI for all NLRIs
	NLRIs   []nlri.NLRI               // NLRIs to announce or withdraw
	NextHop RouteNextHop              // Next-hop policy (announce only)
	Attrs   *attribute.Builder        // Attribute builder (for new routes)
	Wire    *attribute.AttributesWire // Wire passthrough (for forwarding)
	// OriginAS, when nonzero, originates the route as a virtual router with this
	// AS: AS_PATH is [OriginAS] to iBGP peers and [localAS, OriginAS] to eBGP
	// peers (normal export prepend). Distinct from a verbatim `as-path` (route
	// server transparency). Zero uses the legacy default AS_PATH synthesis.
	OriginAS uint32
	// Stale is the LLGR stale level (RFC 9494) carried from the RIB
	// readvertise producer (meta["stale"]). When >0, AnnounceNLRIBatch runs the
	// registered readvertise egress filters per destination peer so stale routes
	// are kept+marked for LLGR-capable peers, depreferenced for non-LLGR iBGP
	// peers, and converted to withdrawals for non-LLGR eBGP peers. Zero (the
	// common case) takes the unchanged grouped announce path.
	Stale uint8
}

// RIBStatsInfo holds RIB statistics for Adj-RIB-In and Adj-RIB-Out.
// Moved from internal/plugin/types.go — this is a BGP-specific type.
type RIBStatsInfo struct {
	InPeerCount   int `json:"in-peer-count"`
	InRouteCount  int `json:"in-route-count"`
	OutPending    int `json:"out-pending"`
	OutWithdrawls int `json:"out-withdrawals"`
	OutSent       int `json:"out-sent"`
}
