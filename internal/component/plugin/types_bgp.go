// Design: docs/architecture/api/process-protocol.md -- BGP reactor types
//
// BGP-specific types and interfaces for the reactor lifecycle. These types
// are used by the BGP reactor, Coordinator, plugin server, and BGP command
// handlers. They live in the plugin package (not a BGP-specific package)
// because the Coordinator and plugin server need them without importing
// BGP packages.
//
// Other protocols (OSPF, IS-IS) would add their own types_<protocol>.go
// file in this package following the same pattern.

package plugin

import (
	"context"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// PeerState represents the high-level state of a BGP peer.
type PeerState uint8

// The values mirror reactor.PeerState one for one, because reactor converts by
// value (reactor.PeerState.PluginState). A new state goes at the END of BOTH
// lists, never in the middle.
const (
	PeerStateStopped PeerState = iota
	PeerStateConnecting
	PeerStateActive
	PeerStateEstablished
	// PeerStateIdleHold means the peer is deliberately down: a prefix limit
	// stopped the session and the family that overflowed asked for no
	// reconnect, so only an operator brings the peer back.
	PeerStateIdleHold
)

func (s PeerState) String() string {
	switch s {
	case PeerStateStopped:
		return "stopped"
	case PeerStateConnecting:
		return "connecting"
	case PeerStateActive:
		return "active"
	case PeerStateEstablished:
		return "established"
	case PeerStateIdleHold:
		return "idle-hold"
	}
	return textbuf.StrIntStr("unknown(", int64(s), ")")
}

// PeerInfo is a snapshot of BGP peer state for API output.
//
// AddrStr and LocalAddrStr return the cached address strings with fallback.
// Use these methods instead of accessing AddressStr/LocalAddressStr directly
// so test code and edge cases (missing cache) produce correct output.
type PeerInfo struct {
	Address         netip.Addr
	LocalAddress    netip.Addr
	AddressStr      string // Cached Address.String(); avoids per-event allocation
	LocalAddressStr string // Cached LocalAddress.String(); avoids per-event allocation

	Name      string // Human-readable peer name for CLI selector
	GroupName string // Peer-group this peer belongs to
	LocalAS   uint32
	PeerAS    uint32
	// RouterID is THIS speaker's BGP Identifier for the session, as the peer's
	// configuration states it. It is not the peer's, and a comparison that owes
	// the peer's identifier MUST read RemoteRouterID below.
	RouterID uint32
	// RemoteRouterID is the peer's BGP Identifier, as its OPEN carried it
	// (reactor.Peer.RemoteRouterID). Zero until the OPEN is read and again
	// after teardown, which is why RFC 4271 Section 9.1.2.2 step f) treats a
	// zero as "no identifier" rather than as the address 0.0.0.0.
	RemoteRouterID  uint32
	ReceiveHoldTime time.Duration // Configured receive hold time (RFC 4271)
	SendHoldTime    time.Duration // Configured send hold time (RFC 9687, 0=auto)
	KeepaliveTime   time.Duration // Configured keepalive interval (RFC 4271, 0=hold/3)
	ConnectRetry    time.Duration // Connect retry interval
	Connect         bool          // Initiate outbound connections
	Accept          bool          // Accept inbound connections
	State           PeerState
	Uptime          time.Duration

	// Route reflection (RFC 4456).
	RouteReflectorClient bool   // Peer is an RR client
	ClusterID            uint32 // Explicit cluster-id (0 = use router-id)

	// Next-hop mode for forwarded UPDATEs (RFC 4271 Section 5.1.3).
	// Values: 0=auto, 1=self, 2=unchanged, 3=explicit.
	NextHopMode    uint8
	NextHopAddress netip.Addr // Only when NextHopMode == 3 (explicit)

	// PrefixUpdated is the ISO date (YYYY-MM-DD) when prefix maximums were
	// last updated from PeeringDB. Empty means manually configured.
	// Active prefix-threshold and prefix-stale warnings live on the report
	// bus (internal/core/report), not on this struct.
	PrefixUpdated string

	// Policy filter chains (after group inheritance + canonicalization).
	ImportFilters []string
	ExportFilters []string

	// Statistics (engine-level counters; NLRI-level counters live in the RIB plugin)
	UpdatesReceived    uint32
	UpdatesSent        uint32
	KeepalivesReceived uint32
	KeepalivesSent     uint32
	EORReceived        uint32
	EORSent            uint32

	// Full message counters (all message types).
	OpensReceived         uint32
	OpensSent             uint32
	NotificationsReceived uint32
	NotificationsSent     uint32
	RefreshReceived       uint32
	RefreshSent           uint32

	// Lifetime session stability counters.
	ConnectionsEstablished uint32
	ConnectionsDropped     uint32
	FlapCount              uint32

	// ConnectRetryCounter is RFC 4271 §8.1.1 mandatory session attribute 2,
	// "the number of times a BGP peer has tried to establish a peer session".
	// Raised by the FSM on every teardown §8.2.2 gives an increment clause,
	// zeroed only by an operator start or stop.
	ConnectRetryCounter uint32

	// Last notification details (lifetime, survives session reset).
	LastNotifCode    uint8
	LastNotifSubcode uint8
	LastNotifRecv    bool
	LastNotifTime    time.Time

	// LastStateChange is the time of the peer's most recent FSM transition.
	// Zero means the peer has never transitioned. Unlike Uptime (derived from
	// EstablishedAt, which ClearStats zeroes on teardown) this survives a
	// session going down, so it can answer "when did this peer last change
	// state" for a peer that is currently DOWN.
	LastStateChange time.Time

	// Activity timestamps.
	LastReadTime  time.Time
	LastWriteTime time.Time

	// Peer type derived from LocalAS vs PeerAS.
	PeerType string // "internal" or "external"

	// Transport details.
	LocalPort  uint16
	RemotePort uint16
	MD5Enabled bool
	BFDEnabled bool

	// GTSM / TTL security (RFC 5082). Zero means the option is not
	// configured for this peer. GTSMOutTTL is the outgoing IP TTL /
	// IPv6 Hop Limit; GTSMMinTTL is the minimum accepted inbound TTL.
	GTSMOutTTL uint8
	GTSMMinTTL uint8

	// RFC 4271 Section 4.2: negotiated timers (min of local and remote).
	NegotiatedHoldTime      time.Duration
	NegotiatedKeepaliveTime time.Duration

	// Inline capabilities (negotiated).
	NegotiationComplete    bool
	NegotiatedASN4         bool
	NegotiatedExtMsg       bool
	NegotiatedRouteRefresh bool
	NegotiatedEnhancedRR   bool
	NegotiatedAddPath      map[string]string // family -> "send"/"receive"/"both"

	// RFC 4724: Graceful restart state.
	GracefulRestart bool
	GRRestartTime   uint16

	// NegotiatedFamilies is the list of address families that completed
	// RFC 4760 multiprotocol negotiation with this peer. Empty until OPEN
	// exchange finishes. Used by `show bgp <family> summary` to scope the
	// summary table to peers carrying a given address family.
	NegotiatedFamilies []family.Family
}

// AddrStr returns the cached address string, falling back to Address.String().
func (p *PeerInfo) AddrStr() string {
	if p.AddressStr != "" {
		return p.AddressStr
	}
	return p.Address.String()
}

// LocalAddrStr returns the cached local address string, falling back to LocalAddress.String().
func (p *PeerInfo) LocalAddrStr() string {
	if p.LocalAddressStr != "" {
		return p.LocalAddressStr
	}
	return p.LocalAddress.String()
}

// PeerCapabilityConfig holds BGP capability configuration for a peer.
// Used by plugin protocol Stage 2 to deliver matching config.
// Values is a flexible map allowing any capability to be represented.
type PeerCapabilityConfig struct {
	Address        string            // Peer IP address
	Values         map[string]string // capability-name -> value (e.g., "hostname" -> "router1.example.com")
	CapabilityJSON string            // Full capability block as JSON - plugins extract what they need
}

// ReactorStats holds BGP reactor-level statistics.
type ReactorStats struct {
	StartTime time.Time
	Uptime    time.Duration
	PeerCount int
	RouterID  uint32 // Local BGP router identifier (uint32 IP)
	LocalAS   uint32 // Local AS number
}

// PeerCapabilitiesInfo holds negotiated and configured BGP capability data for API display.
type PeerCapabilitiesInfo struct {
	Families             []string          // Negotiated address families (e.g., "ipv4/unicast")
	ExtendedMessage      bool              // RFC 8654: Extended message support
	EnhancedRouteRefresh bool              // RFC 7313: Enhanced route refresh
	ASN4                 bool              // RFC 6793: 4-byte ASN support
	AddPath              map[string]string // RFC 7911: family -> "send" for families with ADD-PATH (nil if none)
}

// ReactorIntrospector provides read-only access to BGP peer and reactor state.
type ReactorIntrospector interface {
	// Peers returns information about all configured peers.
	Peers() []PeerInfo

	// Stats returns reactor-level statistics.
	Stats() ReactorStats

	// PeerNegotiatedCapabilities returns negotiated capabilities for a peer.
	// Returns nil if peer not found or negotiation not complete.
	PeerNegotiatedCapabilities(addr netip.Addr) *PeerCapabilitiesInfo

	// GetPeerProcessBindings returns process bindings for a specific peer.
	GetPeerProcessBindings(peerAddr netip.Addr) []PeerProcessBinding

	// GetPeerCapabilityConfigs returns capability configurations for all peers.
	GetPeerCapabilityConfigs() []PeerCapabilityConfig
}

// PolicyTraceEntry holds the result of one filter invocation during a dry-run.
type PolicyTraceEntry struct {
	Filter    string `json:"filter"`
	Canonical string `json:"canonical"`
	Action    string `json:"action"`
	Delta     string `json:"delta,omitempty"`
	TextAfter string `json:"text-after"`
}

// PolicyDryRunResult holds the complete output of a policy dry-run.
type PolicyDryRunResult struct {
	DataMarker
	Direction    string             `json:"direction"`
	Peer         string             `json:"peer"`
	Action       string             `json:"action"`
	Trace        []PolicyTraceEntry `json:"trace"`
	TextBefore   string             `json:"text-before"`
	TextAfter    string             `json:"text-after"`
	ChangedAttrs []string           `json:"changed-attrs,omitempty"`
	// WireChanges lists the wire-level attribute modification operations the
	// policy would apply, mirroring the runtime egress/ingress text-to-wire
	// path. It includes effects the flat filter text cannot express, notably
	// AS4_PATH suppression/rewrite by remove-private-as (RFC 6996/6793).
	// Each entry has the form "<attribute> <verb>", e.g. "AS4_PATH suppressed"
	// or "AS_PATH set". Text-visible changes (e.g. MED) also appear here as
	// their wire operation, complementing the text view in ChangedAttrs.
	WireChanges []string `json:"wire-changes,omitempty"`
}

// PolicyDryRunner is an optional interface for reactors that support
// read-only policy dry-run testing. Command handlers type-assert to this.
type PolicyDryRunner interface {
	PolicyDryRun(peerAddr, direction, filterOverride string, updatePayload []byte, asn4 bool) (*PolicyDryRunResult, error)
}

// FSMTransitionRecord is one peer FSM state change for diagnostic display.
type FSMTransitionRecord struct {
	Timestamp time.Time
	From      string
	To        string
	Reason    string
}

// FSMHistoryProvider is an optional interface for reactors that track
// per-peer FSM transition history. Handlers type-assert to this to
// avoid widening ReactorLifecycle.
type FSMHistoryProvider interface {
	PeerFSMHistory(addr string) []FSMTransitionRecord
}

// BGPCaptureRecord is one BGP message for diagnostic display.
type BGPCaptureRecord struct {
	Timestamp string `json:"timestamp"`
	Direction string `json:"direction"`
	PeerAddr  string `json:"peer-addr"`
	MsgType   string `json:"msg-type"`
	ByteCount int    `json:"byte-count"`
	ErrorCode int    `json:"error-code,omitempty"`
	ErrorSub  int    `json:"error-sub,omitempty"`
}

// BGPCaptureProvider is an optional interface for reactors that capture
// BGP messages. Handlers type-assert to this.
type BGPCaptureProvider interface {
	BGPCaptureSnapshot(limit int, peer string) []BGPCaptureRecord
}

// BGPRawCaptureProvider is an optional interface for reactors that support
// raw byte capture for pcap export. Handlers type-assert to this.
type BGPRawCaptureProvider interface {
	EnableRawCapture()
	DisableRawCapture()
	BGPRawCaptureSnapshot(limit int) []BGPRawCaptureEntry
}

// BGPRawCaptureEntry is one raw captured BGP message.
type BGPRawCaptureEntry struct {
	Timestamp string `json:"timestamp"`
	Direction string `json:"direction"`
	Data      []byte `json:"data"`
}

// ReactorPeerController manages BGP peer lifecycle: shutdown, teardown,
// flow control, and dynamic peer add/remove.
type ReactorPeerController interface {
	// Stop signals the reactor to shut down.
	Stop()

	// TeardownPeer gracefully closes a peer session with NOTIFICATION.
	// RFC 4486: Cease subcodes (2=Admin Shutdown, 3=Peer De-configured, 4=Admin Reset).
	// RFC 8203: shutdownMsg is included for subcodes 2/4 (empty = default message).
	TeardownPeer(addr netip.Addr, subcode uint8, shutdownMsg string) error

	// PausePeer pauses reading from a specific peer's session.
	// Used by flow control to apply backpressure when a plugin's worker pool saturates.
	PausePeer(addr netip.Addr) error

	// ResumePeer resumes reading from a specific peer's session.
	// Used by flow control to release backpressure when a plugin's worker pool drains.
	ResumePeer(addr netip.Addr) error

	// AddDynamicPeer adds a peer from a YANG-parsed config tree.
	// The addr is from the peer selector; tree is the peer-fields config.
	// Calls parsePeerFromTree directly (not the reload pipeline).
	AddDynamicPeer(addr netip.Addr, tree map[string]any) error

	// RemovePeer removes a peer by address.
	RemovePeer(addr netip.Addr) error

	// FlushForwardPool blocks until all forward pool workers have drained their
	// queued items to peer sockets. Used by plugins to ensure route delivery
	// before proceeding with dependent operations (e.g., teardown, withdraw).
	FlushForwardPool(ctx context.Context) error

	// FlushForwardPoolPeer blocks until the forward pool worker for a specific
	// peer address has drained its queued items. Returns nil immediately if no
	// worker exists for that peer.
	FlushForwardPoolPeer(ctx context.Context, addr string) error

	// DrainPeerSync blocks until every Established peer has finished initial
	// route sync (its opQueue drained and sendingInitialRoutes cleared). Peers
	// not yet Established are skipped. Complements FlushForwardPool: routes sent
	// during a peer's initial-sync window bypass the forward pool (they drain
	// direct to the session), so a complete "routes on the wire" barrier needs
	// both. Returns ctx.Err() if the deadline hits first.
	DrainPeerSync(ctx context.Context) error
}

// ReactorCacheCoordinator manages BGP cache consumer registration, forwarding,
// and release. The fast-path methods (ForwardUpdatesDirect, ReleaseUpdates)
// bypass the text-command tokenise path used by the legacy update-route RPC.
type ReactorCacheCoordinator interface {
	// RegisterCacheConsumer initializes tracking for a cache-consumer plugin.
	// unordered=false: FIFO consumer (cumulative ack -- existing behavior).
	// unordered=true: per-entry ack only, no cumulative sweep. Required for
	// consumers like bgp-rs that process entries out of global message ID order.
	// Called when a plugin declares cache-consumer: true during Stage 1 registration.
	RegisterCacheConsumer(name string, unordered bool)

	// UnregisterCacheConsumer removes a cache-consumer plugin and adjusts pending counts.
	// Called when a cache-consumer plugin disconnects or exits.
	UnregisterCacheConsumer(name string)

	// ForwardUpdatesDirect forwards a batch of cached UPDATEs to an explicit
	// destination list, one cached update per ID. Reuses the same per-destination
	// pipeline as ForwardUpdate (egress filters, EBGP wire cache, copy-on-modify).
	// pluginName identifies the cache consumer; after forwarding each id is acked
	// for that consumer (FIFO or unordered per the consumer registration).
	// Returns error only if NO id could be dispatched at all -- missing ids log a
	// BUG warning and continue (rs-fastpath-3 AC-7a).
	//
	// sender is the authority, and it is a different question from pluginName:
	// each destination must attach it with `send [ update ]`, because a full
	// UPDATE lands on that destination's wire
	// (bgp/reactor/send_permission.go). Destinations that grant nothing are
	// dropped and reported; a batch every destination refused is an error.
	ForwardUpdatesDirect(updateIDs []uint64, destinations []netip.AddrPort, pluginName string, sender Sender) error

	// ReleaseUpdates acks a batch of cached UPDATEs for pluginName without
	// forwarding. Used when the plugin decided not to forward (e.g. empty target
	// list). Missing ids log and continue.
	ReleaseUpdates(updateIDs []uint64, pluginName string) error
}

// ReactorRelayCoordinator relays routes a plugin stores as raw wire bytes to a
// single newly-established peer, through the same egress pipeline a live forward
// uses. spec-fixit-bgp-egress-rail-divergence.
//
// Kept separate from ReactorCacheCoordinator rather than folded into it: a cache
// coordinator relays UPDATEs the engine still holds by id, while this relays
// bytes the PLUGIN holds after the cache has dropped them. The two have different
// lifetimes and different implementers, so a caller should be able to depend on
// one without the other (interface segregation, ai/rules/architecture.md).
type ReactorRelayCoordinator interface {
	// RelayStoredRoute relays each stored route to destination through the
	// forward rail, applying the egress transform that route's SOURCE peer
	// implies (AS_PATH prepend, role/OTC, export policy).
	//
	// Returns an error when destination resolves to no established peer or no
	// route could be relayed; per-route failures are logged and do not fail the
	// call. An empty routes slice is a success no-op.
	//
	// sender is the authority: the destination must attach it with
	// `send [ update ]`, because each relayed route leaves as an UPDATE on that
	// peer's wire (bgp/reactor/send_permission.go). One destination, so the
	// permission is all or nothing and a refusal is an error.
	RelayStoredRoute(destination netip.Addr, routes []rpc.StoredRoute, sender Sender) error
}

// ReactorLifecycle is the full BGP reactor interface composed from focused
// sub-interfaces. It extends ProtocolReactor with BGP-specific peer management,
// introspection, and cache coordination.
//
// Consumers should prefer the narrowest sub-interface that satisfies their needs.
// Non-BGP code should use ProtocolReactor instead.
type ReactorLifecycle interface {
	ReactorIntrospector
	ReactorPeerController
	ReactorConfigurator
	ReactorStartupCoordinator
	ReactorCacheCoordinator
	// ReactorRelayCoordinator is composed here on purpose. The plugin server
	// holds a ReactorLifecycle (often the Coordinator facade) and reaches the
	// relay through a type assertion; leaving relay out of the composed
	// interface let the facade compile without the method and fail that
	// assertion at RUNTIME, degrading every peer-up replay to a warning.
	// Composing it makes a missing delegation a compile error.
	ReactorRelayCoordinator
}

// PeerProcessBinding describes which plugin receives messages from a BGP peer.
// It mirrors reactor.ProcessBinding with the content settings already
// resolved.
type PeerProcessBinding struct {
	PluginName string // Reference to plugin name

	// Content settings (HOW messages are formatted)
	Encoding string // "json" | "text" (empty = inherit from plugin)
	Format   string // "parsed" | "raw" | "full" (empty = "parsed")

	// Receive settings (WHAT the peer feeds this process).
	// ReceiveAll is the "*" token. Receive maps each granted event type to the
	// direction it is granted in; a type with no key is not granted.
	ReceiveAll bool
	Receive    map[string]events.Direction

	// Send settings (WHAT this process may generate toward the peer).
	// SendAll is the "*" token.
	SendAll bool
	Send    map[string]bool
}
