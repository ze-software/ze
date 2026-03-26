// Design: docs/architecture/core-design.md — reactor API adapter for plugin integration
// Overview: reactor.go — Reactor struct, lifecycle, and connection management
// Related: reactor_wire.go — zero-allocation wire UPDATE builders
// Detail: reactor_api_batch.go — NLRI batch operations and wire attribute building
// Detail: reactor_api_forward.go — UPDATE forwarding, grouped sending, cache ops
package reactor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	bgpserver "codeberg.org/thomas-mangin/ze/internal/component/bgp/server"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
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
		Name:         s.Name,
		GroupName:    s.GroupName,
		LocalAS:      s.LocalAS,
		PeerAS:       s.PeerAS,
		RouterID:     s.RouterID,
		Connect:      s.Connection.Connect,
		Accept:       s.Connection.Accept,
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
		Name:         s.Name,
		GroupName:    s.GroupName,
		LocalAS:      s.LocalAS,
		PeerAS:       s.PeerAS,
		RouterID:     s.RouterID,
		Connect:      s.Connection.Connect,
		Accept:       s.Connection.Accept,
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
			Address:            s.Address,
			LocalAddress:       s.LocalAddress,
			Name:               s.Name,
			GroupName:          s.GroupName,
			LocalAS:            s.LocalAS,
			PeerAS:             s.PeerAS,
			RouterID:           s.RouterID,
			ReceiveHoldTime:    s.ReceiveHoldTime,
			SendHoldTime:       s.SendHoldTime,
			ConnectRetry:       s.ConnectRetry,
			Connect:            s.Connection.Connect,
			Accept:             s.Connection.Accept,
			State:              p.State().String(),
			UpdatesReceived:    stats.UpdatesReceived,
			UpdatesSent:        stats.UpdatesSent,
			KeepalivesReceived: stats.KeepalivesReceived,
			KeepalivesSent:     stats.KeepalivesSent,
			EORReceived:        stats.EORReceived,
			EORSent:            stats.EORSent,
			PrefixUpdated:      s.PrefixUpdated,
			PrefixWarnings:     p.PrefixWarnedFamilies(),
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

	for addrPort, peer := range a.r.peers {
		if !ipGlobMatch(peerSelector, addrPort.Addr().String()) {
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
	newPeerSettings := make(map[netip.AddrPort]*PeerSettings)
	for _, p := range newPeers {
		newPeerSettings[p.PeerKey()] = p
	}

	// Get current peer addresses and settings snapshot.
	r.mu.RLock()
	currentPeers := make(map[netip.AddrPort]*PeerSettings)
	for key, peer := range r.peers {
		currentPeers[key] = peer.Settings()
	}
	r.mu.RUnlock()

	// Categorize peers: to remove, to add, unchanged.
	var toRemove []netip.AddrPort
	var toAdd []*PeerSettings

	for key := range currentPeers {
		newSettings, exists := newPeerSettings[key]
		if !exists {
			toRemove = append(toRemove, key)
		} else if !peerSettingsEqual(currentPeers[key], newSettings) {
			toRemove = append(toRemove, key)
			toAdd = append(toAdd, newSettings)
			reactorLogger().Debug(label+": peer settings changed", "peer", key)
		}
	}

	for key, settings := range newPeerSettings {
		if _, exists := currentPeers[key]; !exists {
			toAdd = append(toAdd, settings)
		}
	}

	// Remove peers.
	for _, key := range toRemove {
		r.mu.Lock()
		if peer, ok := r.peers[key]; ok {
			peer.Stop()
			delete(r.peers, key)
			reactorLogger().Debug(label+": removed peer", "peer", key)
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
// The tree uses "peer" as the key, with peer names as sub-keys.
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
	for peerName, peerData := range peerMap {
		fields, ok := peerData.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid peer data for %s: %T", peerName, peerData)
		}

		// Read remote > ip and remote > as.
		remoteMap, _ := fields["remote"].(map[string]any)
		if remoteMap == nil {
			return nil, fmt.Errorf("peer %s: missing required remote container", peerName)
		}

		addrStr, _ := remoteMap["ip"].(string)
		if addrStr == "" {
			return nil, fmt.Errorf("peer %s: missing required remote ip", peerName)
		}
		addr, err := netip.ParseAddr(addrStr)
		if err != nil {
			return nil, fmt.Errorf("peer %s: invalid remote ip %q: %w", peerName, addrStr, err)
		}

		var peerAS uint32
		if v, ok := remoteMap["as"].(string); ok {
			parseUint32FromString(v, &peerAS)
		}

		// Read local > as (optional per-peer override).
		var localAS uint32
		if localMap, ok := fields["local"].(map[string]any); ok {
			if v, ok := localMap["as"].(string); ok {
				parseUint32FromString(v, &localAS)
			}
		}

		settings := NewPeerSettings(addr, localAS, peerAS, 0)
		settings.Name = peerName

		// Parse optional fields.
		if v, ok := fields["receive-hold-time"].(string); ok {
			var ht uint32
			parseUint32FromString(v, &ht)
			// RFC 4271 Section 4.2: Hold Time MUST be either zero or at least three seconds.
			if ht >= 1 && ht <= 2 {
				reactorLogger().Warn("invalid receive-hold-time in peer config, ignoring", "peer", peerName, "value", ht)
			} else if ht > 0 {
				settings.ReceiveHoldTime = time.Duration(ht) * time.Second
			}
		}
		if v, ok := fields["send-hold-time"].(string); ok {
			var sht uint32
			parseUint32FromString(v, &sht)
			// RFC 9687: Send Hold Timer must be 0 (auto) or >= 480 seconds.
			if sht != 0 && sht < 480 {
				reactorLogger().Warn("invalid send-hold-time in peer config, ignoring", "peer", peerName, "value", sht)
			} else {
				settings.SendHoldTime = time.Duration(sht) * time.Second
			}
		}
		if v, ok := fields["connect-retry"].(string); ok {
			var cr uint32
			parseUint32FromString(v, &cr)
			if cr > 0 {
				settings.ConnectRetry = time.Duration(cr) * time.Second
			}
		}
		if localMap, ok := fields["local"].(map[string]any); ok {
			if v, ok := localMap["connect"].(string); ok {
				settings.Connection.Connect = v == "true"
			}
		}
		if remoteMap, ok := fields["remote"].(map[string]any); ok {
			if v, ok := remoteMap["accept"].(string); ok {
				settings.Connection.Accept = v == "true"
			}
		}
		if !settings.Connection.Connect && !settings.Connection.Accept {
			reactorLogger().Warn("connect and accept both false, using default", "peer", peerName)
			settings.Connection = ConnectionBoth
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
	if a.ReceiveHoldTime != b.ReceiveHoldTime ||
		a.SendHoldTime != b.SendHoldTime ||
		a.ConnectRetry != b.ConnectRetry ||
		a.GroupUpdates != b.GroupUpdates ||
		a.IgnoreFamilyMismatch != b.IgnoreFamilyMismatch ||
		a.DisableASN4 != b.DisableASN4 {
		return false
	}

	// Compare static routes count (deep comparison would be expensive).
	if len(a.StaticRoutes) != len(b.StaticRoutes) {
		return false
	}

	// Compare capabilities by wire encoding.
	// Reload is rare, capabilities are small (<20 bytes each, <10 per peer).
	if !capabilitiesEqual(a.Capabilities, b.Capabilities) {
		return false
	}

	return true
}

// capabilitiesEqual compares two capability slices by wire encoding.
// Capabilities are sorted by code, then serialized and compared byte-by-byte.
func capabilitiesEqual(a, b []capability.Capability) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}

	// Serialize each capability and compare.
	// Capabilities are small (typically <20 bytes each).
	encodeAll := func(caps []capability.Capability) []byte {
		// Sort by code for deterministic comparison.
		sorted := make([]capability.Capability, len(caps))
		copy(sorted, caps)
		slices.SortFunc(sorted, func(x, y capability.Capability) int {
			return int(x.Code()) - int(y.Code())
		})

		var total int
		for _, c := range sorted {
			total += c.Len()
		}
		buf := make([]byte, total)
		off := 0
		for _, c := range sorted {
			off += c.WriteTo(buf, off)
		}
		return buf[:off]
	}

	return bytes.Equal(encodeAll(a), encodeAll(b))
}

// TeardownPeer gracefully closes a peer session with NOTIFICATION.
// Sends Cease (6) with the specified subcode per RFC 4486.
// RFC 8203: shutdownMsg is included in the NOTIFICATION for subcodes 2/4.
func (a *reactorAPIAdapter) TeardownPeer(addr netip.Addr, subcode uint8, shutdownMsg string) error {
	a.r.mu.RLock()
	peer, exists := a.r.findPeerByAddr(addr)
	a.r.mu.RUnlock()

	if !exists {
		return ErrPeerNotFound
	}

	// Signal teardown with subcode - peer will send NOTIFICATION and close.
	// If session exists, teardown happens immediately.
	// If not connected, teardown is queued to maintain operation order.
	return peer.Teardown(subcode, shutdownMsg)
}

// PausePeer pauses reading from a specific peer's session.
func (a *reactorAPIAdapter) PausePeer(addr netip.Addr) error {
	return a.r.PausePeer(addr)
}

// ResumePeer resumes reading from a specific peer's session.
func (a *reactorAPIAdapter) ResumePeer(addr netip.Addr) error {
	return a.r.ResumePeer(addr)
}

// FlushForwardPool blocks until all forward pool workers have drained their queued items.
// Used by plugins to ensure route delivery before proceeding with dependent operations.
func (a *reactorAPIAdapter) FlushForwardPool(ctx context.Context) error {
	return a.r.fwdPool.Barrier(ctx)
}

// FlushForwardPoolPeer blocks until the forward pool worker for a specific peer
// address has drained its queued items. Returns nil immediately if no worker exists.
func (a *reactorAPIAdapter) FlushForwardPoolPeer(ctx context.Context, addr string) error {
	parsed, err := netip.ParseAddr(addr)
	if err != nil {
		return fmt.Errorf("invalid peer address %q: %w", addr, err)
	}
	return a.r.fwdPool.BarrierPeer(ctx, parsed)
}

// RemovePeer removes a peer by address.
func (a *reactorAPIAdapter) RemovePeer(addr netip.Addr) error {
	return a.r.RemovePeer(addr)
}

// AddDynamicPeer adds a peer from a YANG-parsed config tree.
func (a *reactorAPIAdapter) AddDynamicPeer(addr netip.Addr, tree map[string]any) error {
	return a.r.AddDynamicPeer(addr, tree)
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
			ReceiveCustom:       maps.Clone(b.ReceiveCustom),
			SendUpdate:          b.SendUpdate,
			SendRefresh:         b.SendRefresh,
			SendCustom:          maps.Clone(b.SendCustom),
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

	// Fast path: exact match by parsed AddrPort key.
	if key := parsePeerAddrToKey(selector); key.IsValid() {
		if peer, ok := a.r.peers[key]; ok {
			return []*Peer{peer}
		}
	}

	// Try selector as peer name or bare IP (peers are selectable by either).
	for _, peer := range a.r.peers {
		if peer.settings.Name == selector || peer.settings.Address.String() == selector {
			return []*Peer{peer}
		}
	}

	// Try selector as ASN: "as<N>" (case-insensitive) matches all peers with that PeerAS.
	if len(selector) > 2 && (selector[0] == 'a' || selector[0] == 'A') && (selector[1] == 's' || selector[1] == 'S') {
		if asn, err := strconv.ParseUint(selector[2:], 10, 32); err == nil {
			var peers []*Peer
			for _, peer := range a.r.peers {
				if uint64(peer.settings.PeerAS) == asn {
					peers = append(peers, peer)
				}
			}
			return peers
		}
	}

	// Glob pattern match against the IP portion of each key.
	var peers []*Peer
	for addrPort, peer := range a.r.peers {
		if ipGlobMatch(selector, addrPort.Addr().String()) {
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
