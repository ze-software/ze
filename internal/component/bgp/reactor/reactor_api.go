// Design: docs/architecture/core-design.md — reactor API adapter for plugin integration
// Overview: reactor.go — Reactor struct, lifecycle, and connection management
// Related: reactor_wire.go — zero-allocation wire UPDATE builders
// Detail: reactor_api_routes.go — family-specific route announce/withdraw (L3VPN, labeled, MUP)
// Detail: reactor_api_batch.go — NLRI batch operations and wire attribute building
// Detail: reactor_api_forward.go — UPDATE forwarding, grouped sending, cache ops
package reactor

import (
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"strconv"
	"strings"
	"time"

	bgpserver "codeberg.org/thomas-mangin/ze/internal/component/bgp/server"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// addPathSendDirection is the ADD-PATH direction value for families where we send path-IDs.
// RFC 7911 Section 4: "send" means we include a Path Identifier when advertising.
const addPathSendDirection = "send"

// apiStateObserver emits peer state change messages via the EventDispatcher.
type apiStateObserver struct {
	dispatcher *bgpserver.EventDispatcher
	reactor    *Reactor
}

func (o *apiStateObserver) OnPeerEstablished(peer *Peer) {
	if o.dispatcher == nil {
		return
	}
	s := peer.Settings()
	peerInfo := plugin.PeerInfo{
		Address:      s.Address,
		LocalAddress: s.LocalAddress,
		LocalAS:      s.LocalAS,
		PeerAS:       s.PeerAS,
		RouterID:     s.RouterID,
		State:        peer.State().String(),
	}
	o.dispatcher.OnPeerStateChange(peerInfo, "up", "")
}

func (o *apiStateObserver) OnPeerClosed(peer *Peer, reason string) {
	if o.dispatcher == nil {
		return
	}
	s := peer.Settings()
	peerInfo := plugin.PeerInfo{
		Address:      s.Address,
		LocalAddress: s.LocalAddress,
		LocalAS:      s.LocalAS,
		PeerAS:       s.PeerAS,
		RouterID:     s.RouterID,
		State:        peer.State().String(),
	}
	o.dispatcher.OnPeerStateChange(peerInfo, "down", reason)
}

// reactorAPIAdapter implements plugin.ReactorLifecycle + bgptypes.BGPReactor for the Reactor.
type reactorAPIAdapter struct {
	r *Reactor
}

// Peers returns peer information for the API.
func (a *reactorAPIAdapter) Peers() []plugin.PeerInfo {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	result := make([]plugin.PeerInfo, 0, len(a.r.peers))
	for _, p := range a.r.peers {
		s := p.Settings()
		stats := p.Stats()
		info := plugin.PeerInfo{
			Address:          s.Address,
			LocalAddress:     s.LocalAddress,
			LocalAS:          s.LocalAS,
			PeerAS:           s.PeerAS,
			RouterID:         s.RouterID,
			State:            p.State().String(),
			MessagesReceived: stats.MessagesReceived,
			MessagesSent:     stats.MessagesSent,
			RoutesReceived:   stats.RoutesReceived,
			RoutesSent:       stats.RoutesSent,
		}
		if estAt := p.EstablishedAt(); !estAt.IsZero() {
			info.Uptime = a.r.clock.Now().Sub(estAt)
		}
		result = append(result, info)
	}
	return result
}

// PeerNegotiatedCapabilities returns negotiated capabilities for a peer.
// Returns nil if peer not found or negotiation not complete.
func (a *reactorAPIAdapter) PeerNegotiatedCapabilities(addr netip.Addr) *plugin.PeerCapabilitiesInfo {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	peer, ok := a.r.findPeerByAddr(addr)
	if !ok {
		return nil
	}

	neg := peer.negotiated.Load()
	if neg == nil {
		return nil
	}

	families := neg.Families()
	familyStrs := make([]string, len(families))
	for i, f := range families {
		familyStrs[i] = f.String()
	}

	// Build ADD-PATH map: family → direction for families where we send path-IDs.
	// RFC 7911: ADD-PATH is negotiated per-family in sendCtx.
	var addPath map[string]string
	for _, f := range families {
		if peer.addPathFor(f) {
			if addPath == nil {
				addPath = make(map[string]string)
			}
			addPath[f.String()] = addPathSendDirection
		}
	}

	return &plugin.PeerCapabilitiesInfo{
		Families:             familyStrs,
		ExtendedMessage:      neg.ExtendedMessage,
		EnhancedRouteRefresh: neg.EnhancedRouteRefresh,
		ASN4:                 neg.ASN4,
		AddPath:              addPath,
	}
}

// SoftClearPeer sends ROUTE-REFRESH for all negotiated families of matching peers.
// RFC 2918 Section 3: soft reset via route refresh.
func (a *reactorAPIAdapter) SoftClearPeer(peerSelector string) ([]string, error) {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	familySet := make(map[string]bool)
	var lastErr error
	matched := false

	for addrStr, peer := range a.r.peers {
		if !ipGlobMatch(peerSelector, addrStr) {
			continue
		}
		if peer.State() != PeerStateEstablished {
			continue
		}
		matched = true

		neg := peer.negotiated.Load()
		if neg == nil {
			continue
		}

		for _, f := range neg.Families() {
			rr := &message.RouteRefresh{
				AFI:     message.AFI(f.AFI),
				SAFI:    message.SAFI(f.SAFI),
				Subtype: message.RouteRefreshNormal,
			}
			data := message.PackTo(rr, nil)
			if err := peer.SendRawMessage(0, data); err != nil {
				lastErr = err
			} else {
				familySet[f.String()] = true
			}
		}
	}

	if !matched {
		return nil, ErrPeerNotFound
	}

	families := make([]string, 0, len(familySet))
	for f := range familySet {
		families = append(families, f)
	}

	return families, lastErr
}

// GetPeerCapabilityConfigs returns capability configurations for all peers.
// Used by plugin protocol Stage 2 to deliver matching config.
// Extracts known capability values into a flexible map for pattern matching.
func (a *reactorAPIAdapter) GetPeerCapabilityConfigs() []plugin.PeerCapabilityConfig {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	result := make([]plugin.PeerCapabilityConfig, 0, len(a.r.peers))
	for _, p := range a.r.peers {
		s := p.Settings()
		cfg := plugin.PeerCapabilityConfig{
			Address:        s.Address.String(),
			Values:         make(map[string]string),
			CapabilityJSON: s.CapabilityConfigJSON,
		}

		// Extract capability values via ConfigProvider interface.
		// Each capability that implements ConfigProvider returns its own
		// scoped key-value pairs (e.g., "rfc4724:restart-time" or "draft-xxx:field").
		// This allows new capabilities to be added without modifying this code.
		for _, cap := range s.Capabilities {
			if provider, ok := cap.(capability.ConfigProvider); ok {
				maps.Copy(cfg.Values, provider.ConfigValues())
			}
		}

		// Also include raw capability config values for plugin-declared capabilities.
		// Format: "<name>:<field>" -> value (RFC-style scoping, matches ConfigProvider pattern).
		// Server.go adds "capability " prefix when building path.
		for capName, fields := range s.RawCapabilityConfig {
			for fieldName, value := range fields {
				key := capName + ":" + fieldName
				cfg.Values[key] = value
			}
		}

		result = append(result, cfg)
	}
	return result
}

// GetConfigTree returns the full config as a map for plugin config delivery.
func (a *reactorAPIAdapter) GetConfigTree() map[string]any {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()
	return a.r.configTree
}

// SetConfigTree replaces the running config tree after a successful reload.
func (a *reactorAPIAdapter) SetConfigTree(tree map[string]any) {
	a.r.mu.Lock()
	defer a.r.mu.Unlock()
	a.r.configTree = tree
}

// Stats returns reactor statistics for the API.
func (a *reactorAPIAdapter) Stats() plugin.ReactorStats {
	stats := a.r.Stats()
	return plugin.ReactorStats{
		StartTime: stats.StartTime,
		Uptime:    stats.Uptime,
		PeerCount: stats.PeerCount,
		RouterID:  stats.RouterID,
		LocalAS:   stats.LocalAS,
	}
}

// Stop signals the reactor to stop.
func (a *reactorAPIAdapter) Stop() {
	a.r.Stop()
}

// AnnounceRoute announces a route to matching peers.
// If a peer is in transaction, queues to its Adj-RIB-Out; otherwise sends immediately.
func (a *reactorAPIAdapter) AnnounceRoute(peerSelector string, route bgptypes.RouteSpec) error {
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

	// Build attributes from RouteSpec.Wire (wire-first approach)
	var attrs []attribute.Attribute
	var userASPath []uint32

	if route.Wire != nil {
		// Parse attributes from wire format
		var err error
		attrs, err = route.Wire.All()
		if err != nil {
			return fmt.Errorf("failed to parse route attributes: %w", err)
		}
		// Extract AS_PATH if present
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				userASPath = asp.Segments[0].ASNs
			}
		}
	} else {
		// No wire attributes - use defaults
		attrs = append(attrs, attribute.OriginIGP)
	}

	var lastErr error
	for _, peer := range peers {
		isIBGP := peer.Settings().IsIBGP()

		// Resolve next-hop per peer using RouteNextHop policy
		nextHopAddr, nhErr := peer.resolveNextHop(route.NextHop, n.Family())
		if nhErr != nil {
			// Log but continue - skip this peer if next-hop can't be resolved
			routesLogger().Debug("next-hop resolution failed", "peer", peer.Settings().Address, "error", nhErr)
			continue
		}

		// Build AS_PATH: empty for iBGP, prepend LocalAS for eBGP
		// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS
		var asPath *attribute.ASPath
		switch {
		case len(userASPath) > 0:
			asPath = &attribute.ASPath{
				Segments: []attribute.ASPathSegment{
					{Type: attribute.ASSequence, ASNs: userASPath},
				},
			}
		case isIBGP:
			asPath = &attribute.ASPath{Segments: nil}
		default: // eBGP: prepend local AS
			asPath = &attribute.ASPath{
				Segments: []attribute.ASPathSegment{
					{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
				},
			}
		}

		ribRoute := rib.NewRouteWithASPath(n, nextHopAddr, attrs, asPath)

		// Create resolved route spec for buildAnnounceUpdate
		resolvedRoute := route
		resolvedRoute.NextHop = bgptypes.NewNextHopExplicit(nextHopAddr)

		if !peer.ShouldQueue() {
			// RFC 4271 Section 4.3 - Send UPDATE immediately (zero-allocation path)
			if err := peer.SendAnnounce(resolvedRoute, a.r.config.LocalAS); err != nil {
				lastErr = err
			}
		} else {
			// Session not established or queue draining: queue to preserve order
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
		if !peer.ShouldQueue() {
			// RFC 4271 Section 4.3 - Send UPDATE immediately (zero-allocation path)
			if err := peer.SendWithdraw(prefix); err != nil {
				lastErr = err
			}
		} else {
			// Session not established or queue draining: queue to preserve order
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

// Reload reloads the configuration.
// It re-parses the config file and diffs peers:
// - New peers in config are added
// - Peers not in new config are removed
// - Peers with changed settings are removed and re-added
// Requires ConfigPath to be set and SetReloadFunc to be called.
func (a *reactorAPIAdapter) Reload() error {
	r := a.r

	// Check config path is set.
	configPath := r.config.ConfigPath
	if configPath == "" {
		return ErrNoConfigPath
	}

	// Check reload function is set.
	r.mu.RLock()
	reloadFn := r.reloadFunc
	r.mu.RUnlock()
	if reloadFn == nil {
		return ErrNoReloadFunc
	}

	// Get new peer configs from config file.
	newPeers, err := reloadFn(configPath)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	return a.reconcilePeers(newPeers, "reload")
}

// VerifyConfig validates peer settings without modifying reactor state.
// When reloadFunc is available (production), uses the full config parsing pipeline
// for accurate validation of all peer fields. Falls back to parsePeersFromTree
// for basic address validation in tests.
// Called by the reload coordinator during the verify phase.
func (a *reactorAPIAdapter) VerifyConfig(bgpTree map[string]any) error {
	if peers, err := a.loadPeersFullOrTree(bgpTree); err != nil {
		return err
	} else {
		_ = peers // verify only — discard result
		return nil
	}
}

// ApplyConfigDiff applies peer changes from config.
// When reloadFunc is available (production), uses the full config parsing pipeline
// so PeerSettings are complete (all fields from configToPeer). Falls back to
// parsePeersFromTree in tests.
// Called by the reload coordinator during the apply phase.
func (a *reactorAPIAdapter) ApplyConfigDiff(bgpTree map[string]any) error {
	newPeers, err := a.loadPeersFullOrTree(bgpTree)
	if err != nil {
		return fmt.Errorf("apply config diff: %w", err)
	}

	return a.reconcilePeers(newPeers, "apply config diff")
}

// loadPeersFullOrTree loads peers using the full config parsing pipeline when
// reloadFunc is available (production), or falls back to parsePeersFromTree
// for basic parsing in tests. The full pipeline produces PeerSettings with all
// fields populated (capabilities, static routes, etc.), avoiding false diffs
// in peerSettingsEqual.
func (a *reactorAPIAdapter) loadPeersFullOrTree(bgpTree map[string]any) ([]*PeerSettings, error) {
	r := a.r

	configPath := r.config.ConfigPath
	r.mu.RLock()
	reloadFn := r.reloadFunc
	r.mu.RUnlock()

	if reloadFn != nil && configPath != "" {
		return reloadFn(configPath)
	}

	return parsePeersFromTree(bgpTree)
}

// reconcilePeers diffs newPeers against the reactor's current peers and
// stops removed/changed peers, adds new/changed peers.
// The label parameter is used for log messages (e.g., "reload", "apply config diff").
func (a *reactorAPIAdapter) reconcilePeers(newPeers []*PeerSettings, label string) error {
	r := a.r

	// Build map of new peer settings for quick lookup.
	newPeerSettings := make(map[string]*PeerSettings)
	for _, p := range newPeers {
		newPeerSettings[p.PeerKey()] = p
	}

	// Get current peer addresses and settings snapshot.
	r.mu.RLock()
	currentPeers := make(map[string]*PeerSettings)
	for addr, peer := range r.peers {
		currentPeers[addr] = peer.Settings()
	}
	r.mu.RUnlock()

	// Categorize peers: to remove, to add, unchanged.
	var toRemove []string
	var toAdd []*PeerSettings

	for addr := range currentPeers {
		newSettings, exists := newPeerSettings[addr]
		if !exists {
			toRemove = append(toRemove, addr)
		} else if !peerSettingsEqual(currentPeers[addr], newSettings) {
			toRemove = append(toRemove, addr)
			toAdd = append(toAdd, newSettings)
			reactorLogger().Debug(label+": peer settings changed", "peer", addr)
		}
	}

	for addr, settings := range newPeerSettings {
		if _, exists := currentPeers[addr]; !exists {
			toAdd = append(toAdd, settings)
		}
	}

	// Remove peers.
	for _, addr := range toRemove {
		r.mu.Lock()
		if peer, ok := r.peers[addr]; ok {
			peer.Stop()
			delete(r.peers, addr)
			reactorLogger().Debug(label+": removed peer", "peer", addr)
		}
		r.mu.Unlock()
	}

	// Add peers.
	var addErrors []error
	for _, settings := range toAdd {
		if err := r.AddPeer(settings); err != nil {
			addErrors = append(addErrors, fmt.Errorf("add peer %s: %w", settings.Address, err))
		} else {
			reactorLogger().Debug(label+": added peer", "peer", settings.Address)
		}
	}

	if len(addErrors) > 0 {
		return errors.Join(addErrors...)
	}

	return nil
}

// parsePeersFromTree extracts PeerSettings from a BGP config tree.
// The tree uses "peer" as the key, with peer addresses as sub-keys.
// Field values are strings (from config parser's Tree.ToMap()).
func parsePeersFromTree(bgpTree map[string]any) ([]*PeerSettings, error) {
	peerSection, ok := bgpTree["peer"]
	if !ok {
		return nil, nil // No peers configured.
	}

	peerMap, ok := peerSection.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid peer section type: %T", peerSection)
	}

	var peers []*PeerSettings
	for addrStr, peerData := range peerMap {
		addr, err := netip.ParseAddr(addrStr)
		if err != nil {
			return nil, fmt.Errorf("invalid peer address %q: %w", addrStr, err)
		}

		fields, ok := peerData.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid peer data for %s: %T", addrStr, peerData)
		}

		var localAS, peerAS uint32
		if v, ok := fields["local-as"].(string); ok {
			parseUint32FromString(v, &localAS)
		}
		if v, ok := fields["peer-as"].(string); ok {
			parseUint32FromString(v, &peerAS)
		}

		settings := NewPeerSettings(addr, localAS, peerAS, 0)

		// Parse optional fields.
		if v, ok := fields["hold-time"].(string); ok {
			var ht uint32
			parseUint32FromString(v, &ht)
			if ht > 0 {
				settings.HoldTime = time.Duration(ht) * time.Second
			}
		}
		if v, ok := fields["connection"].(string); ok {
			mode, err := ParseConnectionMode(v)
			if err != nil {
				reactorLogger().Warn("invalid connection mode in peer config, using default", "peer", addr, "value", v, "error", err)
			} else {
				settings.Connection = mode
			}
		}
		if v, ok := fields["router-id"].(string); ok {
			if rid, err := netip.ParseAddr(v); err == nil && rid.Is4() {
				settings.RouterID = ipv4ToUint32(rid)
			}
		}

		peers = append(peers, settings)
	}

	return peers, nil
}

// parseUint32FromString parses a decimal string into a uint32.
func parseUint32FromString(s string, out *uint32) {
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return
		}
		n = n*10 + uint64(c-'0')
	}
	if n <= 0xFFFFFFFF {
		*out = uint32(n)
	}
}

// ipv4ToUint32 converts an IPv4 address to a uint32 (network byte order).
func ipv4ToUint32(addr netip.Addr) uint32 {
	b := addr.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// peerSettingsEqual compares two PeerSettings for reload diffing.
// Returns true if the settings are functionally equivalent.
func peerSettingsEqual(a, b *PeerSettings) bool {
	if a == nil || b == nil {
		return a == b
	}

	// Compare identity fields.
	if a.Address != b.Address ||
		a.LocalAS != b.LocalAS ||
		a.PeerAS != b.PeerAS ||
		a.RouterID != b.RouterID {
		return false
	}

	// Compare connectivity fields.
	if a.LocalAddress != b.LocalAddress ||
		a.Port != b.Port ||
		a.Connection != b.Connection {
		return false
	}

	// Compare behavior fields.
	if a.HoldTime != b.HoldTime ||
		a.GroupUpdates != b.GroupUpdates ||
		a.IgnoreFamilyMismatch != b.IgnoreFamilyMismatch ||
		a.DisableASN4 != b.DisableASN4 {
		return false
	}

	// Compare static routes count (deep comparison would be expensive).
	if len(a.StaticRoutes) != len(b.StaticRoutes) {
		return false
	}

	// Compare capabilities count (deep comparison would be expensive).
	if len(a.Capabilities) != len(b.Capabilities) {
		return false
	}

	return true
}

// TeardownPeer gracefully closes a peer session with NOTIFICATION.
// Sends Cease (6) with the specified subcode per RFC 4486.
func (a *reactorAPIAdapter) TeardownPeer(addr netip.Addr, subcode uint8) error {
	a.r.mu.RLock()
	peer, exists := a.r.findPeerByAddr(addr)
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

// PausePeer pauses reading from a specific peer's session.
func (a *reactorAPIAdapter) PausePeer(addr netip.Addr) error {
	return a.r.PausePeer(addr)
}

// ResumePeer resumes reading from a specific peer's session.
func (a *reactorAPIAdapter) ResumePeer(addr netip.Addr) error {
	return a.r.ResumePeer(addr)
}

// AddDynamicPeer adds a peer with the given configuration.
// Delegates to reactor's AddDynamicPeer which handles defaults.
func (a *reactorAPIAdapter) AddDynamicPeer(config plugin.DynamicPeerConfig) error {
	return a.r.AddDynamicPeer(config)
}

// RemovePeer removes a peer by address.
func (a *reactorAPIAdapter) RemovePeer(addr netip.Addr) error {
	return a.r.RemovePeer(addr)
}

// RIBInRoutes returns routes from Adj-RIB-In.
// Engine has no RIB — route storage is owned by plugins (bgp-rib, bgp-adj-rib-in).
func (a *reactorAPIAdapter) RIBInRoutes(_ string) []rib.RouteJSON {
	return nil
}

// RIBStats returns RIB statistics.
// Engine has no RIB — route storage is owned by plugins.
func (a *reactorAPIAdapter) RIBStats() bgptypes.RIBStatsInfo {
	return bgptypes.RIBStatsInfo{}
}

// ClearRIBIn clears all routes from Adj-RIB-In.
// Engine has no RIB — route storage is owned by plugins.
func (a *reactorAPIAdapter) ClearRIBIn() int {
	return 0
}

// GetPeerProcessBindings returns process bindings for a specific peer.
// Returns nil if peer not found.
// Resolves encoding inheritance: peer binding -> plugin encoder -> "text" default.
func (a *reactorAPIAdapter) GetPeerProcessBindings(peerAddr netip.Addr) []plugin.PeerProcessBinding {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	peer, ok := a.r.findPeerByAddr(peerAddr)
	if !ok {
		return nil
	}

	settings := peer.Settings()
	result := make([]plugin.PeerProcessBinding, 0, len(settings.ProcessBindings))
	for _, b := range settings.ProcessBindings {
		// Resolve encoding: peer override -> plugin default -> "text"
		encoding := b.Encoding
		if encoding == "" {
			encoding = a.getPluginEncoder(b.PluginName)
		}
		if encoding == "" {
			encoding = "text"
		}

		// Resolve format: peer override -> "parsed"
		format := b.Format
		if format == "" {
			format = "parsed"
		}

		result = append(result, plugin.PeerProcessBinding{
			PluginName:          b.PluginName,
			Encoding:            encoding,
			Format:              format,
			ReceiveUpdate:       b.ReceiveUpdate,
			ReceiveOpen:         b.ReceiveOpen,
			ReceiveNotification: b.ReceiveNotification,
			ReceiveKeepalive:    b.ReceiveKeepalive,
			ReceiveRefresh:      b.ReceiveRefresh,
			ReceiveState:        b.ReceiveState,
			ReceiveSent:         b.ReceiveSent,
			ReceiveNegotiated:   b.ReceiveNegotiated,
			SendUpdate:          b.SendUpdate,
			SendRefresh:         b.SendRefresh,
		})
	}
	return result
}

// getPluginEncoder returns the encoder for a plugin, or empty if not found.
func (a *reactorAPIAdapter) getPluginEncoder(name string) string {
	for _, pc := range a.r.config.Plugins {
		if pc.Name == name {
			return pc.Encoder
		}
	}
	return ""
}

// getMatchingPeers returns peers matching the selector.
// Supports: "*" (all peers), exact IP, glob patterns (e.g., "192.168.*.*"),
// or "!addr" exclusion (all peers except the named one).
func (a *reactorAPIAdapter) getMatchingPeers(selector string) []*Peer {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	// Exclusion: "!addr" returns all peers except the one matching addr.
	if strings.HasPrefix(selector, "!") {
		excluded := a.getMatchingPeersLocked(selector[1:])
		excludeSet := make(map[*Peer]struct{}, len(excluded))
		for _, p := range excluded {
			excludeSet[p] = struct{}{}
		}
		peers := make([]*Peer, 0, len(a.r.peers)-len(excluded))
		for _, peer := range a.r.peers {
			if _, skip := excludeSet[peer]; !skip {
				peers = append(peers, peer)
			}
		}
		return peers
	}

	return a.getMatchingPeersLocked(selector)
}

// getMatchingPeersLocked resolves a positive selector (no "!" prefix).
// Caller must hold a.r.mu (read or write).
func (a *reactorAPIAdapter) getMatchingPeersLocked(selector string) []*Peer {
	// Fast path: all peers
	if selector == "*" || selector == "" {
		peers := make([]*Peer, 0, len(a.r.peers))
		for _, peer := range a.r.peers {
			peers = append(peers, peer)
		}
		return peers
	}

	// Fast path: exact match by full key ("addr:port")
	if peer, ok := a.r.peers[selector]; ok {
		return []*Peer{peer}
	}

	// Try selector as bare IP with default port (API selectors are typically IPs)
	if !strings.Contains(selector, "*") {
		if peer, ok := a.r.peers[selector+":"+strconv.Itoa(int(DefaultBGPPort))]; ok {
			return []*Peer{peer}
		}
	}

	// Glob pattern match against the IP portion of each key
	var peers []*Peer
	for key, peer := range a.r.peers {
		// Extract IP from "addr:port" key for glob matching
		addr := key
		if idx := strings.LastIndex(key, ":"); idx >= 0 {
			addr = key[:idx]
		}
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
// Both pattern and ip may include a ":port" suffix, which is stripped before matching.
func ipGlobMatch(pattern, ip string) bool {
	// "*" or empty matches everything
	if pattern == "*" || pattern == "" {
		return true
	}

	// Strip port suffix from both pattern and ip (e.g., "127.0.0.1:179" -> "127.0.0.1").
	// Go formats IPv6+port as "[::1]:179", so LastIndex(":") correctly finds the port separator.
	if idx := strings.LastIndex(pattern, ":"); idx >= 0 {
		if _, err := strconv.Atoi(pattern[idx+1:]); err == nil {
			pattern = pattern[:idx]
		}
	}
	if idx := strings.LastIndex(ip, ":"); idx >= 0 {
		if _, err := strconv.Atoi(ip[idx+1:]); err == nil {
			ip = ip[:idx]
		}
	}

	// Check if pattern looks like IPv4 glob (contains dots)
	if strings.Contains(pattern, ".") && strings.Contains(ip, ".") {
		patternParts := strings.Split(pattern, ".")
		ipParts := strings.Split(ip, ".")

		if len(patternParts) != 4 || len(ipParts) != 4 {
			return false
		}

		for i := range 4 {
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

// SignalAPIReady signals that an API process is ready.
func (a *reactorAPIAdapter) SignalAPIReady() {
	a.r.SignalAPIReady()
}

// AddAPIProcessCount adds to the number of API processes to wait for.
func (a *reactorAPIAdapter) AddAPIProcessCount(count int) {
	a.r.AddAPIProcessCount(count)
}

// SignalPluginStartupComplete signals that all plugin phases are done.
func (a *reactorAPIAdapter) SignalPluginStartupComplete() {
	a.r.SignalPluginStartupComplete()
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
	peer, exists := a.r.findPeerByAddr(peerAddr)
	a.r.mu.RUnlock()

	if !exists {
		return ErrPeerNotFound
	}

	return peer.SendRawMessage(msgType, payload)
}
