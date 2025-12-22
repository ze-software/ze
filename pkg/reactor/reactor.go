// Package reactor implements the BGP reactor - the main orchestrator
// that manages peer sessions, connections, and signal handling.
package reactor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/exa-networks/zebgp/pkg/api"
	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/exa-networks/zebgp/pkg/rib"
)

// Reactor errors.
var (
	ErrAlreadyRunning = errors.New("reactor already running")
	ErrNotRunning     = errors.New("reactor not running")
	ErrPeerExists     = errors.New("peer already exists")
	ErrPeerNotFound   = errors.New("peer not found")
)

// Config holds reactor configuration.
type Config struct {
	// ListenAddr is the address to listen on (e.g., "0.0.0.0:179").
	ListenAddr string

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
}

// APIProcessConfig holds external process configuration for the API.
type APIProcessConfig struct {
	Name    string
	Run     string
	Encoder string
	Respawn bool
}

// Stats holds reactor statistics.
type Stats struct {
	StartTime time.Time
	Uptime    time.Duration
	PeerCount int
}

// ConnectionCallback is called when a connection is matched to a peer.
type ConnectionCallback func(conn net.Conn, settings *PeerSettings)

// Reactor is the main BGP orchestrator.
//
// It manages:
//   - Peer connections (outgoing)
//   - Listener for incoming connections
//   - Signal handling
//   - Graceful shutdown
//   - API server for external communication
//   - RIB (Routing Information Base) for route storage
type Reactor struct {
	config *Config

	peers    map[string]*Peer // keyed by peer address
	listener *Listener
	signals  *SignalHandler
	api      *api.Server // API server for CLI and external processes

	// RIB components
	ribIn    *rib.IncomingRIB // Adj-RIB-In
	ribOut   *rib.OutgoingRIB // Adj-RIB-Out
	ribStore *rib.RouteStore  // Global deduplication store

	connCallback ConnectionCallback

	running   bool
	startTime time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex
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

	// Build route object for queueing
	var n nlri.NLRI
	if route.Prefix.Addr().Is4() {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
	} else {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
	}

	// Build attributes
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
		},
	}
	attrs := []attribute.Attribute{attribute.OriginIGP}

	ribRoute := rib.NewRouteWithASPath(n, route.NextHop, attrs, asPath)

	var lastErr error
	for _, peer := range peers {
		if peer.AdjRIBOut().InTransaction() {
			// Queue to Adj-RIB-Out (will be sent on commit)
			peer.AdjRIBOut().QueueAnnounce(ribRoute)
		} else {
			// Send immediately
			update := buildAnnounceUpdate(route, a.r.config.LocalAS)
			if peer.State() == PeerStateEstablished {
				if err := peer.SendUpdate(update); err != nil {
					lastErr = err
				}
			}
		}
	}
	return lastErr
}

// WithdrawRoute withdraws a route from matching peers.
// If a peer is in transaction, queues to its Adj-RIB-Out; otherwise sends immediately.
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
		if peer.AdjRIBOut().InTransaction() {
			// Queue withdrawal to Adj-RIB-Out (will be sent on commit)
			peer.AdjRIBOut().QueueWithdraw(n)
		} else {
			// Send immediately
			update := buildWithdrawUpdate(prefix)
			if peer.State() == PeerStateEstablished {
				if err := peer.SendUpdate(update); err != nil {
					lastErr = err
				}
			}
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
func buildAnnounceUpdate(route api.RouteSpec, localAS uint32) *message.Update {
	// Build path attributes
	var attrBytes []byte

	// 1. ORIGIN (IGP)
	attrBytes = append(attrBytes, attribute.PackAttribute(attribute.OriginIGP)...)

	// 2. AS_PATH (prepend local AS for eBGP, empty for iBGP routes we originate)
	// Always use 4-byte ASN for API-announced routes (modern standard)
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
		},
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, true)...)

	// 3. NEXT_HOP
	nextHop := &attribute.NextHop{Addr: route.NextHop}
	attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)

	// Build NLRI
	var nlriBytes []byte
	if route.Prefix.Addr().Is4() {
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
		nlriBytes = inet.Bytes()
	} else {
		// IPv6 requires MP_REACH_NLRI (attribute 14) instead of NLRI field
		// For now, only support IPv4 in the simple path
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
		nlriBytes = inet.Bytes()
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}
}

// buildWithdrawUpdate builds an UPDATE message for withdrawing a route.
func buildWithdrawUpdate(prefix netip.Prefix) *message.Update {
	var withdrawnBytes []byte

	if prefix.Addr().Is4() {
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
		withdrawnBytes = inet.Bytes()
	} else {
		// IPv6 requires MP_UNREACH_NLRI (attribute 15) instead of withdrawn field
		// For now, only support IPv4 in the simple path
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, prefix, 0)
		withdrawnBytes = inet.Bytes()
	}

	return &message.Update{
		WithdrawnRoutes: withdrawnBytes,
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

// TeardownPeer gracefully closes a peer session.
func (a *reactorAPIAdapter) TeardownPeer(addr netip.Addr, _ string) error {
	a.r.mu.RLock()
	peer, exists := a.r.peers[addr.String()]
	a.r.mu.RUnlock()

	if !exists {
		return ErrPeerNotFound
	}

	// TODO: Send NOTIFICATION with reason before stopping
	peer.Stop()
	return nil
}

// AnnounceEOR sends an End-of-RIB marker for the given address family.
func (a *reactorAPIAdapter) AnnounceEOR(peerSelector string, afi uint16, safi uint8) error {
	update := message.BuildEOR(nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)})
	return a.sendToMatchingPeers(peerSelector, update)
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
func (a *reactorAPIAdapter) RIBOutRoutes() []api.RIBRoute {
	if a.r.ribOut == nil {
		return nil
	}

	var routes []api.RIBRoute

	// Get pending routes for common families
	families := []nlri.Family{
		{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast},
		{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast},
	}

	for _, family := range families {
		for _, route := range a.r.ribOut.GetPending(family) {
			routes = append(routes, routeToAPIRoute("", route))
		}
	}

	return routes
}

// RIBStats returns RIB statistics.
func (a *reactorAPIAdapter) RIBStats() api.RIBStatsInfo {
	stats := api.RIBStatsInfo{}

	if a.r.ribIn != nil {
		inStats := a.r.ribIn.Stats()
		stats.InPeerCount = inStats.PeerCount
		stats.InRouteCount = inStats.RouteCount
	}

	if a.r.ribOut != nil {
		outStats := a.r.ribOut.Stats()
		stats.OutPending = outStats.PendingAnnouncements
		stats.OutWithdrawls = outStats.PendingWithdrawals
		stats.OutSent = outStats.SentRoutes
	}

	return stats
}

// Transaction support for commit-based batching.

// BeginTransaction starts a new transaction for batched route updates.
// peerSelector is "*" for all peers or a specific peer address.
func (a *reactorAPIAdapter) BeginTransaction(peerSelector, label string) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return api.ErrNoTransaction
	}

	var lastErr error
	for _, peer := range peers {
		if err := peer.AdjRIBOut().BeginTransaction(label); err != nil {
			lastErr = convertRIBError(err)
		}
	}
	return lastErr
}

// CommitTransaction commits the current transaction.
// After committing, flushes pending routes, groups them, and sends to peers.
func (a *reactorAPIAdapter) CommitTransaction(peerSelector string) (api.TransactionResult, error) {
	return a.CommitTransactionWithLabel(peerSelector, "")
}

// CommitTransactionWithLabel commits, verifying the label matches.
// After committing, flushes pending routes, groups them, and sends to peers.
func (a *reactorAPIAdapter) CommitTransactionWithLabel(peerSelector, label string) (api.TransactionResult, error) {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return api.TransactionResult{}, api.ErrNoTransaction
	}

	var totalResult api.TransactionResult
	var lastErr error

	for _, peer := range peers {
		ribOut := peer.AdjRIBOut()

		var stats rib.CommitStats
		var err error

		if label != "" {
			stats, err = ribOut.CommitTransactionWithLabel(label)
		} else {
			stats, err = ribOut.CommitTransaction()
		}

		if err != nil {
			lastErr = convertRIBError(err)
			continue
		}

		// Flush and send for this peer
		updatesSent := a.flushAndSendForPeer(peer)

		totalResult.RoutesAnnounced += stats.RoutesAnnounced
		totalResult.RoutesWithdrawn += stats.RoutesWithdrawn
		totalResult.UpdatesSent += updatesSent
		totalResult.TransactionID = ribOut.TransactionID()
	}

	if lastErr != nil {
		return totalResult, lastErr
	}
	return totalResult, nil
}

// flushAndSendForPeer flushes pending routes for a peer, groups them, and sends.
// Returns the number of UPDATE messages sent.
func (a *reactorAPIAdapter) flushAndSendForPeer(peer *Peer) int {
	ribOut := peer.AdjRIBOut()
	if ribOut == nil {
		return 0
	}

	// Flush all pending routes
	routes := ribOut.FlushAllPending()
	if len(routes) == 0 {
		return 0
	}

	// Get negotiated parameters for CommitService
	neg := peer.messageNegotiated()
	if neg == nil {
		return 0
	}

	// Use CommitService with two-level grouping for proper AS_PATH handling
	cs := rib.NewCommitService(peer, neg, true)
	stats, err := cs.Commit(routes, rib.CommitOptions{SendEOR: false})
	if err != nil {
		return 0
	}

	return stats.UpdatesSent
}

// RollbackTransaction discards all queued routes in the transaction.
func (a *reactorAPIAdapter) RollbackTransaction(peerSelector string) (api.TransactionResult, error) {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return api.TransactionResult{}, api.ErrNoTransaction
	}

	var totalResult api.TransactionResult
	var lastErr error

	for _, peer := range peers {
		ribOut := peer.AdjRIBOut()
		txID := ribOut.TransactionID()
		stats, err := ribOut.RollbackTransaction()
		if err != nil {
			lastErr = convertRIBError(err)
			continue
		}

		totalResult.RoutesDiscarded += stats.RoutesDiscarded
		totalResult.TransactionID = txID
	}

	if lastErr != nil {
		return totalResult, lastErr
	}
	return totalResult, nil
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
func (a *reactorAPIAdapter) InTransaction(peerSelector string) bool {
	peers := a.getMatchingPeers(peerSelector)
	for _, peer := range peers {
		if peer.AdjRIBOut().InTransaction() {
			return true
		}
	}
	return false
}

// TransactionID returns the transaction label for the first matching peer.
func (a *reactorAPIAdapter) TransactionID(peerSelector string) string {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return ""
	}
	return peers[0].AdjRIBOut().TransactionID()
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

// sendWithdrawals sends withdrawal UPDATE messages to a peer.
// Groups withdrawals by family: IPv4 unicast uses WithdrawnRoutes field,
// other families use MP_UNREACH_NLRI attribute.
// Returns number of UPDATE messages sent.
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
		var update *message.Update

		if family == ipv4Unicast {
			// IPv4 unicast: use WithdrawnRoutes field
			var withdrawnBytes []byte
			for _, n := range nlris {
				withdrawnBytes = append(withdrawnBytes, n.Bytes()...)
			}
			update = &message.Update{
				WithdrawnRoutes: withdrawnBytes,
			}
		} else {
			// Other families: use MP_UNREACH_NLRI attribute
			var nlriBytes []byte
			for _, n := range nlris {
				nlriBytes = append(nlriBytes, n.Bytes()...)
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

// convertRIBError converts RIB errors to API errors.
func convertRIBError(err error) error {
	switch {
	case errors.Is(err, rib.ErrAlreadyInTransaction):
		return api.ErrAlreadyInTransaction
	case errors.Is(err, rib.ErrNoTransaction):
		return api.ErrNoTransaction
	case errors.Is(err, rib.ErrLabelMismatch):
		return api.ErrLabelMismatch
	default:
		return err
	}
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
	return &Reactor{
		config:   config,
		peers:    make(map[string]*Peer),
		ribIn:    rib.NewIncomingRIB(),
		ribOut:   rib.NewOutgoingRIB(),
		ribStore: rib.NewRouteStore(100), // Buffer size for dedup workers
	}
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
func (r *Reactor) ListenAddr() net.Addr {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.listener != nil {
		return r.listener.Addr()
	}
	return nil
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

// AddPeer adds a peer to the reactor.
func (r *Reactor) AddPeer(settings *PeerSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := settings.Address.String()
	if _, exists := r.peers[key]; exists {
		return ErrPeerExists
	}

	peer := NewPeer(settings)
	r.peers[key] = peer

	// If reactor is running, start the peer
	if r.running {
		peer.StartWithContext(r.ctx)
	}

	return nil
}

// RemovePeer removes a peer from the reactor.
func (r *Reactor) RemovePeer(addr netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := addr.String()
	peer, exists := r.peers[key]
	if !exists {
		return ErrPeerNotFound
	}

	// Stop peer if running
	peer.Stop()

	delete(r.peers, key)
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

	// Start listener
	if r.config.ListenAddr != "" {
		r.listener = NewListener(r.config.ListenAddr)
		r.listener.SetHandler(r.handleConnection)
		if err := r.listener.StartWithContext(r.ctx); err != nil {
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
				Name:    pc.Name,
				Run:     pc.Run,
				Encoder: pc.Encoder,
				Respawn: pc.Respawn,
				WorkDir: r.config.ConfigDir,
			})
		}
		r.api = api.NewServer(apiConfig, &reactorAPIAdapter{r})
		if err := r.api.StartWithContext(r.ctx); err != nil {
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

	// Stop listener
	if r.listener != nil {
		r.listener.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.listener.Wait(waitCtx)
		cancel()
	}

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

	// Accept connection on peer's session.
	// For passive peers, this triggers the BGP handshake.
	if err := peer.AcceptConnection(conn); err != nil {
		_ = conn.Close()
	}
}
