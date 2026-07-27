// Design: docs/architecture/core-design.md — dynamic peer groups for IXP route servers
// Related: reactor_connection.go — TCP accept integration point
// Related: reactor_peers.go — peer add/remove/lookup

package reactor

import (
	"net/netip"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// DynamicGroupConfig holds the configuration for a dynamic peer group.
// Created from a group with `ip dynamic` + `range` during config resolution.
type DynamicGroupConfig struct {
	GroupName string
	Ranges    []netip.Prefix
	MaxPeers  uint32
	Template  map[string]any

	// LocalAS and RouterID for building PeerSettings without parsePeerFromTree.
	LocalAS  uint32
	RouterID uint32

	// RSClient is true when the group has session/rs-client true.
	RSClient bool

	// Resolved peer settings fields from the group template.
	Settings *PeerSettings

	// Runtime state.
	ActivePeers atomic.Int32
}

// ContainsAddr reports whether addr falls within any of the group's ranges.
func (d *DynamicGroupConfig) ContainsAddr(addr netip.Addr) bool {
	for _, r := range d.Ranges {
		if r.Contains(addr) {
			return true
		}
	}
	return false
}

// findDynamicGroup returns the dynamic group config whose range contains addr
// using longest-prefix-match for overlapping ranges.
// Must be called with r.mu held (RLock or Lock).
func (r *Reactor) findDynamicGroup(addr netip.Addr) *DynamicGroupConfig {
	var best *DynamicGroupConfig
	bestBits := -1
	for _, dg := range r.dynamicGroups {
		for _, prefix := range dg.Ranges {
			if prefix.Contains(addr) && prefix.Bits() > bestBits {
				best = dg
				bestBits = prefix.Bits()
			}
		}
	}
	return best
}

// createDynamicPeer creates a new peer from a dynamic group template for the given
// remote address. Returns the created peer or an error if max-peers is exceeded.
// Must be called with r.mu held (Lock).
func (r *Reactor) createDynamicPeer(dg *DynamicGroupConfig, remoteAddr netip.Addr) (*Peer, error) {
	if dg.MaxPeers > 0 && dg.ActivePeers.Load() >= int32(dg.MaxPeers) {
		return nil, ErrDynamicMaxPeers
	}

	ps, settingsErr := r.buildDynamicPeerSettings(dg, remoteAddr)
	if settingsErr != nil {
		return nil, settingsErr
	}
	peer := NewPeer(ps)
	peer.SetClock(r.clock)
	peer.SetDialer(r.dialer)
	peer.SetReactor(r)
	peer.messageCallback = r.notifyMessageReceiver

	key := ps.PeerKey()
	r.peers[key] = peer
	dg.ActivePeers.Add(1)

	if r.fwdWeights != nil {
		r.fwdWeights.AddPeer(peer.peerAddrLabel(), totalPrefixMax(ps.PrefixMaximum), len(ps.PrefixMaximum))
	}
	if r.fwdPool != nil {
		r.fwdPool.RegisterOutgoingPool(fwdKey{peerAddr: key}, 4096)
	}
	if r.rmetrics != nil {
		r.rmetrics.peersConfigured.Set(float64(len(r.peers)))
		r.rmetrics.peersAddedTotal.Inc()
	}

	if r.running {
		peer.StartWithContext(r.ctx)
	}

	return peer, nil
}

// buildDynamicPeerSettings constructs PeerSettings for a dynamic peer from
// the group template. PeerAS is left as 0 (filled from OPEN message later).
func (r *Reactor) buildDynamicPeerSettings(dg *DynamicGroupConfig, remoteAddr netip.Addr) (*PeerSettings, error) {
	tmpl := dg.Settings
	ps := &PeerSettings{
		Name:            "dyn-" + remoteAddr.String(),
		GroupName:       dg.GroupName,
		Address:         remoteAddr,
		LocalAddress:    tmpl.LocalAddress,
		Port:            DefaultBGPPort,
		LocalAS:         tmpl.LocalAS,
		GlobalLocalAS:   tmpl.GlobalLocalAS,
		PeerAS:          0, // Learned from OPEN
		RouterID:        tmpl.RouterID,
		ReceiveHoldTime: tmpl.ReceiveHoldTime,
		SendHoldTime:    tmpl.SendHoldTime,
		KeepaliveTime:   tmpl.KeepaliveTime,
		ConnectRetry:    tmpl.ConnectRetry,
		Connection:      ConnectionPassive, // Dynamic peers are always passive
		MD5Key:          tmpl.MD5Key,
		MD5IP:           tmpl.MD5IP,
		// OutTTL/MinTTL are not carried on dg.Settings (the resolved
		// template PeerSettings never parses ttl); they are derived
		// below from the raw dg.Template connection > ttl block.
		BFD:              tmpl.BFD,
		GroupUpdates:     tmpl.GroupUpdates,
		RSFastPath:       tmpl.RSFastPath,
		RSClient:         dg.RSClient,
		IsDynamic:        true,
		DisableASN4:      tmpl.DisableASN4,
		Capabilities:     tmpl.Capabilities,
		RequiredFamilies: tmpl.RequiredFamilies,
		IgnoreFamilies:   tmpl.IgnoreFamilies,
		StaticRoutes:     tmpl.StaticRoutes,
		PrefixMaximum:    tmpl.PrefixMaximum,
		PrefixWarning:    tmpl.PrefixWarning,
		PrefixTeardown:   tmpl.PrefixTeardown,
		ProcessBindings:  tmpl.ProcessBindings,
		ImportFilters:    tmpl.ImportFilters,
		ExportFilters:    tmpl.ExportFilters,
		LoopAllowOwnAS:   tmpl.LoopAllowOwnAS,
		NextHopMode:      tmpl.NextHopMode,
		NextHopAddress:   tmpl.NextHopAddress,
		SendCommunity:    tmpl.SendCommunity,

		IgnoreFamilyMismatch:   tmpl.IgnoreFamilyMismatch,
		RequiredCapabilities:   tmpl.RequiredCapabilities,
		RefusedCapabilities:    tmpl.RefusedCapabilities,
		RouteReflectorClient:   tmpl.RouteReflectorClient,
		ClusterID:              tmpl.ClusterID,
		ASOverride:             tmpl.ASOverride,
		LocalASNoPrepend:       tmpl.LocalASNoPrepend,
		LocalASReplaceAS:       tmpl.LocalASReplaceAS,
		DefaultOriginate:       tmpl.DefaultOriginate,
		DefaultOriginateFilter: tmpl.DefaultOriginateFilter,
		RawCapabilityConfig:    tmpl.RawCapabilityConfig,
		CapabilityConfigJSON:   tmpl.CapabilityConfigJSON,
	}
	if dg.Template != nil {
		if connMap, ok := dg.Template["connection"].(map[string]any); ok {
			if ttlMap, ok := connMap["ttl"].(map[string]any); ok {
				outTTL, minTTL, err := parseTTLSettings(dg.GroupName, ttlMap)
				if err != nil {
					return nil, err
				}
				ps.OutTTL = outTTL
				ps.MinTTL = minTTL
			}
		}
	}
	return ps, nil
}

// removeDynamicPeer removes a dynamic peer and decrements the group counter.
// Must be called with r.mu held (Lock).
func (r *Reactor) removeDynamicPeer(peer *Peer) {
	settings := peer.Settings()
	key := settings.PeerKey()

	peer.Stop()
	// Same synchronous release as removePeer and the reload-remove path: Stop
	// only cancels a context, so a dynamic peer removed and recreated (the
	// remove/recreate path this function serves) would otherwise race its own
	// stale claim and refuse the new session with Bad BGP Identifier.
	peer.releaseRouterIDClaim()
	ClearPrefixStale(settings.Address.String())
	if peer.health != nil {
		peer.health.stop()
	}
	delete(r.peers, key)

	if r.fwdWeights != nil {
		r.fwdWeights.RemovePeer(peer.peerAddrLabel())
	}
	if r.fwdPool != nil {
		r.fwdPool.UnregisterOutgoingPool(fwdKey{peerAddr: key})
		r.fwdPool.RemoveSourceStats(settings.Address)
	}
	if r.rmetrics != nil {
		r.rmetrics.peersConfigured.Set(float64(len(r.peers)))
		r.rmetrics.peersRemovedTotal.Inc()
	}

	for _, dg := range r.dynamicGroups {
		if dg.GroupName == settings.GroupName {
			dg.ActivePeers.Add(-1)
			break
		}
	}
}

// tryCreateDynamicPeer checks dynamic groups for the given address and creates a
// peer if a matching group is found. Returns nil if no group matches or max-peers
// is exceeded. Thread-safe: acquires r.mu internally.
func (r *Reactor) tryCreateDynamicPeer(addr netip.Addr) *Peer {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Re-check: another goroutine may have created this peer concurrently.
	if peer, exists := r.findPeerByAddr(addr); exists {
		return peer
	}

	dg := r.findDynamicGroup(addr)
	if dg == nil {
		return nil
	}

	peer, err := r.createDynamicPeer(dg, addr)
	if err != nil {
		reactorLogger().Debug("dynamic peer creation failed", "addr", addr, "group", dg.GroupName, "error", err)
		return nil
	}

	reactorLogger().Info("dynamic peer created", "addr", addr, "group", dg.GroupName)
	return peer
}

// SetDynamicGroups replaces the reactor's dynamic group configs.
// Called during config load and reload. On reload, tears down dynamic peers
// whose group was removed or whose address is no longer in any range.
func (r *Reactor) SetDynamicGroups(groups []*DynamicGroupConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	old := r.dynamicGroups

	// Collect peers to remove BEFORE replacing the groups slice.
	// removeDynamicPeer decrements ActivePeers on the current dynamicGroups;
	// we must do this while the old groups are still installed.
	if len(old) > 0 {
		newGroupNames := make(map[string]bool, len(groups))
		for _, g := range groups {
			newGroupNames[g.GroupName] = true
		}

		var toRemove []*Peer
		for _, peer := range r.peers {
			if !peer.Settings().IsDynamic {
				continue
			}
			groupName := peer.Settings().GroupName
			if !newGroupNames[groupName] {
				toRemove = append(toRemove, peer)
				continue
			}
			inRange := false
			for _, g := range groups {
				if g.GroupName == groupName && g.ContainsAddr(peer.Settings().Address) {
					inRange = true
					break
				}
			}
			if !inRange {
				toRemove = append(toRemove, peer)
			}
		}
		for _, peer := range toRemove {
			r.removeDynamicPeer(peer)
		}
	}

	// Now replace the groups. Surviving dynamic peers' counters were already
	// accounted for on the old groups; the new groups start at zero.
	// Transfer surviving peer counts to the new groups.
	r.dynamicGroups = groups
	for _, peer := range r.peers {
		if !peer.Settings().IsDynamic {
			continue
		}
		for _, g := range groups {
			if g.GroupName == peer.Settings().GroupName {
				g.ActivePeers.Add(1)
				break
			}
		}
	}
}

// resolveDynamicPeerSettings sets PeerAS from the OPEN message and resolves
// config variables ($remote_as, $remote_ip) in static routes and filter chains.
// Called in the Established callback before sendInitialRoutes. Idempotent:
// resolves from the original unresolved templates stored on peer creation.
func (p *Peer) resolveDynamicPeerSettings(session *Session) {
	session.mu.RLock()
	open := session.peerOpen
	session.mu.RUnlock()
	if open == nil {
		return
	}

	// RFC 6793: use 4-byte ASN if available, else 2-byte MyAS.
	remoteAS := uint32(open.MyAS)
	if open.ASN4 > 0 {
		remoteAS = open.ASN4
	}

	// These reads and the dyn* template capture run on the establishment goroutine, the
	// only writer of these fields, so they need no lock here. resolveDynamicPeerSettings
	// is never called concurrently with itself for one peer (a peer has one session at a
	// time). Compute the resolved filter slices before taking p.mu so resolveFilterVars
	// (which allocates) runs outside the lock.
	remoteIP := p.settings.Address.String()
	localAS := p.settings.LocalAS

	// Resolve from the original unresolved templates (set on first call, preserved across reconnections).
	if p.dynImportFilters == nil && len(p.settings.ImportFilters) > 0 {
		p.dynImportFilters = p.settings.ImportFilters
	}
	if p.dynExportFilters == nil && len(p.settings.ExportFilters) > 0 {
		p.dynExportFilters = p.settings.ExportFilters
	}
	var newImport, newExport []filterapi.FilterRef
	if p.dynImportFilters != nil {
		newImport = resolveFilterVars(p.dynImportFilters, localAS, remoteAS, remoteIP)
	}
	if p.dynExportFilters != nil {
		newExport = resolveFilterVars(p.dynExportFilters, localAS, remoteAS, remoteIP)
	}

	// Publish the three mutable settings fields under p.mu so cross-goroutine readers
	// (the routerid_unique.go identifier claim, PeerInfo builders, API/plugin Settings snapshots, filter
	// getters) observe consistent values via the PeerAS()/ImportFilters()/ExportFilters()
	// accessors — never a torn slice header or an unsynchronized PeerAS. refreshForwardFacts
	// is called AFTER releasing p.mu because it re-acquires p.mu.RLock (peer_forward_facts.go)
	// and RWMutex is not reentrant.
	p.mu.Lock()
	p.settings.PeerAS = remoteAS
	if newImport != nil {
		p.settings.ImportFilters = newImport
	}
	if newExport != nil {
		p.settings.ExportFilters = newExport
	}
	p.mu.Unlock()

	p.refreshForwardFacts()
}

// scheduleDynamicPeerCleanup starts a timer that removes a dynamic peer after
// the group's connect-retry timeout if no reconnection arrives. Called from
// notifyPeerClosed for dynamic peers.
func (r *Reactor) scheduleDynamicPeerCleanup(peer *Peer) {
	settings := peer.Settings()
	timeout := settings.ConnectRetry
	if timeout == 0 {
		timeout = DefaultConnectRetry
	}
	addr := settings.Address
	groupName := settings.GroupName

	r.clock.AfterFunc(timeout, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		current, exists := r.findPeerByAddr(addr)
		if !exists || current != peer {
			return
		}
		if current.State() == PeerStateEstablished {
			return
		}
		r.removeDynamicPeer(current)
		reactorLogger().Info("dynamic peer removed after idle timeout", "addr", addr, "group", groupName)
	})
}

// resolveFilterVars replaces $remote_as, $local_as, $remote_ip in filter names.
// Inline in reactor to avoid an import cycle with bgp/config.
func resolveFilterVars(filters []filterapi.FilterRef, localAS, remoteAS uint32, remoteIP string) []filterapi.FilterRef {
	if len(filters) == 0 {
		return filters
	}
	hasVar := false
	for _, f := range filters {
		if strings.ContainsRune(f.Name, '$') {
			hasVar = true
			break
		}
	}
	if !hasVar {
		return filters
	}
	las := textbuf.StringUint32(localAS)
	ras := textbuf.StringUint32(remoteAS)
	resolved := make([]filterapi.FilterRef, len(filters))
	for i, f := range filters {
		if !strings.ContainsRune(f.Name, '$') {
			resolved[i] = f
			continue
		}
		name := strings.ReplaceAll(f.Name, "$remote_as", ras)
		name = strings.ReplaceAll(name, "$local_as", las)
		name = strings.ReplaceAll(name, "$remote_ip", remoteIP)
		resolved[i] = filterapi.FilterRef{Name: name, Inactive: f.Inactive}
	}
	return resolved
}

// ErrDynamicMaxPeers is returned when a dynamic group has reached its max-peers limit.
var ErrDynamicMaxPeers = errDynamicMaxPeers{}

type errDynamicMaxPeers struct{}

func (errDynamicMaxPeers) Error() string { return "dynamic group max-peers exceeded" }
