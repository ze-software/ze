// Package reactor implements the BGP reactor - the main orchestrator
// that manages peer sessions, connections, and signal handling.
package reactor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/exa-networks/zebgp/pkg/api"
	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/fsm"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/exa-networks/zebgp/pkg/rib"
	"github.com/exa-networks/zebgp/pkg/trace"
)

// Reactor errors.
var (
	ErrAlreadyRunning        = errors.New("reactor already running")
	ErrNotRunning            = errors.New("reactor not running")
	ErrPeerExists            = errors.New("peer already exists")
	ErrPeerNotFound          = errors.New("peer not found")
	ErrWatchdogRouteNotFound = errors.New("watchdog route not found")
)

// Address family names for error messages.
const (
	familyIPv4 = "IPv4"
	familyIPv6 = "IPv6"
)

// Config holds reactor configuration.
type Config struct {
	// ListenAddr is the address to listen on (e.g., "0.0.0.0:179").
	//
	// Deprecated: Use Port with per-peer LocalAddress instead.
	ListenAddr string

	// Port is the BGP port to listen on (default 179).
	// Used with per-peer LocalAddress to create listeners.
	Port int

	// RouterID is the local BGP router identifier.
	RouterID uint32

	// LocalAS is the local AS number.
	LocalAS uint32

	// APISocketPath is the path to the Unix socket for API communication.
	// If empty, API server is not started.
	APISocketPath string

	// APIProcesses defines external processes for API communication.
	APIProcesses []APIProcessConfig

	// ConfigDir is the directory containing the config file.
	// Used as working directory for process execution.
	ConfigDir string

	// RecentUpdateTTL is how long update-ids remain valid for forwarding.
	// Default: 60s. Zero disables caching (forwarding won't work).
	RecentUpdateTTL time.Duration

	// RecentUpdateMax is the maximum number of cached updates.
	// Default: 100000. Zero means no limit (not recommended).
	RecentUpdateMax int
}

// APIProcessConfig holds external process configuration for the API.
type APIProcessConfig struct {
	Name          string
	Run           string
	Encoder       string
	Respawn       bool
	ReceiveUpdate bool // Forward received UPDATEs to process stdin
}

// Stats holds reactor statistics.
type Stats struct {
	StartTime time.Time
	Uptime    time.Duration
	PeerCount int
}

// ConnectionCallback is called when a connection is matched to a peer.
type ConnectionCallback func(conn net.Conn, settings *PeerSettings)

// MessageReceiver receives raw BGP messages from peers.
// Messages are passed as raw wire bytes for on-demand parsing based on format config.
type MessageReceiver interface {
	// OnMessageReceived is called when a BGP message is received from a peer.
	// peer contains full peer information for proper JSON encoding.
	// msg contains raw wire bytes - parsing is done by receiver based on format config.
	OnMessageReceived(peer api.PeerInfo, msg api.RawMessage)

	// OnMessageSent is called when a BGP message is sent to a peer.
	// Only UPDATE messages trigger sent events.
	// Used by persist plugin to track routes for replay on reconnect.
	OnMessageSent(peer api.PeerInfo, msg api.RawMessage)
}

// PeerLifecycleObserver receives peer state change notifications.
// Observers are called synchronously in registration order.
// Implementations MUST NOT block; use goroutine for slow processing.
type PeerLifecycleObserver interface {
	OnPeerEstablished(peer *Peer)
	OnPeerClosed(peer *Peer, reason string)
}

// apiStateObserver emits ExaBGP-compatible state messages to API server.
type apiStateObserver struct {
	server  *api.Server
	reactor *Reactor
}

func (o *apiStateObserver) OnPeerEstablished(peer *Peer) {
	if o.server == nil {
		return
	}
	s := peer.Settings()
	peerInfo := api.PeerInfo{
		Address:      s.Address,
		LocalAddress: s.LocalAddress,
		LocalAS:      s.LocalAS,
		PeerAS:       s.PeerAS,
		RouterID:     s.RouterID,
		State:        peer.State().String(),
	}
	o.server.OnPeerStateChange(peerInfo, "up")
}

func (o *apiStateObserver) OnPeerClosed(peer *Peer, reason string) {
	if o.server == nil {
		return
	}
	s := peer.Settings()
	peerInfo := api.PeerInfo{
		Address:      s.Address,
		LocalAddress: s.LocalAddress,
		LocalAS:      s.LocalAS,
		PeerAS:       s.PeerAS,
		RouterID:     s.RouterID,
		State:        peer.State().String(),
	}
	o.server.OnPeerStateChange(peerInfo, "down")
}

// Reactor is the main BGP orchestrator.
//
// It manages:
//   - Peer connections (outgoing)
//   - Listener for incoming connections
//   - Signal handling
//   - Graceful shutdown
//   - API server for external communication
//   - RIB (Routing Information Base) for route storage
//   - Watchdog pools for API-controlled route groups
type Reactor struct {
	config *Config

	peers     map[string]*Peer         // keyed by peer address
	listener  *Listener                // deprecated: single listener for backward compat
	listeners map[netip.Addr]*Listener // one listener per unique LocalAddress
	signals   *SignalHandler
	api       *api.Server // API server for CLI and external processes

	// RIB components
	ribIn    *rib.IncomingRIB // Adj-RIB-In
	ribStore *rib.RouteStore  // Global deduplication store

	// Watchdog pools for API-created routes
	watchdog *WatchdogManager

	// Recent UPDATE cache for efficient forwarding via update-id
	recentUpdates *RecentUpdateCache

	connCallback    ConnectionCallback
	messageReceiver MessageReceiver // Receives raw BGP messages

	// Peer lifecycle observers (called on state transitions)
	peerObservers []PeerLifecycleObserver
	observersMu   sync.RWMutex

	running   bool
	startTime time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex

	// API process synchronization state.
	// Embedded to access fields directly (e.g., r.apiStarted).
	APISyncState
}

// reactorAPIAdapter implements api.ReactorInterface for the Reactor.
type reactorAPIAdapter struct {
	r *Reactor
}

// Peers returns peer information for the API.
func (a *reactorAPIAdapter) Peers() []api.PeerInfo {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	result := make([]api.PeerInfo, 0, len(a.r.peers))
	for _, p := range a.r.peers {
		s := p.Settings()
		info := api.PeerInfo{
			Address:      s.Address,
			LocalAddress: netip.Addr{}, // TODO: get from session
			LocalAS:      s.LocalAS,
			PeerAS:       s.PeerAS,
			RouterID:     s.RouterID,
			State:        p.State().String(),
		}
		if p.State() == PeerStateEstablished {
			info.Uptime = time.Since(a.r.startTime) // TODO: track per-peer
		}
		result = append(result, info)
	}
	return result
}

// Stats returns reactor statistics for the API.
func (a *reactorAPIAdapter) Stats() api.ReactorStats {
	stats := a.r.Stats()
	return api.ReactorStats{
		StartTime: stats.StartTime,
		Uptime:    stats.Uptime,
		PeerCount: stats.PeerCount,
	}
}

// Stop signals the reactor to stop.
func (a *reactorAPIAdapter) Stop() {
	a.r.Stop()
}

// AnnounceRoute announces a route to matching peers.
// If a peer is in transaction, queues to its Adj-RIB-Out; otherwise sends immediately.
func (a *reactorAPIAdapter) AnnounceRoute(peerSelector string, route api.RouteSpec) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Build NLRI
	var n nlri.NLRI
	if route.Prefix.Addr().Is4() {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
	} else {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
	}

	// Build attributes from RouteSpec
	// Order matches buildAnnounceUpdate: ORIGIN, MED, LOCAL_PREF, communities
	var attrs []attribute.Attribute

	// 1. ORIGIN
	if route.Origin != nil {
		attrs = append(attrs, attribute.Origin(*route.Origin))
	} else {
		attrs = append(attrs, attribute.OriginIGP)
	}

	// 2. MED (RFC 4271 §5.1.4: optional non-transitive)
	if route.MED != nil {
		attrs = append(attrs, attribute.MED(*route.MED))
	}

	// 3. LOCAL_PREF (will be filtered by isIBGP check at send time)
	if route.LocalPreference != nil {
		attrs = append(attrs, attribute.LocalPref(*route.LocalPreference))
	}

	// 4. Communities
	if len(route.Communities) > 0 {
		comms := make(attribute.Communities, len(route.Communities))
		for i, c := range route.Communities {
			comms[i] = attribute.Community(c)
		}
		attrs = append(attrs, comms)
	}

	// 5. Large Communities
	if len(route.LargeCommunities) > 0 {
		attrs = append(attrs, attribute.LargeCommunities(route.LargeCommunities))
	}

	// 6. Extended Communities
	if len(route.ExtendedCommunities) > 0 {
		attrs = append(attrs, attribute.ExtendedCommunities(route.ExtendedCommunities))
	}

	var lastErr error
	for _, peer := range peers {
		isIBGP := peer.Settings().IsIBGP()

		// Build AS_PATH: empty for iBGP, prepend LocalAS for eBGP
		// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS
		var asPath *attribute.ASPath
		switch {
		case len(route.ASPath) > 0:
			asPath = &attribute.ASPath{
				Segments: []attribute.ASPathSegment{
					{Type: attribute.ASSequence, ASNs: route.ASPath},
				},
			}
		case isIBGP:
			asPath = &attribute.ASPath{Segments: nil}
		default:
			asPath = &attribute.ASPath{
				Segments: []attribute.ASPathSegment{
					{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
				},
			}
		}

		ribRoute := rib.NewRouteWithASPath(n, route.NextHop, attrs, asPath)

		if peer.State() == PeerStateEstablished {
			// Send immediately
			// RFC 7911: Get PackContext for ADD-PATH encoding
			// RFC 6793: ctx.ASN4 provides 4-byte AS capability
			ctx := peer.packContext(n.Family())
			update := buildAnnounceUpdate(route, a.r.config.LocalAS, isIBGP, ctx)
			if err := peer.SendUpdate(update); err != nil {
				lastErr = err
			}
		} else {
			// Session not established: queue to peer's operation queue
			// This maintains order with any pending teardowns
			peer.QueueAnnounce(ribRoute)
		}
	}
	return lastErr
}

// WithdrawRoute withdraws a route from matching peers.
func (a *reactorAPIAdapter) WithdrawRoute(peerSelector string, prefix netip.Prefix) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Build NLRI for queueing
	var n nlri.NLRI
	if prefix.Addr().Is4() {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
	} else {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, prefix, 0)
	}

	var lastErr error
	for _, peer := range peers {
		if peer.State() == PeerStateEstablished {
			// Send immediately
			// RFC 7911: Get PackContext for ADD-PATH encoding
			ctx := peer.packContext(n.Family())
			update := buildWithdrawUpdate(prefix, ctx)
			if err := peer.SendUpdate(update); err != nil {
				lastErr = err
			}
		} else {
			// Session not established: queue to peer's operation queue
			// This maintains order with any pending announces/teardowns
			peer.QueueWithdraw(n)
		}
	}
	return lastErr
}

// sendToMatchingPeers sends an UPDATE to peers matching the selector.
// Supports: "*" (all peers), exact IP, or glob patterns (e.g., "192.168.*.*").
func (a *reactorAPIAdapter) sendToMatchingPeers(selector string, update *message.Update) error {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	var lastErr error
	sentCount := 0

	for addrStr, peer := range a.r.peers {
		// Check if this peer matches the selector using glob matching
		if !ipGlobMatch(selector, addrStr) {
			continue
		}

		// Only send to established peers
		if peer.State() != PeerStateEstablished {
			continue
		}

		if err := peer.SendUpdate(update); err != nil {
			lastErr = err
		} else {
			sentCount++
		}
	}

	if sentCount == 0 && lastErr == nil {
		// No peers matched or were established
		return errors.New("no established peers to send to")
	}

	return lastErr
}

// buildAnnounceUpdate builds an UPDATE message for announcing a route.
// Uses attributes from RouteSpec if provided, otherwise uses sensible defaults.
//
// Attribute ordering follows RFC 4271 Section 5 (sorted by type code):
//
//  1. ORIGIN (type 1) - RFC 4271 §5.1.1
//  2. AS_PATH (type 2) - RFC 4271 §5.1.2
//  3. NEXT_HOP (type 3) - RFC 4271 §5.1.3
//  4. MED (type 4) - RFC 4271 §5.1.4
//  5. LOCAL_PREF (type 5) - RFC 4271 §5.1.5
//  6. COMMUNITY (type 8) - RFC 1997
//  7. LARGE_COMMUNITY (type 32) - RFC 8092
//
// RFC 7911: ctx provides ADD-PATH capability state for NLRI encoding.
// RFC 6793: ctx.ASN4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
func buildAnnounceUpdate(route api.RouteSpec, localAS uint32, isIBGP bool, ctx *nlri.PackContext) *message.Update {
	// Build path attributes
	var attrBytes []byte

	// Extract ASN4 from context (default to true if nil)
	asn4 := ctx == nil || ctx.ASN4

	// 1. ORIGIN - RFC 4271 §5.1.1: Well-known mandatory attribute.
	// Default to IGP (0) for locally originated routes.
	if route.Origin != nil {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.Origin(*route.Origin))...)
	} else {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.OriginIGP)...)
	}

	// 2. AS_PATH - RFC 4271 §5.1.2: Well-known mandatory attribute.
	// RFC 4271 §5.1.2(a): "When a given BGP speaker advertises the route to an
	// internal peer, the advertising speaker SHALL NOT modify the AS_PATH".
	var asPath *attribute.ASPath
	switch {
	case len(route.ASPath) > 0:
		// Use explicit AS_PATH from route
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: route.ASPath},
			},
		}
	case isIBGP:
		// Empty AS_PATH for iBGP self-originated routes
		asPath = &attribute.ASPath{Segments: nil}
	default:
		// RFC 4271 §5.1.2(b): Prepend local AS for eBGP
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	isIPv6 := route.Prefix.Addr().Is6()

	// 3. NEXT_HOP - RFC 4271 §5.1.3: Well-known mandatory attribute.
	// RFC 4760: For non-IPv4 unicast, next-hop is carried in MP_REACH_NLRI.
	if !isIPv6 {
		nextHop := &attribute.NextHop{Addr: route.NextHop}
		attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)
	}

	// 4. MED - RFC 4271 §5.1.4: Optional non-transitive attribute.
	// Only include if explicitly specified.
	if route.MED != nil {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.MED(*route.MED))...)
	}

	// 5. LOCAL_PREF - RFC 4271 §5.1.5: Well-known attribute for iBGP only.
	// RFC 4271: "A BGP speaker MUST NOT include this attribute in UPDATE
	// messages it sends to external peers". User-specified LOCAL_PREF for
	// eBGP sessions is silently ignored per RFC.
	if isIBGP {
		if route.LocalPreference != nil {
			attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(*route.LocalPreference))...)
		} else {
			attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
		}
	}

	// 6. COMMUNITY - RFC 1997: Optional transitive attribute.
	if len(route.Communities) > 0 {
		comms := make(attribute.Communities, len(route.Communities))
		for i, c := range route.Communities {
			comms[i] = attribute.Community(c)
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(comms)...)
	}

	// 7. LARGE_COMMUNITY - RFC 8092: Optional transitive attribute.
	if len(route.LargeCommunities) > 0 {
		lcomms := attribute.LargeCommunities(route.LargeCommunities)
		attrBytes = append(attrBytes, attribute.PackAttribute(lcomms)...)
	}

	// 8. EXTENDED_COMMUNITIES - RFC 4360: Optional transitive attribute.
	if len(route.ExtendedCommunities) > 0 {
		extComms := attribute.ExtendedCommunities(route.ExtendedCommunities)
		attrBytes = append(attrBytes, attribute.PackAttribute(extComms)...)
	}

	// Build NLRI
	// RFC 7911: Pack uses ADD-PATH encoding when negotiated
	var nlriBytes []byte
	if !isIPv6 {
		// IPv4: Use regular NLRI field
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
		nlriBytes = inet.Pack(ctx)
	} else {
		// IPv6: Use MP_REACH_NLRI attribute (RFC 4760)
		// Build NLRI bytes for the prefix
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
		mpReach := &attribute.MPReachNLRI{
			AFI:      attribute.AFIIPv6,
			SAFI:     attribute.SAFIUnicast,
			NextHops: []netip.Addr{route.NextHop},
			NLRI:     inet.Pack(ctx),
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(mpReach)...)
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}
}

// buildWithdrawUpdate builds an UPDATE message for withdrawing a route.
// RFC 4760 Section 4: IPv6 withdrawals use MP_UNREACH_NLRI attribute.
// RFC 7911: ctx provides ADD-PATH capability state for NLRI encoding.
func buildWithdrawUpdate(prefix netip.Prefix, ctx *nlri.PackContext) *message.Update {
	if prefix.Addr().Is4() {
		// IPv4: Use WithdrawnRoutes field
		// RFC 7911: Pack uses ADD-PATH encoding when negotiated
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
		return &message.Update{
			WithdrawnRoutes: inet.Pack(ctx),
		}
	}

	// IPv6: Use MP_UNREACH_NLRI attribute (RFC 4760 Section 4)
	// RFC 7911: Pack uses ADD-PATH encoding when negotiated
	inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, prefix, 0)
	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFIIPv6,
		SAFI: attribute.SAFIUnicast,
		NLRI: inet.Pack(ctx),
	}

	return &message.Update{
		PathAttributes: attribute.PackAttribute(mpUnreach),
	}
}

// Reload reloads the configuration.
// TODO: Implement full configuration reload.
func (a *reactorAPIAdapter) Reload() error {
	// For now, just return success.
	// Full implementation would:
	// 1. Parse new config file
	// 2. Diff with current config
	// 3. Add/remove peers
	// 4. Update peer settings
	return nil
}

// AnnounceFlowSpec announces a FlowSpec route to matching peers.
// TODO: Implement when FlowSpec RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceFlowSpec(_ string, _ api.FlowSpecRoute) error {
	return errors.New("flowspec: not implemented")
}

// WithdrawFlowSpec withdraws a FlowSpec route from matching peers.
// TODO: Implement when FlowSpec RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawFlowSpec(_ string, _ api.FlowSpecRoute) error {
	return errors.New("flowspec: not implemented")
}

// AnnounceVPLS announces a VPLS route to matching peers.
// TODO: Implement when VPLS RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceVPLS(_ string, _ api.VPLSRoute) error {
	return errors.New("vpls: not implemented")
}

// WithdrawVPLS withdraws a VPLS route from matching peers.
// TODO: Implement when VPLS RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawVPLS(_ string, _ api.VPLSRoute) error {
	return errors.New("vpls: not implemented")
}

// AnnounceL2VPN announces an L2VPN/EVPN route to matching peers.
// TODO: Implement when L2VPN/EVPN RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceL2VPN(_ string, _ api.L2VPNRoute) error {
	return errors.New("l2vpn: not implemented")
}

// WithdrawL2VPN withdraws an L2VPN/EVPN route from matching peers.
// TODO: Implement when L2VPN/EVPN RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawL2VPN(_ string, _ api.L2VPNRoute) error {
	return errors.New("l2vpn: not implemented")
}

// AnnounceL3VPN announces an L3VPN (MPLS VPN) route to matching peers.
// TODO: Implement when L3VPN RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceL3VPN(_ string, _ api.L3VPNRoute) error {
	return errors.New("l3vpn: not implemented")
}

// WithdrawL3VPN withdraws an L3VPN route from matching peers.
// TODO: Implement when L3VPN RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawL3VPN(_ string, _ api.L3VPNRoute) error {
	return errors.New("l3vpn: not implemented")
}

// AnnounceLabeledUnicast announces an MPLS labeled unicast route (SAFI 4).
// RFC 8277 - Using BGP to Bind MPLS Labels to Address Prefixes.
//
// Supports three modes like AnnounceRoute:
//   - Transaction mode: queues to Adj-RIB-Out (sent on commit)
//   - Established: sends immediately and tracks for re-announcement
//   - Not established: queues to peer's operation queue.
func (a *reactorAPIAdapter) AnnounceLabeledUnicast(peerSelector string, route api.LabeledUnicastRoute) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	var lastErr error
	for _, peer := range peers {
		isIBGP := peer.Settings().IsIBGP()

		// Build rib.Route with ALL attributes (not just Origin like AnnounceRoute bug)
		ribRoute := a.buildLabeledUnicastRIBRoute(route, isIBGP)

		if peer.State() == PeerStateEstablished {
			// Send immediately
			family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
			if route.Prefix.Addr().Is6() {
				family.AFI = nlri.AFIIPv6
			}
			ctx := peer.packContext(family)

			// Build UPDATE using UpdateBuilder for immediate send
			ub := message.NewUpdateBuilder(a.r.config.LocalAS, isIBGP, ctx)
			params := a.buildLabeledUnicastParams(route)
			update := ub.BuildLabeledUnicast(params)

			if err := peer.SendUpdate(update); err != nil {
				lastErr = err
			}
		} else {
			// Session not established: queue to peer's operation queue
			// This maintains order with any pending teardowns
			peer.QueueAnnounce(ribRoute)
		}
	}
	return lastErr
}

// buildLabeledUnicastParams converts an API route to message.LabeledUnicastParams.
func (a *reactorAPIAdapter) buildLabeledUnicastParams(route api.LabeledUnicastRoute) message.LabeledUnicastParams {
	// Default label if not specified
	label := uint32(0)
	if len(route.Labels) > 0 {
		label = route.Labels[0]
	}

	params := message.LabeledUnicastParams{
		Prefix:  route.Prefix,
		PathID:  route.PathID, // RFC 7911 ADD-PATH
		NextHop: route.NextHop,
		Label:   label,
		Origin:  attribute.OriginIGP,
	}

	// Set optional attributes
	if route.Origin != nil {
		params.Origin = attribute.Origin(*route.Origin)
	}
	if route.LocalPreference != nil {
		params.LocalPreference = *route.LocalPreference
	}
	if route.MED != nil {
		params.MED = *route.MED
	}
	if len(route.ASPath) > 0 {
		params.ASPath = route.ASPath
	}
	if len(route.Communities) > 0 {
		params.Communities = route.Communities
	}
	if len(route.LargeCommunities) > 0 {
		lc := make([][3]uint32, len(route.LargeCommunities))
		for i, c := range route.LargeCommunities {
			lc[i] = [3]uint32{c.GlobalAdmin, c.LocalData1, c.LocalData2}
		}
		params.LargeCommunities = lc
	}
	if len(route.ExtendedCommunities) > 0 {
		params.ExtCommunityBytes = attribute.ExtendedCommunities(route.ExtendedCommunities).Pack()
	}

	return params
}

// buildLabeledUnicastRIBRoute creates a rib.Route from a LabeledUnicastRoute.
// Unlike AnnounceRoute which only stores OriginIGP, this stores ALL attributes.
// This ensures attributes are preserved when routes are queued and replayed.
//
// RFC 8277: Labeled unicast routes include MPLS labels in the NLRI.
// RFC 7911: PathID is included when ADD-PATH is negotiated.
func (a *reactorAPIAdapter) buildLabeledUnicastRIBRoute(route api.LabeledUnicastRoute, isIBGP bool) *rib.Route {
	// 1. Build NLRI with nlri.LabeledUnicast
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
	if route.Prefix.Addr().Is6() {
		family.AFI = nlri.AFIIPv6
	}

	// Default label if not specified
	labels := route.Labels
	if len(labels) == 0 {
		labels = []uint32{0}
	}

	n := nlri.NewLabeledUnicast(family, route.Prefix, labels, route.PathID)

	// 2. Build attributes - MUST INCLUDE ALL (unlike AnnounceRoute bug)
	var attrs []attribute.Attribute

	// Origin (required)
	if route.Origin != nil {
		attrs = append(attrs, attribute.Origin(*route.Origin))
	} else {
		attrs = append(attrs, attribute.OriginIGP)
	}

	// LocalPreference (optional, iBGP only per RFC 4271)
	if route.LocalPreference != nil {
		attrs = append(attrs, attribute.LocalPref(*route.LocalPreference))
	}

	// MED (optional)
	if route.MED != nil {
		attrs = append(attrs, attribute.MED(*route.MED))
	}

	// Communities (optional)
	if len(route.Communities) > 0 {
		comms := make(attribute.Communities, len(route.Communities))
		for i, c := range route.Communities {
			comms[i] = attribute.Community(c)
		}
		attrs = append(attrs, comms)
	}

	// LargeCommunities (optional)
	if len(route.LargeCommunities) > 0 {
		lcs := make(attribute.LargeCommunities, len(route.LargeCommunities))
		copy(lcs, route.LargeCommunities)
		attrs = append(attrs, lcs)
	}

	// ExtendedCommunities (optional)
	if len(route.ExtendedCommunities) > 0 {
		attrs = append(attrs, attribute.ExtendedCommunities(route.ExtendedCommunities))
	}

	// 3. Build AS-PATH
	// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS
	var asPath *attribute.ASPath
	switch {
	case len(route.ASPath) > 0:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: route.ASPath},
			},
		}
	case isIBGP:
		asPath = &attribute.ASPath{Segments: nil}
	default:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
			},
		}
	}

	return rib.NewRouteWithASPath(n, route.NextHop, attrs, asPath)
}

// WithdrawLabeledUnicast withdraws an MPLS labeled unicast route.
// RFC 8277 - Uses MP_UNREACH_NLRI with SAFI 4.
//
// Supports three modes like WithdrawRoute:
//   - Transaction mode: queues to Adj-RIB-Out (sent on commit)
//   - Established: sends immediately and removes from sent cache
//   - Not established: queues to peer's operation queue.
func (a *reactorAPIAdapter) WithdrawLabeledUnicast(peerSelector string, route api.LabeledUnicastRoute) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Build NLRI for queueing
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
	if route.Prefix.Addr().Is6() {
		family.AFI = nlri.AFIIPv6
	}

	// Default label for withdrawal
	labels := route.Labels
	if len(labels) == 0 {
		labels = []uint32{0x800000} // RFC 8277 withdrawal label
	}

	n := nlri.NewLabeledUnicast(family, route.Prefix, labels, route.PathID)

	var lastErr error
	for _, peer := range peers {
		if peer.State() == PeerStateEstablished {
			// Send immediately
			ctx := peer.packContext(family)

			// Build withdrawal using existing helper
			staticRoute := StaticRoute{
				Prefix: route.Prefix,
				Label:  labels[0],
			}

			update := buildMPUnreachLabeledUnicast(staticRoute, ctx)
			if err := peer.SendUpdate(update); err != nil {
				lastErr = err
			}
		} else {
			// Session not established: queue to peer's operation queue
			// This maintains order with any pending announces/teardowns
			peer.QueueWithdraw(n)
		}
	}
	return lastErr
}

// AnnounceMUPRoute announces a MUP route (SAFI 85) to matching peers.
// draft-mpmz-bess-mup-safi - Mobile User Plane.
func (a *reactorAPIAdapter) AnnounceMUPRoute(peerSelector string, spec api.MUPRouteSpec) error {
	return a.sendMUPRoute(peerSelector, spec, false)
}

// WithdrawMUPRoute withdraws a MUP route from matching peers.
// Uses MP_UNREACH_NLRI with SAFI 85.
func (a *reactorAPIAdapter) WithdrawMUPRoute(peerSelector string, spec api.MUPRouteSpec) error {
	return a.sendMUPRoute(peerSelector, spec, true)
}

// sendMUPRoute is a common helper for announce/withdraw MUP routes.
func (a *reactorAPIAdapter) sendMUPRoute(peerSelector string, spec api.MUPRouteSpec, isWithdraw bool) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Convert API spec to reactor MUPRoute
	mupRoute, err := convertAPIMUPRoute(spec)
	if err != nil {
		return fmt.Errorf("convert MUP route: %w", err)
	}

	var lastErr error
	for _, peer := range peers {
		if peer.State() != PeerStateEstablished {
			continue
		}

		// Check if MUP family is negotiated
		nc := peer.negotiated.Load()
		if nc == nil {
			continue
		}
		if spec.IsIPv6 && !nc.Has(nlri.IPv6MUP) {
			continue // Skip peer that doesn't support IPv6 MUP
		}
		if !spec.IsIPv6 && !nc.Has(nlri.IPv4MUP) {
			continue // Skip peer that doesn't support IPv4 MUP
		}

		family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: safiMUP}
		if spec.IsIPv6 {
			family.AFI = nlri.AFIIPv6
		}
		ctx := peer.packContext(family)

		// Build UPDATE using UpdateBuilder
		ub := message.NewUpdateBuilder(peer.settings.LocalAS, peer.settings.IsIBGP(), ctx)
		var update *message.Update
		if isWithdraw {
			update = ub.BuildMUPWithdraw(toMUPParams(mupRoute))
		} else {
			update = ub.BuildMUP(toMUPParams(mupRoute))
		}

		if err := peer.SendUpdate(update); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// TeardownPeer gracefully closes a peer session with NOTIFICATION.
// Sends Cease (6) with the specified subcode per RFC 4486.
func (a *reactorAPIAdapter) TeardownPeer(addr netip.Addr, subcode uint8) error {
	a.r.mu.RLock()
	peer, exists := a.r.peers[addr.String()]
	a.r.mu.RUnlock()

	if !exists {
		return ErrPeerNotFound
	}

	// Signal teardown with subcode - peer will send NOTIFICATION and close.
	// If session exists, teardown happens immediately.
	// If not connected, teardown is queued to maintain operation order.
	peer.Teardown(subcode)
	return nil
}

// AnnounceEOR sends an End-of-RIB marker for the given address family.
func (a *reactorAPIAdapter) AnnounceEOR(peerSelector string, afi uint16, safi uint8) error {
	update := message.BuildEOR(nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)})
	return a.sendToMatchingPeers(peerSelector, update)
}

// AnnounceWatchdog announces all routes in the named watchdog group.
// Routes are moved from withdrawn (-) to announced (+) state.
// Checks global pools first, then per-peer WatchdogGroups.
// Returns error only for send failures, not for missing groups.
func (a *reactorAPIAdapter) AnnounceWatchdog(peerSelector, name string) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return nil // No matching peers
	}

	// Check global pool first
	globalPool := a.r.watchdog.GetPool(name)
	if globalPool != nil {
		var lastErr error
		for _, peer := range peers {
			if peer.State() != PeerStateEstablished {
				continue
			}
			peerAddr := peer.Settings().Address.String()
			localAddr := peer.Settings().LocalAddress
			routes := a.r.watchdog.AnnouncePool(name, peerAddr)
			for _, pr := range routes {
				// RFC 7911: Get PackContext for ADD-PATH encoding
				ctx := peer.packContext(routeFamily(pr.StaticRoute))
				update := buildAnnounceUpdateFromStatic(pr.StaticRoute, a.r.config.LocalAS, peer.Settings().IsIBGP(), localAddr, ctx)
				if err := peer.SendUpdate(update); err != nil {
					lastErr = err
				}
			}
		}
		return lastErr
	}

	// Fall back to per-peer WatchdogGroups
	var lastErr error
	found := false
	for _, peer := range peers {
		err := peer.AnnounceWatchdog(name)
		if err != nil {
			if errors.Is(err, ErrWatchdogNotFound) {
				// This peer doesn't have the group - skip, try others
				continue
			}
			// Real error (send failure) - record but continue with other peers
			lastErr = err
		} else {
			found = true
		}
	}

	// If no peer had the group, return not found error
	if !found && lastErr == nil {
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}
	return lastErr
}

// WithdrawWatchdog withdraws all routes in the named watchdog group.
// Routes are moved from announced (+) to withdrawn (-) state.
// Checks global pools first, then per-peer WatchdogGroups.
// Returns error only for send failures, not for missing groups.
func (a *reactorAPIAdapter) WithdrawWatchdog(peerSelector, name string) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return nil // No matching peers
	}

	// Check global pool first
	globalPool := a.r.watchdog.GetPool(name)
	if globalPool != nil {
		var lastErr error
		for _, peer := range peers {
			if peer.State() != PeerStateEstablished {
				continue
			}
			peerAddr := peer.Settings().Address.String()
			routes := a.r.watchdog.WithdrawPool(name, peerAddr)
			for _, pr := range routes {
				// RFC 7911: Get PackContext for ADD-PATH encoding
				family := nlri.IPv4Unicast
				if pr.Prefix.Addr().Is6() {
					family = nlri.IPv6Unicast
				}
				ctx := peer.packContext(family)
				update := buildWithdrawUpdate(pr.Prefix, ctx)
				if err := peer.SendUpdate(update); err != nil {
					lastErr = err
				}
			}
		}
		return lastErr
	}

	// Fall back to per-peer WatchdogGroups
	var lastErr error
	found := false
	for _, peer := range peers {
		err := peer.WithdrawWatchdog(name)
		if err != nil {
			if errors.Is(err, ErrWatchdogNotFound) {
				// This peer doesn't have the group - skip, try others
				continue
			}
			// Real error (send failure) - record but continue with other peers
			lastErr = err
		} else {
			found = true
		}
	}

	// If no peer had the group, return not found error
	if !found && lastErr == nil {
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}
	return lastErr
}

// AddWatchdogRoute adds a route to a global watchdog pool.
// Implements api.ReactorInterface.
func (a *reactorAPIAdapter) AddWatchdogRoute(route api.RouteSpec, poolName string) error {
	// Convert api.RouteSpec to StaticRoute
	sr := StaticRoute{
		Prefix:      route.Prefix,
		NextHop:     route.NextHop,
		NextHopSelf: route.NextHopSelf,
	}
	if route.Origin != nil {
		sr.Origin = *route.Origin
	}
	if route.LocalPreference != nil {
		sr.LocalPreference = *route.LocalPreference
	}
	if route.MED != nil {
		sr.MED = *route.MED
	}
	if len(route.ASPath) > 0 {
		sr.ASPath = route.ASPath
	}
	if len(route.Communities) > 0 {
		sr.Communities = route.Communities
	}
	if len(route.LargeCommunities) > 0 {
		sr.LargeCommunities = make([][3]uint32, len(route.LargeCommunities))
		for i, lc := range route.LargeCommunities {
			sr.LargeCommunities[i] = [3]uint32{lc.GlobalAdmin, lc.LocalData1, lc.LocalData2}
		}
	}

	return a.r.AddWatchdogRoute(sr, poolName)
}

// RemoveWatchdogRoute removes a route from a global watchdog pool.
// Implements api.ReactorInterface.
func (a *reactorAPIAdapter) RemoveWatchdogRoute(routeKey, poolName string) error {
	return a.r.RemoveWatchdogRoute(routeKey, poolName)
}

// buildAnnounceUpdateFromStatic builds an UPDATE message from a StaticRoute.
// Uses the route's attributes, with defaults for missing values.
// localAddress is used to resolve "next-hop self" routes.
// RFC 7911: ctx provides ADD-PATH capability state for NLRI encoding.
func buildAnnounceUpdateFromStatic(route StaticRoute, localAS uint32, isIBGP bool, localAddress netip.Addr, ctx *nlri.PackContext) *message.Update {
	// Resolve next-hop: use local address if NextHopSelf, otherwise use configured NextHop
	nextHop := route.NextHop
	if route.NextHopSelf && localAddress.IsValid() {
		nextHop = localAddress
	}

	// Convert StaticRoute to RouteSpec for buildAnnounceUpdate
	spec := api.RouteSpec{
		Prefix:  route.Prefix,
		NextHop: nextHop,
	}

	// Copy optional attributes
	if route.Origin != 0 {
		origin := route.Origin
		spec.Origin = &origin
	}
	if route.LocalPreference != 0 {
		lp := route.LocalPreference
		spec.LocalPreference = &lp
	}
	if route.MED != 0 {
		med := route.MED
		spec.MED = &med
	}
	if len(route.ASPath) > 0 {
		spec.ASPath = route.ASPath
	}
	if len(route.Communities) > 0 {
		spec.Communities = route.Communities
	}
	if len(route.LargeCommunities) > 0 {
		spec.LargeCommunities = make([]attribute.LargeCommunity, len(route.LargeCommunities))
		for i, lc := range route.LargeCommunities {
			spec.LargeCommunities[i] = attribute.LargeCommunity{
				GlobalAdmin: lc[0],
				LocalData1:  lc[1],
				LocalData2:  lc[2],
			}
		}
	}

	// ctx provides ASN4 and ADD-PATH capability state
	return buildAnnounceUpdate(spec, localAS, isIBGP, ctx)
}

// RIBInRoutes returns routes from Adj-RIB-In.
func (a *reactorAPIAdapter) RIBInRoutes(peerID string) []api.RIBRoute {
	if a.r.ribIn == nil {
		return nil
	}

	var routes []api.RIBRoute

	// If peerID specified, get routes for that peer only
	if peerID != "" {
		for _, route := range a.r.ribIn.GetPeerRoutes(peerID) {
			routes = append(routes, routeToAPIRoute(peerID, route))
		}
		return routes
	}

	// Get routes from all peers
	a.r.mu.RLock()
	peerIDs := make([]string, 0, len(a.r.peers))
	for id := range a.r.peers {
		peerIDs = append(peerIDs, id)
	}
	a.r.mu.RUnlock()

	for _, id := range peerIDs {
		for _, route := range a.r.ribIn.GetPeerRoutes(id) {
			routes = append(routes, routeToAPIRoute(id, route))
		}
	}

	return routes
}

// RIBOutRoutes returns routes from Adj-RIB-Out.
//
// Deprecated: Adj-RIB-Out tracking removed. Returns nil.
func (a *reactorAPIAdapter) RIBOutRoutes() []api.RIBRoute {
	return nil
}

// RIBStats returns RIB statistics.
func (a *reactorAPIAdapter) RIBStats() api.RIBStatsInfo {
	stats := api.RIBStatsInfo{}

	if a.r.ribIn != nil {
		inStats := a.r.ribIn.Stats()
		stats.InPeerCount = inStats.PeerCount
		stats.InRouteCount = inStats.RouteCount
	}

	// Note: Adj-RIB-Out tracking removed. OutPending/OutWithdrawls/OutSent always 0.

	return stats
}

// ClearRIBIn clears all routes from Adj-RIB-In.
func (a *reactorAPIAdapter) ClearRIBIn() int {
	if a.r.ribIn == nil {
		return 0
	}
	return a.r.ribIn.ClearAll()
}

// ClearRIBOut queues withdrawals for all routes in Adj-RIB-Out.
//
// Deprecated: Adj-RIB-Out tracking removed. Returns 0.
func (a *reactorAPIAdapter) ClearRIBOut() int {
	return 0
}

// FlushRIBOut re-queues all sent routes for re-announcement.
//
// Deprecated: Adj-RIB-Out tracking removed. Returns 0.
func (a *reactorAPIAdapter) FlushRIBOut() int {
	return 0
}

// GetPeerAPIBindings returns API bindings for a specific peer.
// Returns nil if peer not found.
// Resolves encoding inheritance: peer binding → process encoder → "text" default.
func (a *reactorAPIAdapter) GetPeerAPIBindings(peerAddr netip.Addr) []api.PeerAPIBinding {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	peer, ok := a.r.peers[peerAddr.String()]
	if !ok {
		return nil
	}

	settings := peer.Settings()
	result := make([]api.PeerAPIBinding, 0, len(settings.APIBindings))
	for _, b := range settings.APIBindings {
		// Resolve encoding: peer override → process default → "text"
		encoding := b.Encoding
		if encoding == "" {
			encoding = a.getProcessEncoder(b.ProcessName)
		}
		if encoding == "" {
			encoding = "text"
		}

		// Resolve format: peer override → "parsed"
		format := b.Format
		if format == "" {
			format = "parsed"
		}

		result = append(result, api.PeerAPIBinding{
			ProcessName:         b.ProcessName,
			Encoding:            encoding,
			Format:              format,
			ReceiveUpdate:       b.ReceiveUpdate,
			ReceiveOpen:         b.ReceiveOpen,
			ReceiveNotification: b.ReceiveNotification,
			ReceiveKeepalive:    b.ReceiveKeepalive,
			ReceiveRefresh:      b.ReceiveRefresh,
			ReceiveState:        b.ReceiveState,
			ReceiveSent:         b.ReceiveSent,
			SendUpdate:          b.SendUpdate,
			SendRefresh:         b.SendRefresh,
		})
	}
	return result
}

// getProcessEncoder returns the encoder for a process, or empty if not found.
func (a *reactorAPIAdapter) getProcessEncoder(name string) string {
	for _, pc := range a.r.config.APIProcesses {
		if pc.Name == name {
			return pc.Encoder
		}
	}
	return ""
}

// Transaction support for commit-based batching.
// Note: Per-peer Adj-RIB-Out transactions removed. Use CommitManager instead.

// BeginTransaction starts a new transaction for batched route updates.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Use CommitManager via "commit <name> start".
func (a *reactorAPIAdapter) BeginTransaction(peerSelector, label string) error {
	return errors.New("per-peer transactions removed; use 'commit <name> start' instead")
}

// CommitTransaction commits the current transaction.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Use CommitManager via "commit <name> end".
func (a *reactorAPIAdapter) CommitTransaction(peerSelector string) (api.TransactionResult, error) {
	return api.TransactionResult{}, errors.New("per-peer transactions removed; use 'commit <name> end' instead")
}

// CommitTransactionWithLabel commits, verifying the label matches.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Use CommitManager via "commit <name> end".
func (a *reactorAPIAdapter) CommitTransactionWithLabel(peerSelector, label string) (api.TransactionResult, error) {
	return api.TransactionResult{}, errors.New("per-peer transactions removed; use 'commit <name> end' instead")
}

// RollbackTransaction discards all queued routes in the transaction.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Use CommitManager via "commit <name> rollback".
func (a *reactorAPIAdapter) RollbackTransaction(peerSelector string) (api.TransactionResult, error) {
	return api.TransactionResult{}, errors.New("per-peer transactions removed; use 'commit <name> rollback' instead")
}

// getMatchingPeers returns peers matching the selector.
// Supports: "*" (all peers), exact IP, or glob patterns (e.g., "192.168.*.*").
func (a *reactorAPIAdapter) getMatchingPeers(selector string) []*Peer {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	// Fast path: all peers
	if selector == "*" || selector == "" {
		peers := make([]*Peer, 0, len(a.r.peers))
		for _, peer := range a.r.peers {
			peers = append(peers, peer)
		}
		return peers
	}

	// Fast path: exact match (no wildcards)
	if peer, ok := a.r.peers[selector]; ok {
		return []*Peer{peer}
	}

	// Glob pattern match
	var peers []*Peer
	for addr, peer := range a.r.peers {
		if ipGlobMatch(selector, addr) {
			peers = append(peers, peer)
		}
	}
	return peers
}

// ipGlobMatch checks if an IP address matches a glob pattern.
// Pattern "*" matches any IP (IPv4 or IPv6).
// For IPv4, each octet can be "*" to match any value 0-255.
// Examples: "192.168.*.*", "10.*.0.1", "*.*.*.1".
func ipGlobMatch(pattern, ip string) bool {
	// "*" or empty matches everything
	if pattern == "*" || pattern == "" {
		return true
	}

	// Check if pattern looks like IPv4 glob (contains dots)
	if strings.Contains(pattern, ".") && strings.Contains(ip, ".") {
		patternParts := strings.Split(pattern, ".")
		ipParts := strings.Split(ip, ".")

		if len(patternParts) != 4 || len(ipParts) != 4 {
			return false
		}

		for i := 0; i < 4; i++ {
			if patternParts[i] == "*" {
				continue // wildcard matches any octet
			}
			if patternParts[i] != ipParts[i] {
				return false
			}
		}
		return true
	}

	// For IPv6 or exact match, just compare strings
	return pattern == ip
}

// InTransaction returns true if any matching peer has an active transaction.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Always returns false.
func (a *reactorAPIAdapter) InTransaction(peerSelector string) bool {
	return false
}

// TransactionID returns the transaction label for the first matching peer.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Always returns empty string.
func (a *reactorAPIAdapter) TransactionID(peerSelector string) string {
	return ""
}

// SendRoutes sends routes directly to matching peers using CommitService.
// This bypasses OutgoingRIB transaction and is used for named commits.
func (a *reactorAPIAdapter) SendRoutes(peerSelector string, routes []*rib.Route, withdrawals []nlri.NLRI, sendEOR bool) (api.TransactionResult, error) {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return api.TransactionResult{}, errors.New("no peers match selector")
	}

	var totalResult api.TransactionResult

	// Collect families for EOR (from both routes and withdrawals)
	seen := make(map[nlri.Family]bool)
	for _, r := range routes {
		seen[r.NLRI().Family()] = true
	}
	for _, n := range withdrawals {
		seen[n.Family()] = true
	}
	families := make([]nlri.Family, 0, len(seen))
	for f := range seen {
		families = append(families, f)
	}

	// Track stats once (not per-peer)
	totalResult.RoutesAnnounced = len(routes)
	totalResult.RoutesWithdrawn = len(withdrawals)

	for _, peer := range peers {
		// Get negotiated parameters for CommitService
		neg := peer.messageNegotiated()
		if neg == nil {
			continue // Peer not established
		}

		// Use CommitService with two-level grouping for announcements
		cs := rib.NewCommitService(peer, neg, true)

		// Send announcements
		if len(routes) > 0 {
			stats, err := cs.Commit(routes, rib.CommitOptions{SendEOR: false})
			if err != nil {
				continue
			}
			totalResult.UpdatesSent += stats.UpdatesSent
		}

		// Send withdrawals
		if len(withdrawals) > 0 {
			updatesSent := a.sendWithdrawals(peer, withdrawals)
			totalResult.UpdatesSent += updatesSent
		}

		// Send EOR for each family if requested
		if sendEOR {
			for _, f := range families {
				eor := message.BuildEOR(f)
				if err := peer.SendUpdate(eor); err == nil {
					totalResult.UpdatesSent++
				}
			}
		}
	}

	// Build family strings for result
	familyStrs := make([]string, len(families))
	for i, f := range families {
		familyStrs[i] = f.String()
	}
	totalResult.Families = familyStrs

	return totalResult, nil
}

// sendWithdrawals sends withdrawal UPDATE messages for the given NLRIs.
// Groups by family for efficient packing.
// RFC 7911: Uses Pack(ctx) for ADD-PATH aware encoding.
func (a *reactorAPIAdapter) sendWithdrawals(peer *Peer, withdrawals []nlri.NLRI) int {
	if len(withdrawals) == 0 {
		return 0
	}

	// Group withdrawals by family
	byFamily := make(map[nlri.Family][]nlri.NLRI)
	for _, n := range withdrawals {
		f := n.Family()
		byFamily[f] = append(byFamily[f], n)
	}

	updatesSent := 0
	ipv4Unicast := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	for family, nlris := range byFamily {
		// RFC 7911: Get PackContext for ADD-PATH encoding
		ctx := peer.packContext(family)
		var update *message.Update

		if family == ipv4Unicast {
			// IPv4 unicast: use WithdrawnRoutes field
			var withdrawnBytes []byte
			for _, n := range nlris {
				withdrawnBytes = append(withdrawnBytes, n.Pack(ctx)...)
			}
			update = &message.Update{
				WithdrawnRoutes: withdrawnBytes,
			}
		} else {
			// Other families: use MP_UNREACH_NLRI attribute
			var nlriBytes []byte
			for _, n := range nlris {
				nlriBytes = append(nlriBytes, n.Pack(ctx)...)
			}
			mpUnreach := &attribute.MPUnreachNLRI{
				AFI:  attribute.AFI(family.AFI),
				SAFI: attribute.SAFI(family.SAFI),
				NLRI: nlriBytes,
			}
			update = &message.Update{
				PathAttributes: attribute.PackAttribute(mpUnreach),
			}
		}

		if err := peer.SendUpdate(update); err == nil {
			updatesSent++
		}
	}

	return updatesSent
}

// ForwardUpdate forwards a cached UPDATE to peers matching the selector.
// Looks up the update by ID from the cache and sends to matching peers.
// One-shot: deletes from cache after forwarding completes.
//
// Zero-copy optimization: When source and destination encoding contexts match
// (same ASN4, ADD-PATH capabilities), the raw UPDATE bytes are forwarded
// directly without re-encoding.
//
// RFC 8654 compliance: If the UPDATE exceeds a peer's max message size
// (4096 without Extended Message, 65535 with), it is split into multiple
// smaller UPDATEs that each fit within the limit.
func (a *reactorAPIAdapter) ForwardUpdate(sel *api.Selector, updateID uint64) error {
	// Look up update by ID from cache
	update, ok := a.r.recentUpdates.Get(updateID)
	if !ok {
		return ErrUpdateExpired
	}

	// Get matching peers
	a.r.mu.RLock()
	var matchingPeers []*Peer
	for _, peer := range a.r.peers {
		addr := peer.Settings().Address
		if sel.Matches(addr) && addr != update.SourcePeerIP {
			// Don't forward back to source peer (implicit loop prevention)
			matchingPeers = append(matchingPeers, peer)
		}
	}
	a.r.mu.RUnlock()

	if len(matchingPeers) == 0 {
		return fmt.Errorf("no peers match selector %s", sel)
	}

	// Convert to routes for adj-rib-out persistence and splitting (parse once, reuse)
	// Track error separately: non-fatal for adj-rib-out, fatal for split path
	routes, routesErr := update.ConvertToRoutes()
	if routesErr != nil {
		// Warn about persistence failure - forwarding will work but routes won't survive reconnect
		trace.Log(trace.Routes, "forward update %d: ConvertToRoutes failed: %v (routes will not persist)", updateID, routesErr)
	}

	// Calculate update size once (header + body)
	updateSize := message.HeaderLen + len(update.RawBytes)

	// Forward to all matching peers, collect errors
	// Lazy parsing: only parse if we need to re-encode
	var parsedUpdate *message.Update
	var errs []error
	var sentCount int

	for _, peer := range matchingPeers {
		if peer.State() != PeerStateEstablished {
			continue // Skip non-established peers
		}

		sentCount++

		// Get max message size for this peer (RFC 8654)
		nc := peer.negotiated.Load()
		extendedMessage := nc != nil && nc.ExtendedMessage
		maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, extendedMessage))

		// Check if UPDATE exceeds peer's max message size
		if updateSize > maxMsgSize {
			// Split path: UPDATE too large for this peer
			// Requires routes from ConvertToRoutes - fail if conversion failed
			if routesErr != nil {
				errs = append(errs, fmt.Errorf("peer %s: cannot split UPDATE: %w",
					peer.Settings().Address, routesErr))
				continue
			}
			if err := a.sendSplitUpdate(peer, update, routes, maxMsgSize); err != nil {
				errs = append(errs, fmt.Errorf("peer %s: %w", peer.Settings().Address, err))
			}
		} else {
			// Normal path: UPDATE fits based on original size
			destCtxID := peer.SendContextID()

			// Zero-copy path: use raw bytes when contexts match
			// Both must be non-zero (registered) and equal
			if update.SourceCtxID != 0 && destCtxID != 0 && update.SourceCtxID == destCtxID {
				if err := peer.SendRawUpdateBody(update.RawBytes); err != nil {
					errs = append(errs, fmt.Errorf("peer %s: %w", peer.Settings().Address, err))
				}
			} else {
				// Re-encode path: parse (lazily) and send
				if parsedUpdate == nil {
					var parseErr error
					parsedUpdate, parseErr = message.UnpackUpdate(update.RawBytes)
					if parseErr != nil {
						return fmt.Errorf("parsing cached update: %w", parseErr)
					}
				}

				// Check repacked size - may differ from original due to ASN4 encoding changes
				// Size = Header(19) + WithdrawnLen(2) + Withdrawn + AttrLen(2) + Attrs + NLRI
				repackedSize := message.HeaderLen + 4 + len(parsedUpdate.WithdrawnRoutes) +
					len(parsedUpdate.PathAttributes) + len(parsedUpdate.NLRI)
				if repackedSize > maxMsgSize {
					// Re-encoded UPDATE is too large, fall back to split path
					if routesErr != nil {
						errs = append(errs, fmt.Errorf("peer %s: re-encoded UPDATE size %d exceeds max %d and cannot split: %w",
							peer.Settings().Address, repackedSize, maxMsgSize, routesErr))
					} else if err := a.sendSplitUpdate(peer, update, routes, maxMsgSize); err != nil {
						errs = append(errs, fmt.Errorf("peer %s: %w", peer.Settings().Address, err))
					}
				} else {
					if err := peer.SendUpdate(parsedUpdate); err != nil {
						errs = append(errs, fmt.Errorf("peer %s: %w", peer.Settings().Address, err))
					}
				}
			}
		}
	}

	// One-shot: delete from cache after forwarding (even on partial failure)
	a.r.recentUpdates.Delete(updateID)

	if sentCount == 0 {
		return fmt.Errorf("no established peers to forward to")
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// sendSplitUpdate sends an UPDATE split into multiple messages to fit maxMsgSize.
// Used when forwarding from Extended Message peer to non-Extended peer.
//
// RFC 8654 compliance: Messages are split so each fits within the peer's
// maximum message size (4096 without Extended Message capability).
func (a *reactorAPIAdapter) sendSplitUpdate(peer *Peer, update *ReceivedUpdate, routes []*rib.Route, maxMsgSize int) error {
	var errs []error

	// Send announcements (routes) in batches
	if len(routes) > 0 {
		if err := a.sendRoutesWithLimit(peer, routes, maxMsgSize); err != nil {
			errs = append(errs, fmt.Errorf("sending routes: %w", err))
		}
	}

	// Send withdrawals in batches
	if len(update.Withdraws) > 0 {
		if err := a.sendWithdrawalsWithLimit(peer, update.Withdraws, maxMsgSize); err != nil {
			errs = append(errs, fmt.Errorf("sending withdrawals: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// sendRoutesWithLimit sends routes in batches that fit within maxMsgSize.
//
// When GroupUpdates is enabled (default), routes with identical attributes are
// grouped into single UPDATE messages. This reduces UPDATE count from O(routes)
// to O(routes/capacity), dramatically improving efficiency for large route sets.
//
// When GroupUpdates is disabled, routes are sent individually (legacy behavior).
func (a *reactorAPIAdapter) sendRoutesWithLimit(peer *Peer, routes []*rib.Route, maxMsgSize int) error {
	if len(routes) == 0 {
		return nil
	}

	// Fall back to individual sending if grouping disabled
	if !peer.settings.GroupUpdates {
		return a.sendRoutesIndividually(peer, routes, maxMsgSize)
	}

	// Group routes by attributes + AS_PATH
	attrGroups := rib.GroupByAttributesTwoLevel(routes)

	var errs []error
	for _, attrGroup := range attrGroups {
		for _, aspGroup := range attrGroup.ByASPath {
			if err := a.sendASPathGroup(peer, &attrGroup, &aspGroup, maxMsgSize); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// sendRoutesIndividually sends routes one at a time (legacy behavior).
func (a *reactorAPIAdapter) sendRoutesIndividually(peer *Peer, routes []*rib.Route, maxMsgSize int) error {
	var errs []error

	for _, route := range routes {
		family := route.NLRI().Family()
		ctx := peer.packContext(family)
		update := buildRIBRouteUpdate(route, peer.settings.LocalAS, peer.settings.IsIBGP(), ctx)

		if err := peer.sendUpdateWithSplit(update, maxMsgSize, family); err != nil {
			errs = append(errs, fmt.Errorf("route %s: %w", route.NLRI(), err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// sendASPathGroup sends routes in an AS_PATH group as efficiently as possible.
// For IPv4 unicast: uses BuildGroupedUnicastWithLimit to pack multiple NLRIs.
// For MP families: builds UPDATE with MP_REACH_NLRI containing grouped NLRIs.
func (a *reactorAPIAdapter) sendASPathGroup(peer *Peer, attrGroup *rib.AttributeGroup, aspGroup *rib.ASPathGroup, maxMsgSize int) error {
	if len(aspGroup.Routes) == 0 {
		return nil
	}

	family := attrGroup.Family
	ctx := peer.packContext(family)

	// IPv4 unicast: use BuildGroupedUnicastWithLimit
	if family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast {
		return a.sendGroupedIPv4Unicast(peer, aspGroup.Routes, ctx, maxMsgSize)
	}

	// MP families: build UPDATE with MP_REACH_NLRI containing grouped NLRIs
	return a.sendGroupedMPFamily(peer, aspGroup.Routes, family, ctx, maxMsgSize)
}

// sendGroupedIPv4Unicast sends grouped IPv4 unicast routes using BuildGroupedUnicastWithLimit.
func (a *reactorAPIAdapter) sendGroupedIPv4Unicast(peer *Peer, routes []*rib.Route, ctx *nlri.PackContext, maxMsgSize int) error {
	// Check if any route has complex AS_PATH (AS_SET, CONFED, multiple segments)
	// that can't be represented in UnicastParams.ASPath (which is just []uint32).
	// Fall back to individual sending for such routes.
	for _, route := range routes {
		if hasComplexASPath(route) {
			return a.sendRoutesIndividually(peer, routes, maxMsgSize)
		}
	}

	// Convert to UnicastParams
	params := make([]message.UnicastParams, len(routes))
	for i, route := range routes {
		params[i] = toRIBRouteUnicastParams(route)
	}

	// Build grouped UPDATEs respecting size limits
	ub := message.NewUpdateBuilder(peer.settings.LocalAS, peer.settings.IsIBGP(), ctx)
	updates, err := ub.BuildGroupedUnicastWithLimit(params, maxMsgSize)
	if err != nil {
		return fmt.Errorf("building grouped IPv4 unicast: %w", err)
	}

	// Send all UPDATEs
	var errs []error
	for _, update := range updates {
		if err := peer.SendUpdate(update); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// hasComplexASPath returns true if the route's AS_PATH can't be represented
// as a simple []uint32 (has AS_SET, CONFED segments, or multiple segments).
func hasComplexASPath(route *rib.Route) bool {
	asPath := route.ASPath()
	if asPath == nil || len(asPath.Segments) == 0 {
		return false
	}

	// Multiple segments = complex
	if len(asPath.Segments) > 1 {
		return true
	}

	// Single segment: only AS_SEQUENCE is simple
	seg := asPath.Segments[0]
	return seg.Type != attribute.ASSequence
}

// sendGroupedMPFamily sends grouped MP family routes (IPv6, VPN, etc.).
// Packs multiple NLRIs into MP_REACH_NLRI attribute.
func (a *reactorAPIAdapter) sendGroupedMPFamily(peer *Peer, routes []*rib.Route, family nlri.Family, ctx *nlri.PackContext, maxMsgSize int) error {
	if len(routes) == 0 {
		return nil
	}

	// Pack all NLRIs
	var nlriBytes []byte
	for _, route := range routes {
		nlriBytes = append(nlriBytes, route.NLRI().Pack(ctx)...)
	}

	// Build grouped UPDATE with all NLRIs
	firstRoute := routes[0]
	groupedUpdate := a.buildGroupedMPUpdate(firstRoute, nlriBytes, family, peer.settings.LocalAS, peer.settings.IsIBGP(), ctx)

	// Check actual size of grouped update
	msgSize := message.HeaderLen + 4 + len(groupedUpdate.PathAttributes)
	if msgSize <= maxMsgSize {
		return peer.SendUpdate(groupedUpdate)
	}

	// Need to split - calculate available space for NLRI
	// MP_REACH_NLRI overhead: header(3-4) + AFI(2) + SAFI(1) + NH-len(1) + NH + Reserved(1)
	// Next-hop sizes: IPv4=4, IPv6=16 or 32 (global+link-local), VPN=12 or 24
	nhLen := nextHopLength(family, firstRoute.NextHop())
	mpReachOverhead := 4 + 2 + 1 + 1 + nhLen + 1 // extended header + AFI + SAFI + NH-len + NH + reserved

	// Base attributes (without MP_REACH_NLRI's NLRI portion)
	baseAttrSize := len(groupedUpdate.PathAttributes) - len(nlriBytes)
	availableNLRISpace := maxMsgSize - message.HeaderLen - 4 - baseAttrSize - mpReachOverhead

	if availableNLRISpace <= 0 {
		return fmt.Errorf("attributes too large for MP family: %d bytes, max %d", baseAttrSize+mpReachOverhead, maxMsgSize-message.HeaderLen-4)
	}

	// Split NLRIs into chunks
	sendCtx := peer.SendContext()
	addPath := sendCtx != nil && sendCtx.AddPathFor(family)
	chunks, err := message.ChunkMPNLRI(nlriBytes, uint16(family.AFI), uint8(family.SAFI), addPath, availableNLRISpace)
	if err != nil {
		return fmt.Errorf("chunking MP NLRI: %w", err)
	}

	var errs []error
	for _, chunk := range chunks {
		chunkUpdate := a.buildGroupedMPUpdate(firstRoute, chunk, family, peer.settings.LocalAS, peer.settings.IsIBGP(), ctx)
		if err := peer.SendUpdate(chunkUpdate); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// nextHopLength returns the wire length of next-hop for a given family.
func nextHopLength(family nlri.Family, nh netip.Addr) int {
	switch {
	case family.AFI == nlri.AFIIPv4:
		return 4
	case family.AFI == nlri.AFIIPv6:
		// Could be 16 (global only) or 32 (global + link-local)
		// Conservative: assume 32 for safety
		return 32
	case family.SAFI == nlri.SAFIVPN:
		// VPN: RD (8) + address (4 or 16)
		if family.AFI == nlri.AFIIPv4 {
			return 12 // RD + IPv4
		}
		return 24 // RD + IPv6
	default:
		// Conservative default
		if nh.Is6() {
			return 32
		}
		return 4
	}
}

// buildGroupedMPUpdate builds an UPDATE with MP_REACH_NLRI containing multiple NLRIs.
func (a *reactorAPIAdapter) buildGroupedMPUpdate(templateRoute *rib.Route, nlriBytes []byte, family nlri.Family, localAS uint32, isIBGP bool, ctx *nlri.PackContext) *message.Update {
	var attrBytes []byte

	// Extract ASN4 from context
	asn4 := ctx == nil || ctx.ASN4

	// 1. ORIGIN
	foundOrigin := false
	for _, attr := range templateRoute.Attributes() {
		if _, ok := attr.(attribute.Origin); ok {
			attrBytes = append(attrBytes, attribute.PackAttribute(attr)...)
			foundOrigin = true
			break
		}
	}
	if !foundOrigin {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.OriginIGP)...)
	}

	// 2. AS_PATH
	storedASPath := templateRoute.ASPath()
	hasStoredASPath := storedASPath != nil && len(storedASPath.Segments) > 0

	switch {
	case hasStoredASPath:
		attrBytes = append(attrBytes, attribute.PackASPathAttribute(storedASPath, asn4)...)
	case isIBGP || localAS == 0:
		emptyASPath := &attribute.ASPath{Segments: nil}
		attrBytes = append(attrBytes, attribute.PackASPathAttribute(emptyASPath, asn4)...)
	default:
		ebgpASPath := &attribute.ASPath{
			Segments: []attribute.ASPathSegment{{
				Type: attribute.ASSequence,
				ASNs: []uint32{localAS},
			}},
		}
		attrBytes = append(attrBytes, attribute.PackASPathAttribute(ebgpASPath, asn4)...)
	}

	// MP_REACH_NLRI with grouped NLRIs
	mpReach := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(family.AFI),
		SAFI:     attribute.SAFI(family.SAFI),
		NextHops: []netip.Addr{templateRoute.NextHop()},
		NLRI:     nlriBytes,
	}
	attrBytes = append(attrBytes, attribute.PackAttribute(mpReach)...)

	// LOCAL_PREF for iBGP
	if isIBGP {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
	}

	// Copy optional attributes
	for _, attr := range templateRoute.Attributes() {
		switch attr.(type) {
		case attribute.Origin, *attribute.ASPath, *attribute.NextHop, attribute.LocalPref:
			continue
		case attribute.MED, attribute.Communities,
			attribute.ExtendedCommunities, attribute.LargeCommunities,
			attribute.IPv6ExtendedCommunities,
			attribute.AtomicAggregate, *attribute.Aggregator,
			attribute.OriginatorID, attribute.ClusterList:
			attrBytes = append(attrBytes, attribute.PackAttribute(attr)...)
		}
	}

	return &message.Update{
		PathAttributes: attrBytes,
	}
}

// toRIBRouteUnicastParams converts a RIB route to UnicastParams for grouped building.
// Extracts attributes from the route's attribute slice for use with BuildGroupedUnicastWithLimit.
func toRIBRouteUnicastParams(route *rib.Route) message.UnicastParams {
	params := message.UnicastParams{
		NextHop: route.NextHop(),
		Origin:  attribute.OriginIGP, // Default
	}

	// Extract prefix and path-id from NLRI
	if n := route.NLRI(); n != nil {
		if inet, ok := n.(*nlri.INET); ok {
			params.Prefix = inet.Prefix()
			params.PathID = inet.PathID()
		}
	}

	// Extract AS_PATH if present
	if asPath := route.ASPath(); asPath != nil {
		for _, seg := range asPath.Segments {
			if seg.Type == attribute.ASSequence {
				params.ASPath = append(params.ASPath, seg.ASNs...)
			}
		}
	}

	// Extract attributes from the route's attribute slice
	for _, attr := range route.Attributes() {
		switch a := attr.(type) {
		case attribute.Origin:
			params.Origin = a
		case attribute.MED:
			params.MED = uint32(a)
		case attribute.LocalPref:
			params.LocalPreference = uint32(a)
		case attribute.Communities:
			params.Communities = make([]uint32, len(a))
			for i, c := range a {
				params.Communities[i] = uint32(c)
			}
		case attribute.ExtendedCommunities:
			params.ExtCommunityBytes = a.Pack()
		case attribute.LargeCommunities:
			params.LargeCommunities = make([][3]uint32, len(a))
			for i, lc := range a {
				params.LargeCommunities[i] = [3]uint32{lc.GlobalAdmin, lc.LocalData1, lc.LocalData2}
			}
		case attribute.AtomicAggregate:
			params.AtomicAggregate = true
		case *attribute.Aggregator:
			params.HasAggregator = true
			params.AggregatorASN = a.ASN
			if a.Address.Is4() {
				params.AggregatorIP = a.Address.As4()
			}
		case attribute.OriginatorID:
			if addr := netip.Addr(a); addr.Is4() {
				ip4 := addr.As4()
				params.OriginatorID = uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
			}
		case attribute.ClusterList:
			params.ClusterList = make([]uint32, len(a))
			copy(params.ClusterList, a)
		}
	}

	return params
}

// sendWithdrawalsWithLimit sends withdrawals using SplitUpdate for size limiting.
// Groups withdrawals by family to ensure correct Add-Path detection for each.
// Uses the same splitting infrastructure as announcements for consistency.
func (a *reactorAPIAdapter) sendWithdrawalsWithLimit(peer *Peer, withdraws []nlri.NLRI, maxMsgSize int) error {
	if len(withdraws) == 0 {
		return nil
	}

	// Group withdrawals by family for correct Add-Path detection
	// BGP spec requires same-family NLRIs in each UPDATE, and Add-Path is per-family
	byFamily := make(map[nlri.Family][]byte)
	for _, n := range withdraws {
		family := n.Family()
		byFamily[family] = append(byFamily[family], n.Bytes()...)
	}

	var errs []error
	for family, withdrawnBytes := range byFamily {
		// Build withdrawal-only UPDATE for this family
		update := &message.Update{
			WithdrawnRoutes: withdrawnBytes,
		}

		// Use sendUpdateWithSplit for consistent splitting and Add-Path handling
		if err := peer.sendUpdateWithSplit(update, maxMsgSize, family); err != nil {
			errs = append(errs, fmt.Errorf("sending %s withdrawals: %w", family, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// DeleteUpdate removes an update from the cache without forwarding.
// Used when controller decides not to forward (filtering).
func (a *reactorAPIAdapter) DeleteUpdate(updateID uint64) error {
	if !a.r.recentUpdates.Delete(updateID) {
		return ErrUpdateExpired
	}
	return nil
}

// SignalAPIReady signals that an API process is ready.
func (a *reactorAPIAdapter) SignalAPIReady() {
	a.r.SignalAPIReady()
}

// SignalPeerAPIReady signals that a peer-specific API initialization is complete.
func (a *reactorAPIAdapter) SignalPeerAPIReady(peerAddr string) {
	a.r.SignalPeerAPIReady(peerAddr)
}

// SendRawMessage sends raw bytes to a peer.
// If msgType is 0, payload is a full BGP packet (user provides marker+header).
// If msgType is non-zero, payload is message body (we add the header).
func (a *reactorAPIAdapter) SendRawMessage(peerAddr netip.Addr, msgType uint8, payload []byte) error {
	a.r.mu.RLock()
	peer, exists := a.r.peers[peerAddr.String()]
	a.r.mu.RUnlock()

	if !exists {
		return ErrPeerNotFound
	}

	return peer.SendRawMessage(msgType, payload)
}

// routeToAPIRoute converts a RIB route to an API route.
func routeToAPIRoute(peerID string, route *rib.Route) api.RIBRoute {
	apiRoute := api.RIBRoute{
		Peer: peerID,
	}

	if route.NLRI() != nil {
		apiRoute.Prefix = route.NLRI().String()
	}

	if route.NextHop().IsValid() {
		apiRoute.NextHop = route.NextHop().String()
	}

	if asPath := route.ASPath(); asPath != nil {
		apiRoute.ASPath = formatASPath(asPath)
	}

	return apiRoute
}

// formatASPath formats an AS path for display.
func formatASPath(asPath *attribute.ASPath) string {
	if asPath == nil || len(asPath.Segments) == 0 {
		return ""
	}

	var result string
	for i, seg := range asPath.Segments {
		if i > 0 {
			result += " "
		}
		switch seg.Type {
		case attribute.ASSet:
			result += "{"
			for j, asn := range seg.ASNs {
				if j > 0 {
					result += ","
				}
				result += fmt.Sprintf("%d", asn)
			}
			result += "}"
		case attribute.ASSequence:
			for j, asn := range seg.ASNs {
				if j > 0 {
					result += " "
				}
				result += fmt.Sprintf("%d", asn)
			}
		case attribute.ASConfedSet, attribute.ASConfedSequence:
			// Confederation segments - format similar to regular segments
			for j, asn := range seg.ASNs {
				if j > 0 {
					result += " "
				}
				result += fmt.Sprintf("%d", asn)
			}
		}
	}
	return result
}

// New creates a new reactor with the given configuration.
func New(config *Config) *Reactor {
	// Apply defaults for recent update cache
	ttl := config.RecentUpdateTTL
	if ttl == 0 {
		ttl = 60 * time.Second // Default: 60 seconds
	}
	maxEntries := config.RecentUpdateMax
	if maxEntries == 0 {
		maxEntries = 100000 // Default: 100k entries
	}

	return &Reactor{
		config:        config,
		peers:         make(map[string]*Peer),
		listeners:     make(map[netip.Addr]*Listener),
		ribIn:         rib.NewIncomingRIB(),
		ribStore:      rib.NewRouteStore(100), // Buffer size for dedup workers
		watchdog:      NewWatchdogManager(),
		recentUpdates: NewRecentUpdateCache(ttl, maxEntries),
	}
}

// WatchdogManager returns the global watchdog pool manager.
func (r *Reactor) WatchdogManager() *WatchdogManager {
	return r.watchdog
}

// AddWatchdogRoute adds a route to a global watchdog pool.
// Creates the pool if it doesn't exist.
// The route will be announced to all peers when "announce watchdog <name>" is called.
// Returns ErrRouteExists if a route with the same key already exists in the pool.
func (r *Reactor) AddWatchdogRoute(route StaticRoute, poolName string) error {
	_, err := r.watchdog.AddRoute(poolName, route)
	return err
}

// RemoveWatchdogRoute removes a route from a global watchdog pool.
// Returns ErrWatchdogNotFound if pool doesn't exist.
// Returns ErrWatchdogRouteNotFound if route doesn't exist in pool.
// Sends withdrawals to all peers that had the route announced.
func (r *Reactor) RemoveWatchdogRoute(routeKey, poolName string) error {
	// Check pool exists first (for better error message)
	if r.watchdog.GetPool(poolName) == nil {
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, poolName)
	}

	// Atomically remove route and get its data for withdrawals
	removedRoute, ok := r.watchdog.RemoveAndGetRoute(poolName, routeKey)
	if !ok {
		return fmt.Errorf("%w: %s", ErrWatchdogRouteNotFound, routeKey)
	}

	// Send withdrawals to all peers that had this route announced
	// Route is already removed from pool, so no race condition
	r.mu.RLock()
	for _, peer := range r.peers {
		if peer.State() != PeerStateEstablished {
			continue
		}
		peerAddr := peer.Settings().Address.String()
		// Note: removedRoute.announced is no longer protected by pool mutex,
		// but it's safe because the route is now orphaned (no concurrent access)
		if removedRoute.announced[peerAddr] {
			// RFC 7911: Get PackContext for ADD-PATH encoding
			family := nlri.IPv4Unicast
			if removedRoute.Prefix.Addr().Is6() {
				family = nlri.IPv6Unicast
			}
			ctx := peer.packContext(family)
			update := buildWithdrawUpdate(removedRoute.Prefix, ctx)
			_ = peer.SendUpdate(update) // Best effort, continue on error
		}
	}
	r.mu.RUnlock()

	return nil
}

// Running returns true if the reactor is running.
func (r *Reactor) Running() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// Peers returns all configured peers.
func (r *Reactor) Peers() []*Peer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	peers := make([]*Peer, 0, len(r.peers))
	for _, p := range r.peers {
		peers = append(peers, p)
	}
	return peers
}

// ListenAddr returns the listener's bound address.
//
// Deprecated: Use ListenAddrs() for multi-listener support.
func (r *Reactor) ListenAddr() net.Addr {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return legacy listener if set
	if r.listener != nil {
		return r.listener.Addr()
	}
	// Return first listener from multi-listener map (for backward compat)
	for _, l := range r.listeners {
		if addr := l.Addr(); addr != nil {
			return addr
		}
	}
	return nil
}

// ListenAddrs returns all addresses the reactor is listening on.
func (r *Reactor) ListenAddrs() []net.Addr {
	r.mu.RLock()
	defer r.mu.RUnlock()

	addrs := make([]net.Addr, 0, len(r.listeners)+1)

	// Include legacy listener if set
	if r.listener != nil {
		if addr := r.listener.Addr(); addr != nil {
			addrs = append(addrs, addr)
		}
	}

	// Include all multi-listeners
	for _, l := range r.listeners {
		if addr := l.Addr(); addr != nil {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

// Stats returns current reactor statistics.
func (r *Reactor) Stats() *Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &Stats{
		StartTime: r.startTime,
		PeerCount: len(r.peers),
	}
	if r.running {
		stats.Uptime = time.Since(r.startTime)
	}
	return stats
}

// SetConnectionCallback sets the callback for matched incoming connections.
func (r *Reactor) SetConnectionCallback(cb ConnectionCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connCallback = cb
}

// SetMessageReceiver sets the receiver for raw BGP messages.
// When set, OnMessageReceived is called with raw wire bytes for all message types.
// This allows the receiver to control parsing based on format configuration.
func (r *Reactor) SetMessageReceiver(receiver MessageReceiver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messageReceiver = receiver
}

// AddPeerObserver registers an observer for peer lifecycle events.
// Observers are called synchronously in registration order.
// MUST NOT block; use goroutine for slow processing.
func (r *Reactor) AddPeerObserver(obs PeerLifecycleObserver) {
	r.observersMu.Lock()
	defer r.observersMu.Unlock()
	r.peerObservers = append(r.peerObservers, obs)
}

// notifyPeerEstablished calls all observers when peer reaches Established.
func (r *Reactor) notifyPeerEstablished(peer *Peer) {
	r.observersMu.RLock()
	observers := r.peerObservers
	r.observersMu.RUnlock()

	for _, obs := range observers {
		obs.OnPeerEstablished(peer)
	}
}

// notifyPeerClosed calls all observers when peer leaves Established.
func (r *Reactor) notifyPeerClosed(peer *Peer, reason string) {
	r.observersMu.RLock()
	observers := r.peerObservers
	r.observersMu.RUnlock()

	for _, obs := range observers {
		obs.OnPeerClosed(peer, reason)
	}
}

// notifyMessageReceiver notifies the message receiver of a raw BGP message.
// Called from session when a BGP message is sent or received.
// peerAddr is used to look up full PeerInfo from the peers map.
// direction is "sent" or "received".
func (r *Reactor) notifyMessageReceiver(peerAddr netip.Addr, msgType message.MessageType, rawBytes []byte, direction string) {
	r.mu.RLock()
	receiver := r.messageReceiver
	peer, hasPeer := r.peers[peerAddr.String()]

	// Build PeerInfo while holding lock to avoid race on state
	var peerInfo api.PeerInfo
	if hasPeer {
		s := peer.Settings()
		peerInfo = api.PeerInfo{
			Address:      s.Address,
			LocalAddress: s.LocalAddress,
			LocalAS:      s.LocalAS,
			PeerAS:       s.PeerAS,
			RouterID:     s.RouterID,
			State:        peer.State().String(),
		}
	} else {
		peerInfo = api.PeerInfo{Address: peerAddr}
	}
	r.mu.RUnlock()

	if receiver == nil {
		return
	}

	// Copy raw bytes - the original slice is reused by session's read buffer.
	// Without this copy, the data would be corrupted on next message read.
	bytesCopy := make([]byte, len(rawBytes))
	copy(bytesCopy, rawBytes)

	// Assign message ID for all message types
	messageID := nextMsgID()

	msg := api.RawMessage{
		Type:      msgType,
		RawBytes:  bytesCopy,
		Timestamp: time.Now(),
		Direction: direction,
		MessageID: messageID,
	}

	// UPDATE-specific: create WireUpdate for lazy parsing and caching
	if msgType == message.TypeUPDATE && hasPeer {
		ctxID := peer.RecvContextID()

		// Create WireUpdate - wraps UPDATE payload with context
		wireUpdate := api.NewWireUpdate(bytesCopy, ctxID)
		msg.WireUpdate = wireUpdate

		// Derive AttrsWire from WireUpdate for backward compatibility
		msg.AttrsWire = wireUpdate.Attrs()

		// Cache the update for forwarding (only for received updates)
		if direction == "received" {
			r.recentUpdates.Add(&ReceivedUpdate{
				UpdateID:     messageID,
				RawBytes:     bytesCopy, // Already a copy, safe to store
				Attrs:        msg.AttrsWire,
				SourcePeerIP: peerAddr,
				SourceCtxID:  ctxID,
				ReceivedAt:   msg.Timestamp,
			})
		}
	}

	// Route to appropriate handler based on direction
	if direction == "sent" {
		receiver.OnMessageSent(peerInfo, msg)
	} else {
		receiver.OnMessageReceived(peerInfo, msg)
	}
}

// AddPeer adds a peer to the reactor.
func (r *Reactor) AddPeer(settings *PeerSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Normalize peer Address for consistent lookup (handles IPv4-mapped IPv6)
	// This ensures connections from 10.0.0.1 match peers configured as ::ffff:10.0.0.1
	settings.Address = settings.Address.Unmap()

	key := settings.Address.String()
	if _, exists := r.peers[key]; exists {
		return ErrPeerExists
	}

	// Validate and normalize LocalAddress (only if set)
	if settings.LocalAddress.IsValid() {
		// Normalize IPv4-mapped IPv6 addresses (e.g., ::ffff:192.168.1.1 -> 192.168.1.1)
		settings.LocalAddress = settings.LocalAddress.Unmap()

		// Check self-referential (Address == LocalAddress)
		// Allow for loopback (127.0.0.0/8 or ::1) to support testing with next-hop self
		isLoopback := settings.Address.IsLoopback() && settings.LocalAddress.IsLoopback()
		if settings.Address == settings.LocalAddress && !isLoopback {
			return fmt.Errorf("peer %s: address cannot equal local-address", settings.Address)
		}

		// Check link-local IPv6 (requires zone ID, not portable)
		if settings.LocalAddress.Is6() && settings.LocalAddress.IsLinkLocalUnicast() {
			return fmt.Errorf("peer %s: link-local addresses not supported for local-address", settings.Address)
		}

		// Check address family mismatch (IPv4 peer with IPv6 LocalAddress or vice versa)
		// Note: Both Address and LocalAddress are already unmapped at this point
		if settings.Address.Is4() != settings.LocalAddress.Is4() {
			return fmt.Errorf("peer %s: address family mismatch (IPv4/IPv6)", settings.Address)
		}
	}

	peer := NewPeer(settings)
	peer.SetGlobalWatchdog(r.watchdog)
	peer.SetReactor(r)
	// Set message callback to forward raw bytes to reactor's message receiver
	peer.messageCallback = r.notifyMessageReceiver
	r.peers[key] = peer

	// If reactor is running, start the peer and create listener if needed
	if r.running {
		// Start listener for this LocalAddress if it doesn't exist
		if settings.LocalAddress.IsValid() {
			if _, hasListener := r.listeners[settings.LocalAddress]; !hasListener {
				if err := r.startListenerForAddress(settings.LocalAddress); err != nil {
					// Rollback peer addition
					delete(r.peers, key)
					return err
				}
			}
		}
		peer.StartWithContext(r.ctx)
	}

	return nil
}

// RemovePeer removes a peer from the reactor.
func (r *Reactor) RemovePeer(addr netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Normalize address for consistent lookup (handles IPv4-mapped IPv6)
	addr = addr.Unmap()

	key := addr.String()
	peer, exists := r.peers[key]
	if !exists {
		return ErrPeerNotFound
	}

	localAddr := peer.Settings().LocalAddress

	// Stop peer if running
	peer.Stop()

	delete(r.peers, key)

	// Check if any other peer uses this LocalAddress
	if localAddr.IsValid() {
		stillUsed := false
		for _, p := range r.peers {
			if p.Settings().LocalAddress == localAddr {
				stillUsed = true
				break
			}
		}

		// Stop listener if no longer needed
		if !stillUsed {
			if listener, ok := r.listeners[localAddr]; ok {
				listener.Stop()
				waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = listener.Wait(waitCtx)
				cancel()
				delete(r.listeners, localAddr)
			}
		}
	}

	return nil
}

// Start begins the reactor with a background context.
func (r *Reactor) Start() error {
	return r.StartWithContext(context.Background())
}

// StartWithContext begins the reactor with the given context.
func (r *Reactor) StartWithContext(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return ErrAlreadyRunning
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.startTime = time.Now()

	// Start legacy listener if ListenAddr is configured (backward compatibility)
	if r.config.ListenAddr != "" {
		r.listener = NewListener(r.config.ListenAddr)
		r.listener.SetHandler(r.handleConnection)
		if err := r.listener.StartWithContext(r.ctx); err != nil {
			r.cancel()
			return err
		}
	}

	// Start multi-listeners based on peer LocalAddresses (new behavior)
	// Collect unique LocalAddresses from peers
	localAddrs := make(map[netip.Addr]struct{})
	for _, peer := range r.peers {
		localAddr := peer.Settings().LocalAddress
		if localAddr.IsValid() {
			localAddrs[localAddr] = struct{}{}
		}
	}

	// Create listener for each unique LocalAddress
	for addr := range localAddrs {
		if err := r.startListenerForAddress(addr); err != nil {
			// Cleanup already-started listeners on error
			r.stopAllListeners()
			if r.listener != nil {
				r.listener.Stop()
			}
			r.cancel()
			return err
		}
	}

	// Start API server if configured
	if r.config.APISocketPath != "" {
		apiConfig := &api.ServerConfig{
			SocketPath: r.config.APISocketPath,
		}
		// Convert process configs
		for _, pc := range r.config.APIProcesses {
			apiConfig.Processes = append(apiConfig.Processes, api.ProcessConfig{
				Name:          pc.Name,
				Run:           pc.Run,
				Encoder:       pc.Encoder,
				Respawn:       pc.Respawn,
				WorkDir:       r.config.ConfigDir,
				ReceiveUpdate: pc.ReceiveUpdate,
			})
		}
		r.api = api.NewServer(apiConfig, &reactorAPIAdapter{r})
		// Set API server as message receiver for raw byte access
		r.messageReceiver = r.api
		// Register API state observer for peer lifecycle events
		r.AddPeerObserver(&apiStateObserver{server: r.api, reactor: r})

		// Set process count for API sync - wait for all processes to send "api ready"
		r.SetAPIProcessCount(len(r.config.APIProcesses))

		if err := r.api.StartWithContext(r.ctx); err != nil {
			r.stopAllListeners()
			if r.listener != nil {
				r.listener.Stop()
			}
			r.cancel()
			return err
		}
	}

	// Start signal handler
	r.signals = NewSignalHandler()
	r.signals.OnShutdown(func() {
		r.Stop()
	})
	r.signals.StartWithContext(r.ctx)

	// Wait for API processes to signal readiness before starting peers.
	// All processes must send "session api ready" before BGP sessions start.
	r.WaitForAPIReady()

	// Start all peers (passive peers wait for incoming connections).
	for _, peer := range r.peers {
		peer.StartWithContext(r.ctx)
	}

	r.running = true

	// Monitor context for shutdown
	r.wg.Add(1)
	go r.monitor()

	return nil
}

// Stop signals the reactor to stop.
func (r *Reactor) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// startListenerForAddress creates and starts a listener for the given local address.
// Must be called with r.mu held.
func (r *Reactor) startListenerForAddress(addr netip.Addr) error {
	// Check if listener already exists for this address
	if _, exists := r.listeners[addr]; exists {
		return nil // Already listening on this address
	}

	// Use config.Port directly (0 means ephemeral port for testing)
	// Production configs should set Port explicitly (typically 179)
	listenAddr := net.JoinHostPort(addr.String(), strconv.Itoa(r.config.Port))
	listener := NewListener(listenAddr)

	// Capture addr in closure so handleConnectionWithContext knows which listener accepted
	localAddr := addr
	listener.SetHandler(func(conn net.Conn) {
		r.handleConnectionWithContext(conn, localAddr)
	})

	if err := listener.StartWithContext(r.ctx); err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	r.listeners[addr] = listener
	return nil
}

// stopAllListeners stops all multi-listeners and waits for them to finish.
// Must be called with r.mu held.
func (r *Reactor) stopAllListeners() {
	for addr, listener := range r.listeners {
		listener.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = listener.Wait(waitCtx)
		cancel()
		delete(r.listeners, addr)
	}
}

// Wait waits for the reactor to stop.
func (r *Reactor) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// monitor watches for shutdown and cleans up.
func (r *Reactor) monitor() {
	defer r.wg.Done()

	<-r.ctx.Done()

	r.cleanup()
}

// cleanup stops all components.
func (r *Reactor) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop API server
	if r.api != nil {
		r.api.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.api.Wait(waitCtx)
		cancel()
	}

	// Stop legacy listener
	if r.listener != nil {
		r.listener.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.listener.Wait(waitCtx)
		cancel()
	}

	// Stop all multi-listeners
	r.stopAllListeners()

	// Stop signal handler
	if r.signals != nil {
		r.signals.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = r.signals.Wait(waitCtx)
		cancel()
	}

	// Stop all peers
	for _, peer := range r.peers {
		peer.Stop()
	}

	// Wait for all peers
	for _, peer := range r.peers {
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = peer.Wait(waitCtx)
		cancel()
	}

	// Stop RIB store workers
	if r.ribStore != nil {
		r.ribStore.Stop()
	}

	r.running = false
	r.cancel = nil
}

// handleConnection handles an incoming TCP connection.
// RFC 4271 §6.8: Connection collision detection.
//
// Architecture:
//
//	handleConnection()
//	├── ESTABLISHED → rejectConnectionCollision() [NOTIFICATION 6/7]
//	├── OpenConfirm → SetPendingConnection() + go handlePendingCollision()
//	│                  └── Read OPEN → ResolvePendingCollision()
//	│                       ├── Local wins → rejectConnectionCollision()
//	│                       └── Remote wins → CloseWithNotification() existing
//	│                                        + acceptPendingConnection()
//	└── Other states → normal AcceptConnection()
func (r *Reactor) handleConnection(conn net.Conn) {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		_ = conn.Close()
		return
	}
	peerIP, _ := netip.AddrFromSlice(remoteAddr.IP)
	peerIP = peerIP.Unmap() // Handle IPv4-mapped IPv6

	r.mu.RLock()
	peer, exists := r.peers[peerIP.String()]
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		// Unknown peer, close connection
		_ = conn.Close()
		return
	}

	settings := peer.Settings()

	// Call callback if set
	if cb != nil {
		cb(conn, settings)
		return
	}

	// RFC 4271 §6.8: Check for collision with ESTABLISHED session.
	// "collision with existing BGP connection that is in the Established
	// state causes closing of the newly created connection"
	if peer.State() == PeerStateEstablished {
		r.rejectConnectionCollision(conn)
		return
	}

	// RFC 4271 §6.8: Check for collision with OpenConfirm session.
	// Queue the connection and wait for OPEN to compare BGP IDs.
	if peer.SessionState() == fsm.StateOpenConfirm {
		if err := peer.SetPendingConnection(conn); err != nil {
			// Already have a pending connection, reject this one
			r.rejectConnectionCollision(conn)
			return
		}
		// Start goroutine to read OPEN and resolve collision
		go r.handlePendingCollision(peer, conn)
		return
	}

	// Accept connection on peer's session.
	// For passive peers, this triggers the BGP handshake.
	if err := peer.AcceptConnection(conn); err != nil {
		_ = conn.Close()
	}
}

// handleConnectionWithContext handles an incoming TCP connection with listener context.
// listenerAddr is the local address the listener is bound to.
// This validates that the connection arrived on the expected listener for RFC compliance.
func (r *Reactor) handleConnectionWithContext(conn net.Conn, listenerAddr netip.Addr) {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		_ = conn.Close()
		return
	}
	peerIP, _ := netip.AddrFromSlice(remoteAddr.IP)
	peerIP = peerIP.Unmap() // Handle IPv4-mapped IPv6

	r.mu.RLock()
	peer, exists := r.peers[peerIP.String()]
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		// Unknown peer, close connection
		_ = conn.Close()
		return
	}

	settings := peer.Settings()

	// RFC compliance: verify connection arrived on expected listener
	// This validates peer connected to the correct LocalAddress
	if settings.LocalAddress.IsValid() && settings.LocalAddress != listenerAddr {
		// Connection to wrong local address - misconfiguration or routing anomaly
		// Log and reject (logging infrastructure not used here, just close)
		_ = conn.Close()
		return
	}

	// Call callback if set
	if cb != nil {
		cb(conn, settings)
		return
	}

	// RFC 4271 §6.8: Check for collision with ESTABLISHED session.
	if peer.State() == PeerStateEstablished {
		r.rejectConnectionCollision(conn)
		return
	}

	// RFC 4271 §6.8: Check for collision with OpenConfirm session.
	if peer.SessionState() == fsm.StateOpenConfirm {
		if err := peer.SetPendingConnection(conn); err != nil {
			r.rejectConnectionCollision(conn)
			return
		}
		go r.handlePendingCollision(peer, conn)
		return
	}

	// Accept connection on peer's session.
	if err := peer.AcceptConnection(conn); err != nil {
		_ = conn.Close()
	}
}

// rejectConnectionCollision sends NOTIFICATION Cease/Connection Collision (6/7)
// and closes the connection. RFC 4271 §6.8.
func (r *Reactor) rejectConnectionCollision(conn net.Conn) {
	notif := &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseConnectionCollision,
	}
	data, err := notif.Pack(nil)
	if err == nil {
		_, _ = conn.Write(data)
	}
	_ = conn.Close()
}

// handlePendingCollision reads OPEN from a pending connection and resolves collision.
// RFC 4271 §6.8: Upon receipt of OPEN, compare BGP IDs and close the loser.
func (r *Reactor) handlePendingCollision(peer *Peer, conn net.Conn) {
	buf := make([]byte, message.MaxMsgLen)

	// Set read deadline - use hold time or 90s default
	holdTime := peer.Settings().HoldTime
	if holdTime == 0 {
		holdTime = 90 * time.Second
	}
	_ = conn.SetReadDeadline(time.Now().Add(holdTime))

	// Read BGP header
	_, err := io.ReadFull(conn, buf[:message.HeaderLen])
	if err != nil {
		peer.ClearPendingConnection()
		_ = conn.Close()
		return
	}

	hdr, err := message.ParseHeader(buf[:message.HeaderLen])
	if err != nil {
		peer.ClearPendingConnection()
		r.rejectConnectionCollision(conn)
		return
	}

	// Must be OPEN message
	if hdr.Type != message.TypeOPEN {
		peer.ClearPendingConnection()
		r.rejectConnectionCollision(conn)
		return
	}

	// Read OPEN body
	_, err = io.ReadFull(conn, buf[message.HeaderLen:hdr.Length])
	if err != nil {
		peer.ClearPendingConnection()
		_ = conn.Close()
		return
	}

	// Parse OPEN
	open, err := message.UnpackOpen(buf[message.HeaderLen:hdr.Length])
	if err != nil {
		peer.ClearPendingConnection()
		r.rejectConnectionCollision(conn)
		return
	}

	// Resolve collision using BGP ID from OPEN
	acceptPending, pendingConn, pendingOpen := peer.ResolvePendingCollision(open)

	if !acceptPending {
		// Local wins: close pending with NOTIFICATION
		r.rejectConnectionCollision(pendingConn)
		return
	}

	// Remote wins: existing session is being closed, accept pending
	// We need to wait a bit for the existing session to close, then
	// start a new session with the pending connection
	r.acceptPendingConnection(peer, pendingConn, pendingOpen)
}

// acceptPendingConnection accepts a pending connection after collision resolution.
// The existing session has been closed, so we accept the pending connection with its pre-read OPEN.
func (r *Reactor) acceptPendingConnection(peer *Peer, conn net.Conn, open *message.Open) {
	// Wait for existing session to fully close
	// The CloseWithNotification was called in ResolvePendingCollision
	time.Sleep(100 * time.Millisecond)

	// Accept connection with the pre-received OPEN
	if err := peer.AcceptConnectionWithOpen(conn, open); err != nil {
		// Failed to accept - peer may have been stopped or old session not yet closed
		_ = conn.Close()
	}
}

// convertAPIMUPRoute converts an api.MUPRouteSpec to a reactor.MUPRoute.
// This function parses the string fields in the API spec into wire-format bytes.
func convertAPIMUPRoute(spec api.MUPRouteSpec) (MUPRoute, error) {
	route := MUPRoute{
		IsIPv6: spec.IsIPv6,
	}

	// Convert route type string to numeric
	switch spec.RouteType {
	case "mup-isd":
		route.RouteType = 1
	case "mup-dsd":
		route.RouteType = 2
	case "mup-t1st":
		route.RouteType = 3
	case "mup-t2st":
		route.RouteType = 4
	default:
		return route, fmt.Errorf("unknown MUP route type: %s", spec.RouteType)
	}

	// Parse NextHop
	if spec.NextHop != "" {
		ip, err := netip.ParseAddr(spec.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Build MUP NLRI bytes (simplified - reuse from config/loader pattern)
	nlriBytes, err := buildAPIMUPNLRI(spec)
	if err != nil {
		return route, fmt.Errorf("build MUP NLRI: %w", err)
	}
	route.NLRI = nlriBytes

	// Parse extended communities if present
	if spec.ExtCommunity != "" {
		ecBytes, err := parseAPIExtCommunity(spec.ExtCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = ecBytes
	}

	// Parse SRv6 Prefix-SID if present
	if spec.PrefixSID != "" {
		sidBytes, err := parseAPIPrefixSIDSRv6(spec.PrefixSID)
		if err != nil {
			return route, fmt.Errorf("parse prefix-sid-srv6: %w", err)
		}
		route.PrefixSID = sidBytes
	}

	return route, nil
}

// buildAPIMUPNLRI builds MUP NLRI bytes from API spec.
func buildAPIMUPNLRI(spec api.MUPRouteSpec) ([]byte, error) {
	// Determine route type code
	var routeType nlri.MUPRouteType
	switch spec.RouteType {
	case "mup-isd":
		routeType = nlri.MUPISD
	case "mup-dsd":
		routeType = nlri.MUPDSD
	case "mup-t1st":
		routeType = nlri.MUPT1ST
	case "mup-t2st":
		routeType = nlri.MUPT2ST
	default:
		return nil, fmt.Errorf("unknown MUP route type: %s", spec.RouteType)
	}

	// Parse RD
	var rd nlri.RouteDistinguisher
	if spec.RD != "" {
		parsed, err := parseRD(spec.RD)
		if err != nil {
			return nil, fmt.Errorf("invalid RD %q: %w", spec.RD, err)
		}
		rd = parsed
	}

	// Build route-type-specific data
	var data []byte
	switch routeType {
	case nlri.MUPISD:
		if spec.Prefix == "" {
			return nil, fmt.Errorf("MUP ISD requires prefix")
		}
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid ISD prefix %q: %w", spec.Prefix, err)
		}
		// Validate family match
		if spec.IsIPv6 != prefix.Addr().Is6() {
			expected := familyIPv4
			if spec.IsIPv6 {
				expected = familyIPv6
			}
			return nil, fmt.Errorf("prefix %q is not %s", spec.Prefix, expected)
		}
		data = buildMUPPrefix(prefix)

	case nlri.MUPDSD:
		if spec.Address == "" {
			return nil, fmt.Errorf("MUP DSD requires address")
		}
		addr, err := netip.ParseAddr(spec.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid DSD address %q: %w", spec.Address, err)
		}
		// Validate family match
		if spec.IsIPv6 != addr.Is6() {
			expected := familyIPv4
			if spec.IsIPv6 {
				expected = familyIPv6
			}
			return nil, fmt.Errorf("address %q is not %s", spec.Address, expected)
		}
		data = addr.AsSlice()

	case nlri.MUPT1ST:
		if spec.Prefix == "" {
			return nil, fmt.Errorf("MUP T1ST requires prefix")
		}
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid T1ST prefix %q: %w", spec.Prefix, err)
		}
		// Validate family match
		if spec.IsIPv6 != prefix.Addr().Is6() {
			expected := familyIPv4
			if spec.IsIPv6 {
				expected = familyIPv6
			}
			return nil, fmt.Errorf("prefix %q is not %s", spec.Prefix, expected)
		}
		data = buildMUPPrefix(prefix)
		// TODO: Add TEID, QFI, endpoint if needed

	case nlri.MUPT2ST:
		if spec.Address == "" {
			return nil, fmt.Errorf("MUP T2ST requires address")
		}
		ep, err := netip.ParseAddr(spec.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid T2ST endpoint %q: %w", spec.Address, err)
		}
		// Validate family match
		if spec.IsIPv6 != ep.Is6() {
			expected := familyIPv4
			if spec.IsIPv6 {
				expected = familyIPv6
			}
			return nil, fmt.Errorf("address %q is not %s", spec.Address, expected)
		}
		epBytes := ep.AsSlice()
		data = append(data, byte(len(epBytes)*8))
		data = append(data, epBytes...)
		// TODO: Add TEID encoding
	}

	// Determine AFI
	afi := nlri.AFIIPv4
	if spec.IsIPv6 {
		afi = nlri.AFIIPv6
	}

	mup := nlri.NewMUPFull(afi, nlri.MUPArch3GPP5G, routeType, rd, data)
	return mup.Pack(nil), nil
}

// buildMUPPrefix encodes a prefix for MUP NLRI.
func buildMUPPrefix(prefix netip.Prefix) []byte {
	bits := prefix.Bits()
	addr := prefix.Addr()
	addrBytes := addr.AsSlice()
	prefixBytes := (bits + 7) / 8
	result := make([]byte, 1+prefixBytes)
	result[0] = byte(bits)
	copy(result[1:], addrBytes[:prefixBytes])
	return result
}

// parseRD parses a Route Distinguisher string.
// Delegates to nlri.ParseRDString for the actual parsing.
func parseRD(s string) (nlri.RouteDistinguisher, error) {
	return nlri.ParseRDString(s)
}

// parseAPIExtCommunity parses extended community string to bytes.
func parseAPIExtCommunity(s string) ([]byte, error) {
	// Strip brackets if present: "[target:10:10]" -> "target:10:10"
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)

	// Parse "type:ASN:value" format (e.g., "target:10:10")
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid extended community: %s", s)
	}

	ecType := strings.ToLower(parts[0])
	switch ecType {
	case "target":
		// Route Target: Type 0x00, Subtype 0x02
		if len(parts) != 3 {
			return nil, fmt.Errorf("target requires ASN:value format")
		}
		var asn, val uint64
		if _, err := fmt.Sscanf(parts[1]+":"+parts[2], "%d:%d", &asn, &val); err != nil {
			return nil, fmt.Errorf("invalid target values: %s:%s", parts[1], parts[2])
		}
		ec := [8]byte{0x00, 0x02}
		ec[2] = byte(asn >> 8)
		ec[3] = byte(asn)
		ec[4] = byte(val >> 24)
		ec[5] = byte(val >> 16)
		ec[6] = byte(val >> 8)
		ec[7] = byte(val)
		return ec[:], nil

	case "origin":
		// Route Origin: Type 0x00, Subtype 0x03
		if len(parts) != 3 {
			return nil, fmt.Errorf("origin requires ASN:value format")
		}
		var asn, val uint64
		if _, err := fmt.Sscanf(parts[1]+":"+parts[2], "%d:%d", &asn, &val); err != nil {
			return nil, fmt.Errorf("invalid origin values: %s:%s", parts[1], parts[2])
		}
		ec := [8]byte{0x00, 0x03}
		ec[2] = byte(asn >> 8)
		ec[3] = byte(asn)
		ec[4] = byte(val >> 24)
		ec[5] = byte(val >> 16)
		ec[6] = byte(val >> 8)
		ec[7] = byte(val)
		return ec[:], nil

	default:
		return nil, fmt.Errorf("unknown extended community type: %s", ecType)
	}
}

// parseAPIPrefixSIDSRv6 parses SRv6 Prefix-SID string to bytes.
func parseAPIPrefixSIDSRv6(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}

	// Parse service type
	var serviceType byte
	switch {
	case strings.HasPrefix(s, "l3-service"):
		serviceType = 5 // TLV Type 5: SRv6 L3 Service
		s = strings.TrimPrefix(s, "l3-service")
	case strings.HasPrefix(s, "l2-service"):
		serviceType = 6 // TLV Type 6: SRv6 L2 Service
		s = strings.TrimPrefix(s, "l2-service")
	default:
		return nil, fmt.Errorf("invalid srv6 prefix-sid: expected l3-service or l2-service")
	}
	s = strings.TrimSpace(s)

	// Parse IPv6 address
	fields := strings.Fields(s)
	if len(fields) < 1 {
		return nil, fmt.Errorf("invalid srv6 prefix-sid: missing IPv6 address")
	}
	ipv6, err := netip.ParseAddr(fields[0])
	if err != nil || !ipv6.Is6() {
		return nil, fmt.Errorf("invalid srv6 prefix-sid: expected IPv6 address, got %q", fields[0])
	}

	var behavior byte
	var sidStruct []byte

	// Parse optional behavior (0xNN format)
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "0x") || strings.HasPrefix(f, "0X") {
			behVal, err := parseHexByte(f)
			if err != nil {
				return nil, fmt.Errorf("invalid srv6 behavior %q: %w", f, err)
			}
			behavior = behVal
		} else if strings.HasPrefix(f, "[") {
			// Parse SID structure [LB,LN,Func,Arg,TransLen,TransOffset]
			structStr := strings.TrimPrefix(f, "[")
			structStr = strings.TrimSuffix(structStr, "]")
			parts := strings.Split(structStr, ",")
			if len(parts) != 6 {
				return nil, fmt.Errorf("invalid srv6 SID structure: expected 6 values")
			}
			for _, p := range parts {
				v, err := parseUint8(strings.TrimSpace(p))
				if err != nil {
					return nil, fmt.Errorf("invalid srv6 SID structure value %q: %w", p, err)
				}
				sidStruct = append(sidStruct, v)
			}
		}
	}

	// Build wire format per RFC 9252
	var innerValue []byte
	innerValue = append(innerValue, 0) // reserved
	innerValue = append(innerValue, ipv6.AsSlice()...)
	innerValue = append(innerValue, 0)        // flags
	innerValue = append(innerValue, 0)        // reserved
	innerValue = append(innerValue, behavior) // behavior

	if len(sidStruct) == 6 {
		innerValue = append(innerValue, 0, 1)
		innerValue = append(innerValue, 0, byte(len(sidStruct)))
		innerValue = append(innerValue, sidStruct...)
	}

	innerLen := len(innerValue)
	innerTLV := []byte{0, 1, byte(innerLen >> 8), byte(innerLen)}
	innerTLV = append(innerTLV, innerValue...)

	outerLen := len(innerTLV)
	result := []byte{serviceType, byte(outerLen >> 8), byte(outerLen)}
	result = append(result, innerTLV...)

	return result, nil
}

func parseHexByte(s string) (byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	var v uint64
	_, err := fmt.Sscanf(s, "%x", &v)
	if err != nil || v > 255 {
		return 0, fmt.Errorf("invalid hex byte: %s", s)
	}
	return byte(v), nil
}

func parseUint8(s string) (byte, error) {
	var v uint64
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil || v > 255 {
		return 0, fmt.Errorf("invalid uint8: %s", s)
	}
	return byte(v), nil
}
