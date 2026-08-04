// Design: docs/architecture/core-design.md — peer configuration settings
// Related: config.go — config tree parsing produces PeerSettings
//
// Package reactor implements the BGP reactor - the main orchestrator
// that manages peer sessions, connections, and signal handling.
package reactor

import (
	"maps"
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// NextHopMode values for PeerSettings.NextHopMode.
const (
	// NextHopAuto is the default: rewrite for eBGP, preserve for iBGP (RFC 4271 Section 5.1.3).
	NextHopAuto uint8 = iota
	// NextHopSelf always rewrites next-hop to local address.
	NextHopSelf
	// NextHopUnchanged never rewrites next-hop.
	NextHopUnchanged
	// NextHopExplicit sets next-hop to PeerSettings.NextHopAddress.
	NextHopExplicit
)

// DefaultBGPPort is the standard BGP port per RFC 4271.
// Source of truth: ze-bgp-conf.yang (environment > tcp > port, default 179).
// Used as fallback when YANG schema defaults are not applied (tests, PeerKey).
const DefaultBGPPort = 179

// ConnectionMode controls TCP connection establishment for a peer.
// Two independent booleans: Connect (dial out) and Accept (accept inbound).
// RFC 4271 Section 8.1.1: PassiveTcpEstablishment optional attribute.
type ConnectionMode struct {
	Connect bool // Initiate outbound TCP connections
	Accept  bool // Accept inbound TCP connections
}

// ConnectionBoth is the default: initiate and accept connections.
var ConnectionBoth = ConnectionMode{Connect: true, Accept: true}

// ConnectionActive initiates only.
var ConnectionActive = ConnectionMode{Connect: true, Accept: false}

// ConnectionPassive accepts only.
var ConnectionPassive = ConnectionMode{Connect: false, Accept: true}

// IsActive reports whether dialing out is enabled.
func (m ConnectionMode) IsActive() bool { return m.Connect }

// IsPassive reports whether accepting inbound is enabled.
func (m ConnectionMode) IsPassive() bool { return m.Accept }

// DefaultReceiveHoldTime is the default receive hold time per RFC 4271.
// Source of truth: ze-bgp-conf.yang (timer > receive-hold-time, default 90).
// Production config reads this from YANG via ApplyDefaults.
const DefaultReceiveHoldTime = 90 * time.Second

// DefaultConnectRetry is the default connect retry interval per RFC 4271.
// Source of truth: ze-bgp-conf.yang (timer > connect-retry, default 120).
// Production config reads this from YANG via ApplyDefaults.
const DefaultConnectRetry = 120 * time.Second

// StaticRoute represents a route to announce when session is established.
// Fields are stored in both serializable (string/uint) and wire-ready formats.
//
// IMMUTABILITY: StaticRoute and its slices (ASPath, Communities, etc.) must not
// be mutated after being stored. Peer settings store shallow
// copies for efficiency; mutation would corrupt internal state.
type StaticRoute struct {
	Prefix  netip.Prefix
	NextHop bgptypes.RouteNextHop // Encapsulates next-hop policy (explicit or self)

	Origin          uint8  // 0=IGP, 1=EGP, 2=INCOMPLETE
	LocalPreference uint32 // For iBGP
	MED             uint32 // Multi-Exit Discriminator

	// Communities (RFC 1997) - each uint32 is ASN:value (high 16 bits : low 16 bits)
	Communities []uint32

	// Large communities (RFC 8092) - each is [3]uint32: GlobalAdmin:LocalData1:LocalData2
	LargeCommunities [][3]uint32

	// Extended communities - both forms for serialization and wire encoding
	ExtCommunity      string // Original string (e.g., "target:72:1")
	ExtCommunityBytes []byte // Wire-format (8 bytes each, sorted)

	PathID uint32   // ADD-PATH path identifier
	Labels []uint32 // RFC 8277: MPLS label stack (20-bit values)

	// Route Distinguisher - both forms for serialization and wire encoding
	RD      string  // Original string (e.g., "100:100")
	RDBytes [8]byte // Wire-format (8 bytes)

	// AS_PATH - list of AS numbers in AS_SEQUENCE
	ASPath []uint32

	// Aggregator (RFC 4271) - 4-byte ASN + 4-byte IP
	AggregatorASN uint32
	AggregatorIP  [4]byte
	HasAggregator bool

	// ATOMIC_AGGREGATE flag
	AtomicAggregate bool

	// ORIGINATOR_ID and CLUSTER_LIST (RFC 4456)
	OriginatorID uint32
	ClusterList  []uint32

	// AIGP metric (RFC 7311)
	AIGPMetric *uint64

	// BGP Prefix-SID (RFC 8669) - wire-format bytes for attribute type 40
	PrefixSIDBytes []byte

	// Raw attributes (code, flags, value bytes)
	RawAttributes []RawAttribute
}

// RawAttribute represents a raw BGP path attribute.
type RawAttribute struct {
	Code  uint8
	Flags uint8
	Value []byte
}

// IsVPN returns true if this is a VPN route (has RD).
func (r *StaticRoute) IsVPN() bool {
	return r.RD != ""
}

// IsLabeledUnicast returns true if this is a labeled unicast route (has labels but no RD).
// RFC 8277 Section 2: Labeled routes have MPLS label stack but no Route Distinguisher.
func (r *StaticRoute) IsLabeledUnicast() bool {
	return len(r.Labels) > 0 && r.RD == ""
}

// SingleLabel returns the first label from the label stack, or 0 if empty.
func (r *StaticRoute) SingleLabel() uint32 {
	if len(r.Labels) > 0 {
		return r.Labels[0]
	}
	return 0
}

// RouteKey returns a unique key for this route, suitable for use as a map key.
// Includes prefix, RD (for VPN), and PathID (for ADD-PATH).
// PathID is always included since 0 is a valid path identifier.
func (r *StaticRoute) RouteKey() string {
	key := r.Prefix.String()
	if r.RD != "" {
		key = r.RD + ":" + key
	}
	b := textbuf.Get()
	defer b.Release()
	return b.Str(key).Byte('#').Uint32(r.PathID).String()
}

// PluginRoute is a generic route built by a plugin's config route parser.
// Carries pre-built wire bytes so the reactor needs no family-specific code.
type PluginRoute struct {
	Family   string // "ipv4/sr-policy", etc.
	IsIPv6   bool
	NLRI     []byte // Pre-built NLRI wire bytes.
	NextHop  netip.Addr
	RawAttrs [][]byte // Extra pre-built attribute wire bytes (flags+code+len+value).

	// ASPath is the configured AS_PATH, encoded with ASN4 context by BuildPlugin.
	ASPath []uint32
	// LocalPreference is emitted only on iBGP sessions (0 = default 100).
	LocalPreference uint32
	// Group lets same-family same-attribute routes pack into one UPDATE (MVPN).
	Group bool
	// MapV4NextHop maps an IPv4 next-hop to IPv4-mapped IPv6 for IPv6 families.
	MapV4NextHop bool
}

// BFDSettings carries the per-peer BFD opt-in parsed from the YANG
// `bgp peer connection bfd { ... }` block. A nil BFD field on
// PeerSettings means the peer does not use BFD at all (the reactor
// skips the BFD client wiring); a non-nil struct with Enabled=false
// means the operator left the container in place but suspended it
// (for maintenance) -- the reactor logs and skips the BFD client
// until the next reload with Enabled=true.
//
// Field names mirror the YANG leaf names one-for-one so the parser
// and the audit trail read identically.
type BFDSettings struct {
	// Enabled is the master switch. Default true (YANG default).
	Enabled bool
	// MultiHop is true when the YANG mode leaf is "multi-hop". Stored
	// as a bool rather than an enum because the only two modes,
	// single-hop and multi-hop, map cleanly to api.SingleHop /
	// api.MultiHop at conversion time.
	MultiHop bool
	// Profile is the name of a profile defined under the top-level
	// bfd { profile ... } block. The BFD plugin resolves it; the BGP
	// parser does not validate it (cross-component lookup would pull
	// the BGP tree into the BFD plugin's lifecycle).
	Profile string
	// MinTTL is the multi-hop minimum receive TTL. Zero means use the
	// BFD plugin default (254).
	MinTTL uint8
	// Interface is the optional single-hop egress interface. Empty
	// means let the BFD plugin derive it from the peer's local
	// address.
	Interface string
}

// PeerSettings contains configuration for a BGP peer.
type PeerSettings struct {
	// Name is an optional human-readable peer name for CLI selector.
	Name string

	// GroupName is the peer-group this peer belongs to.
	GroupName string

	// Address is the peer's IP address.
	Address netip.Addr

	// LocalAddress is our local IP for this session.
	LocalAddress netip.Addr

	// LinkLocal is the IPv6 link-local address for MP_REACH next-hop (RFC 2545 Section 3).
	// When set, IPv6 unicast MP_REACH_NLRI includes 32-byte next-hop (global + link-local).
	LinkLocal netip.Addr

	// Port is the peer's BGP port (default 179).
	Port uint16

	// LocalAS is our effective AS number for this session.
	// Equals the per-peer local-as override when set, otherwise the global local-as.
	LocalAS uint32

	// GlobalLocalAS is the router's global local-as (bgp/session/asn/local),
	// preserved even when LocalAS is overridden per-peer. Used by local-as
	// modifiers to know the "real" AS for dual-prepend semantics.
	// Equals LocalAS when no override is active.
	GlobalLocalAS uint32

	// PeerAS is the peer's AS number.
	PeerAS uint32

	// RouterID is our BGP router identifier (IPv4 format).
	RouterID uint32

	// ReceiveHoldTime is the proposed receive hold time (default 90s, RFC 4271).
	// Advertised in OPEN; negotiated value is min(local, remote).
	ReceiveHoldTime time.Duration

	// SendHoldTime is the send hold timer duration (RFC 9687).
	// 0 = automatic: max(8 minutes, 2x ReceiveHoldTime).
	// Explicit value >= 480s overrides the formula.
	SendHoldTime time.Duration

	// KeepaliveTime is the explicit keepalive interval (RFC 4271 Section 10).
	// 0 = auto: holdTime/3. Non-zero overrides the derivation.
	KeepaliveTime time.Duration

	// ConnectRetry is the initial connect retry interval (default 5s).
	// Used as the base for exponential backoff in peer.run().
	ConnectRetry time.Duration

	// Connection controls TCP connection establishment mode.
	// ConnectionBoth (default): initiate and accept.
	// ConnectionPassive: accept only (no dial out).
	// ConnectionActive: dial only (no bind/listen).
	Connection ConnectionMode

	// MD5Key is the TCP MD5 authentication key (RFC 2385).
	// When non-empty, TCP_MD5SIG is applied on both dialer and listener sockets.
	// The MD5IP field specifies which address to authenticate (defaults to Address).
	MD5Key string

	// MD5IP overrides the peer address used for TCP_MD5SIG setsockopt.
	// Useful for multihop BGP where the MD5 key is bound to a different address.
	// Defaults to Address when empty.
	MD5IP netip.Addr

	// OutTTL is the outgoing IP TTL / IPv6 Hop Limit for BGP TCP packets.
	// Zero means leave the OS default unchanged. RFC 5082 GTSM uses 255.
	OutTTL uint8

	// MinTTL is the minimum accepted inbound IP TTL / IPv6 Hop Limit.
	// Zero means do not install a kernel receive-side TTL gate.
	MinTTL uint8

	// BFD is the parsed BFD opt-in from YANG
	// `bgp peer connection bfd { ... }`. nil means the peer does not
	// opt into BFD; the reactor skips all BFD client wiring for this
	// peer. Non-nil means the BGP peer lifecycle calls
	// internal/component/bfd/api.GetService() when the session
	// transitions to Established and tears the session down on a
	// BFD Down event.
	BFD *BFDSettings

	// Capture is the per-peer protocol event capture opt-in, from YANG
	// `bgp peer capture { ... }`. Disabled by default; when enabled the
	// session tees every inbound wire message to a bounded JSONL file
	// (capture_replay.go).
	Capture CaptureSettings

	// GroupUpdates indicates whether to group compatible routes in single UPDATE.
	// Default: true (reduces UPDATE count from O(routes) to O(routes/capacity)).
	GroupUpdates bool

	// IsDynamic marks this peer as created from a dynamic group template.
	// Dynamic peers are created at connection time, not config time.
	IsDynamic bool

	// RSClient marks this peer as an RS-client for transparent AS-path forwarding.
	// RFC 7947 Section 2.2.2: the RS MUST NOT modify AS_PATH for RS-client peers.
	RSClient bool

	// RSFastPath enables reactor-native RS forwarding for this peer's group.
	// When true, received UPDATEs are forwarded directly from notifyMessageReceiver
	// via reactorForwardRS, bypassing the plugin dispatch -> bgp-rs -> ForwardCached
	// chain. Peers with ExportFilters are excluded from the fast path and fall
	// back to bgp-rs ForwardCached.
	RSFastPath bool

	// IgnoreFamilyMismatch ignores NLRI for non-negotiated AFI/SAFI instead of error.
	// RFC 4760 Section 6: speaker MAY treat non-negotiated AFI/SAFI as error.
	// Default false = error (RFC-correct), true = log warning and skip.
	IgnoreFamilyMismatch bool

	// DisableASN4 prevents advertising 4-byte ASN capability.
	DisableASN4 bool

	// Capabilities to advertise in OPEN message.
	Capabilities []capability.Capability

	// RequiredFamilies are address families that must be negotiated.
	// Session will be rejected with NOTIFICATION if peer doesn't support these.
	RequiredFamilies []capability.Family

	// IgnoreFamilies are address families with lenient UPDATE validation.
	// NLRI for these families will be skipped (not error) if not negotiated.
	IgnoreFamilies []capability.Family

	// RequiredCapabilities are non-family capability codes that must be negotiated.
	// Session will be rejected with NOTIFICATION if peer doesn't support these.
	// RFC 5492 Section 3: Unsupported Capability subcode.
	RequiredCapabilities []capability.Code

	// RefusedCapabilities are capability codes that must NOT be present in peer's OPEN.
	// Session will be rejected with NOTIFICATION if peer advertises any of these.
	// Unlike require, refuse checks against peer's raw capabilities, not negotiated intersection.
	RefusedCapabilities []capability.Code

	// RequiredAddPathFamilies are families that must have ADD-PATH negotiated.
	RequiredAddPathFamilies []capability.Family
	// RefusedAddPathFamilies are families that must NOT have ADD-PATH in peer's OPEN.
	RefusedAddPathFamilies []capability.Family

	// StaticRoutes are announced when session is established.
	StaticRoutes []StaticRoute

	// Exotic route types (MUP/VPLS/MVPN/FlowSpec/SR-Policy) all flow through the
	// generic plugin-route path.
	PluginRoutes []PluginRoute

	// PrefixMaximum is the hard maximum number of prefixes accepted per family.
	// Key is "afi/safi" string (e.g., "ipv4/unicast"). Mandatory for every negotiated family.
	// RFC 4486 Section 4: exceeding triggers Cease/MaxPrefixes NOTIFICATION.
	PrefixMaximum map[string]uint32

	// PrefixWarning is the warning threshold per family.
	// Defaults to 90% of PrefixMaximum when not explicitly configured.
	PrefixWarning map[string]uint32

	// PrefixTeardown controls, per family, whether exceeding the maximum stops
	// the session. Key is "afi/safi", same as PrefixMaximum.
	// Read it through PrefixTeardownFor, never with a bare map index: an
	// absent key means enabled (the YANG default), and a bare read returns
	// false, which is warn-only and disables the protection.
	//
	// NewPeerSettings seeds no entry here on purpose. The accessor delivers the
	// YANG default for every family that configures no value, so seeding the map
	// would state the same default in two places.
	PrefixTeardown map[string]bool

	// PrefixIdleTimeout is the seconds to wait before auto-reconnect after a
	// prefix teardown, per family. Key is "afi/safi". The family that
	// exceeded its maximum selects the value; another family's value never
	// sizes the delay. 0 keeps the peer DOWN.
	// Repeated teardowns double the delay, capped at one hour (peer_run.go).
	PrefixIdleTimeout map[string]uint16

	// PrefixReconnect is the per-family answer to "what does the peer do after
	// this family stopped the session". Key is "afi/safi". An absent key, or
	// PrefixReconnectUnset, means the answer comes from PrefixIdleTimeout.
	// Read it through PrefixReconnectFor, never with a bare map index.
	PrefixReconnect map[string]PrefixReconnectMode

	// PrefixUpdated is the ISO date (YYYY-MM-DD) when the prefix maximum was
	// last updated from PeeringDB, per family. Key is "afi/safi". Empty means
	// manually configured (no staleness tracking).
	// Hidden leaf -- not shown in config output.
	// The peer-level surfaces (JSON, report bus, metrics) report the oldest of
	// these dates through OldestPrefixUpdated.
	PrefixUpdated map[string]string

	// Process bindings - which plugins receive messages from this peer.
	ProcessBindings []ProcessBinding

	// ImportFilters is the ordered import filter chain for this peer.
	// Cumulative: bgp-level + group-level + peer-level, in order.
	// Each entry carries the canonical "<plugin>:<filter>" name plus its
	// deactivation state (deactivated refs stay in the chain but are skipped).
	ImportFilters []filterapi.FilterRef

	// ExportFilters is the ordered export filter chain for this peer.
	// Cumulative: bgp-level + group-level + peer-level, in order.
	ExportFilters []filterapi.FilterRef

	// LoopAllowOwnAS is the number of own-AS occurrences to tolerate in AS_PATH.
	// From loop-detection filter config. 0 = reject on first (RFC 4271 Section 9 default).
	LoopAllowOwnAS uint8

	// LoopClusterID is the explicit cluster-id for CLUSTER_LIST loop detection.
	// From loop-detection filter config. 0 = use RouterID (RFC 4456 Section 8 default).
	LoopClusterID uint32

	// LoopDisabled disables loop detection for this peer.
	// Set when the peer's import chain has inactive: on its loop-detection filter.
	LoopDisabled bool

	// AcceptSRv6PrefixSID allows PrefixSID attribute (code 40) from EBGP peers.
	// RFC 8669 Section 4: PrefixSID from EBGP outside the SR domain MUST be
	// discarded unless configured to accept. Default false = discard from EBGP.
	// Has no effect on IBGP sessions (always accepted).
	AcceptSRv6PrefixSID bool

	// RouteReflectorClient marks this peer as a route reflector client (RFC 4456).
	// When true, routes from this peer are forwarded to all other clients and non-clients.
	// When false (non-client), routes from this peer are forwarded to clients only.
	RouteReflectorClient bool

	// ClusterID is the cluster identifier for route reflection (RFC 4456 Section 7).
	// Prepended to CLUSTER_LIST on reflected routes.
	// 0 means use RouterID (default per RFC 4456).
	ClusterID uint32

	// NextHopMode controls next-hop rewriting for forwarded UPDATEs.
	// RFC 4271 Section 5.1.3.
	//   NextHopAuto (0): rewrite for eBGP, preserve for iBGP (default)
	//   NextHopSelf (1): always rewrite to local address
	//   NextHopUnchanged (2): never rewrite
	//   NextHopExplicit (3): set to NextHopAddress
	NextHopMode uint8

	// NextHopAddress is the explicit next-hop IP when NextHopMode == NextHopExplicit.
	NextHopAddress netip.Addr

	// ASOverride replaces the peer's ASN with local ASN in outbound AS_PATH.
	// Used in VPN/multi-site scenarios.
	ASOverride bool

	// LocalASNoPrepend prevents prepending the real ASN before the local-as override.
	// Only relevant when session/asn/local is set (local-as override).
	LocalASNoPrepend bool

	// LocalASReplaceAS replaces the real ASN entirely with the local-as override.
	// Only relevant when session/asn/local is set.
	LocalASReplaceAS bool

	// SendCommunity controls which community types to include in outbound UPDATEs.
	// nil/empty means send all (default). "none" means suppress all.
	// Individual types: "standard", "large", "extended".
	SendCommunity []string

	// DefaultOriginate tracks per-family default route origination.
	// Key is "afi/safi" string (e.g., "ipv4/unicast").
	DefaultOriginate map[string]bool

	// DefaultOriginateFilter tracks per-family conditional origination filters.
	// Key is "afi/safi" string. Empty value means unconditional.
	DefaultOriginateFilter map[string]string

	// RawCapabilityConfig stores parsed capability config values for plugin delivery.
	// Maps capability name → field name → value (e.g., "graceful-restart" → "restart-time" → "120").
	// Populated from config blocks like: capability { graceful-restart { restart-time 120; } }
	// Used for plugin-declared capabilities that don't have Go capability types.
	RawCapabilityConfig map[string]map[string]string

	// CapabilityConfigJSON is the entire capability block as JSON for plugin delivery.
	// Plugins receive this and extract what they need based on their YANG knowledge.
	// This replaces the need for per-plugin extraction code in the config loader.
	CapabilityConfigJSON string
}

// ProcessBinding represents a binding between this peer and a plugin.
// Controls what messages are forwarded and in what format.
type ProcessBinding struct {
	PluginName string // Reference to plugin name

	// Content settings (HOW messages are formatted)
	Encoding string // "json" | "text" (empty = inherit from plugin)
	Format   string // "parsed" | "raw" | "full" (empty = "parsed")

	// Receive settings (WHAT message types to forward)
	ReceiveUpdate       bool
	ReceiveOpen         bool
	ReceiveNotification bool
	ReceiveKeepalive    bool
	ReceiveRefresh      bool
	ReceiveState        bool
	ReceiveSent         bool            // Forward sent UPDATE events
	ReceiveNegotiated   bool            // Forward negotiated capabilities after OPEN exchange
	ReceiveCustom       map[string]bool // Plugin-registered event types (e.g., "update-rpki")

	// Send settings (WHAT message types plugin can send)
	SendUpdate  bool
	SendRefresh bool
	SendCustom  map[string]bool // Plugin-registered send types (e.g., "enhanced-refresh")
}

// NewPeerSettings creates a peer settings with default values.
// In production, YANG schema defaults are applied to the config tree before parsing,
// so these values serve as fallbacks for direct callers (tests, API).
// They MUST match the YANG defaults in ze-bgp-conf.yang.
func NewPeerSettings(address netip.Addr, localAS, peerAS, routerID uint32) *PeerSettings {
	return &PeerSettings{
		Address:         address,
		Port:            DefaultBGPPort,
		LocalAS:         localAS,
		GlobalLocalAS:   localAS, // default: no override, global == effective
		PeerAS:          peerAS,
		RouterID:        routerID,
		ReceiveHoldTime: DefaultReceiveHoldTime,
		ConnectRetry:    DefaultConnectRetry,
		Connection:      ConnectionBoth,
		GroupUpdates:    true,
		// Capture defaults MUST match the YANG defaults of the peer's
		// `capture` container in ze-bgp-conf.yang.
		Capture: CaptureSettings{
			Directory:   DefaultCaptureDirectory,
			MaximumSize: DefaultCaptureMaximumSize,
			OnLimit:     CaptureLimitRotate,
		},
	}
}

// PrefixTeardownFor reports whether exceeding the prefix maximum of fam must
// tear the session down. fam is an "afi/safi" string.
//
// An unconfigured family reads as ENABLED. The YANG default is
// `teardown true` (ze-bgp-conf.yang), and the prefix limit is the defense
// against a peer that floods the RIB, so the absent case must never read as
// warn-only (ai/rules/fail-closed-guards.md). This is why the enforcement path
// calls this method and never indexes PrefixTeardown directly: a bare map read
// returns the zero value false and silently disables the protection.
func (n *PeerSettings) PrefixTeardownFor(fam string) bool {
	if teardown, ok := n.PrefixTeardown[fam]; ok {
		return teardown
	}
	return true
}

// PrefixIdleTimeoutFor returns the seconds to wait before reconnecting after
// fam exceeded its prefix maximum. fam is an "afi/safi" string.
//
// An unconfigured family returns 0, which is also the YANG default. Zero is not
// "reconnect immediately" and not "reconnect on the usual backoff": it keeps the
// peer down, and PrefixReconnectFor is the accessor that states it.
func (n *PeerSettings) PrefixIdleTimeoutFor(fam string) uint16 {
	return n.PrefixIdleTimeout[fam]
}

// PrefixReconnectMode says what a peer does after one of its families stopped
// the session for exceeding its prefix maximum. It is the Go form of the
// per-family `prefix { reconnect ...; }` leaf (ze-bgp-conf.yang).
type PrefixReconnectMode uint8

const (
	// PrefixReconnectUnset means the family stated no `reconnect` value. The
	// mode then comes from `idle-timeout`. It is the zero value, so an
	// unconfigured family is never mistaken for a configured one.
	PrefixReconnectUnset PrefixReconnectMode = iota
	// PrefixReconnectNever keeps the peer down. Only an operator brings it back.
	PrefixReconnectNever
	// PrefixReconnectBackoff reconnects on the usual connect backoff of the peer.
	PrefixReconnectBackoff
	// PrefixReconnectTimer reconnects after `idle-timeout` seconds, doubled on
	// each repeat teardown and capped at one hour.
	PrefixReconnectTimer
)

// String returns the YANG enum spelling, which is also what the log line and
// the report bus show the operator.
func (m PrefixReconnectMode) String() string {
	switch m {
	case PrefixReconnectNever:
		return "never"
	case PrefixReconnectBackoff:
		return "backoff"
	case PrefixReconnectTimer:
		return "timer"
	case PrefixReconnectUnset:
		return "unset"
	}
	// The value itself, not a bare "unknown": a mode that reaches here is a bug,
	// and the number is what identifies which one. Also keeps `goconst` quiet
	// about a third "unknown" literal in this package.
	return textbuf.StrUintStr("unknown(", uint64(m), ")")
}

// ParsePrefixReconnectMode maps a YANG enum value to its mode. ok is false for
// any other string, which the config parser rejects rather than approximates.
func ParsePrefixReconnectMode(s string) (mode PrefixReconnectMode, ok bool) {
	switch s {
	case "never":
		return PrefixReconnectNever, true
	case "backoff":
		return PrefixReconnectBackoff, true
	case "timer":
		return PrefixReconnectTimer, true
	}
	return PrefixReconnectUnset, false
}

// PrefixReconnectFor resolves what the peer does after fam stopped the session
// for exceeding its prefix maximum. fam is an "afi/safi" string. It never
// returns PrefixReconnectUnset.
//
// A family that states `reconnect` gets what it asked for. A family that states
// only `idle-timeout N` with N above zero gets the timer, which is what every
// config written before the `reconnect` leaf existed means.
//
// Everything else, including a family the peer never configured, reads as
// NEVER. That is the fail-closed direction here (ai/rules/fail-closed-guards.md):
// a session stopped for flooding the RIB that comes straight back re-floods it,
// and the peer flaps until an operator notices. Staying down is visible, and it
// is what an operator gets from Cisco and from Juniper for the same event.
// A timer of zero seconds is not a timer. `parsePrefixReconnect`
// (config_prefix.go) rejects that pair, so a config cannot reach it, but a
// PeerSettings built in Go can. Timer with a zero wait would give Peer.run a
// delay of 0, and the doubling never leaves zero, so the peer would reconnect
// at once, re-exceed its maximum, and flap with no backoff at all. It resolves
// to NEVER here rather than in the parser alone, because this accessor is the
// one reader and the fail-closed decision belongs where it cannot be bypassed.
func (n *PeerSettings) PrefixReconnectFor(fam string) PrefixReconnectMode {
	if mode, ok := n.PrefixReconnect[fam]; ok && mode != PrefixReconnectUnset {
		if mode == PrefixReconnectTimer && n.PrefixIdleTimeout[fam] == 0 {
			return PrefixReconnectNever
		}
		return mode
	}
	if n.PrefixIdleTimeout[fam] > 0 {
		return PrefixReconnectTimer
	}
	return PrefixReconnectNever
}

// OldestPrefixUpdated returns the oldest per-family prefix `updated` date, in
// YYYY-MM-DD form, or "" when no family carries one.
//
// The peer-level surfaces keep one date each: the `prefix-updated` JSON key
// (internal/component/bgp/plugins/cmd/peer/peer.go), the prefix-stale report
// bus warning, and the ze_bgp_prefix_stale gauge. The oldest date is the
// conservative choice: the staleness alarm fires while any family is stale.
//
// Families are walked in sorted key order so a peer whose dates do not parse
// still reports the same value on every run. A value that does not parse loses
// to any value that does, and is returned only when no family parses.
func (n *PeerSettings) OldestPrefixUpdated() string {
	var oldest string
	var oldestTime time.Time
	var unparsed string

	for _, fam := range slices.Sorted(maps.Keys(n.PrefixUpdated)) {
		updated := n.PrefixUpdated[fam]
		if updated == "" {
			continue
		}
		t, err := time.Parse(time.DateOnly, updated)
		if err != nil {
			if unparsed == "" {
				unparsed = updated
			}
			continue
		}
		if oldest == "" || t.Before(oldestTime) {
			oldest, oldestTime = updated, t
		}
	}
	if oldest != "" {
		return oldest
	}
	return unparsed
}

// PeerKey returns the map key for this peer as a netip.AddrPort value type.
// This uniquely identifies a peer even when multiple peers share the same IP.
// Uses DefaultBGPPort when Port is zero (unset).
func (n *PeerSettings) PeerKey() netip.AddrPort {
	port := n.Port
	if port == 0 {
		port = DefaultBGPPort
	}
	return PeerKeyFromAddrPort(n.Address, port)
}

// PeerKeyFromAddrPort builds a peer map key from address and port.
// Returns a netip.AddrPort value type (20 bytes, comparable, zero allocation).
func PeerKeyFromAddrPort(addr netip.Addr, port uint16) netip.AddrPort {
	return netip.AddrPortFrom(addr, port)
}

// IsIBGP returns true if this is an internal BGP session (same AS).
func (n *PeerSettings) IsIBGP() bool {
	return n.LocalAS == n.PeerAS
}

// IsEBGP returns true if this is an external BGP session (different AS).
func (n *PeerSettings) IsEBGP() bool {
	return n.LocalAS != n.PeerAS
}

// EffectiveClusterID returns the cluster-id for route reflection.
// RFC 4456 Section 7: defaults to router-id when not explicitly configured.
func (n *PeerSettings) EffectiveClusterID() uint32 {
	if n.ClusterID != 0 {
		return n.ClusterID
	}
	return n.RouterID
}
