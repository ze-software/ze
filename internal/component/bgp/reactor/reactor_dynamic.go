// Design: docs/architecture/core-design.md — dynamic peer groups for IXP route servers
// RFC: rfc/short/rfc4271.md — Section 4.2, the peer's AS arrives in its OPEN
// Related: reactor_connection.go — TCP accept integration point
// Related: reactor_peers.go — peer add/remove/lookup

package reactor

import (
	"maps"
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

	// Settings is the PeerSettings every member of this group is built from.
	//
	// It is the ONLY statement of what a member's session looks like. LocalAS,
	// RouterID and RSClient were fields here as well until 2026-08-13, each one
	// a second copy of a value Settings already held, and a second copy is a
	// value that can disagree with the first.
	Settings *PeerSettings

	// Runtime state.
	ActivePeers atomic.Int32
}

// containsAddr reports whether addr falls within any of the group's ranges.
func (d *DynamicGroupConfig) containsAddr(addr netip.Addr) bool {
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
		r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: key}, 4096)
	}
	if r.rmetrics != nil {
		r.rmetrics.peersConfigured.Set(float64(len(r.peers)))
		r.rmetrics.peersAddedTotal.Inc()
	}

	// The index is keyed on the peer ADDRESS and this peer has one, so it enters
	// it like any other peer: buildDynamicPeerSettings copies the whole
	// template, ProcessBindings included, so its group's attach blocks are
	// already on it. Without this republish delivery finds no edges for it and
	// feeds it to nobody, which is the fail-closed guard reading as a dead
	// session (delivery_graph.go).
	if r.deliveryPublished {
		r.publishDeliveryGraphLocked()
	}

	if r.running {
		peer.StartWithContext(r.ctx)
	}

	return peer, nil
}

// buildDynamicPeerSettings constructs PeerSettings for a dynamic peer from the
// group template.
//
// INHERITANCE IS THE DEFAULT AND DIVERGENCE IS THE EXCEPTION. The whole template
// is copied, then only the fields below are written. A field-by-field copy stood
// here until 2026-08-13 and it dropped every setting nobody remembered to add to
// it: the group's address families, its ADD-PATH require/refuse sets, its filter
// chains, its MD5 password. Each of those is a silent loss on an established
// session, and none of them fails a build.
//
// TestDynamicPeerInheritsEveryPeerSettingsField (reactor_dynamic_inherit_test.go)
// walks PeerSettings by reflection and refuses a field that is neither inherited
// nor named in its divergence table, so the next field added is inherited or the
// test says why it is not.
func (r *Reactor) buildDynamicPeerSettings(dg *DynamicGroupConfig, remoteAddr netip.Addr) (*PeerSettings, error) {
	tmpl := dg.Settings
	settings := *tmpl
	ps := &settings

	// The divergence set. Every other field arrived above, by inheritance.
	var tb textbuf.Buffer
	ps.Name = tb.Str("dyn-").Addr(remoteAddr).String()
	ps.GroupName = dg.GroupName
	ps.Address = remoteAddr
	// The peer map is keyed on (address, port) and a dynamic peer's remote port
	// is the ephemeral source port of the connection it arrived on, which names
	// no configuration. The canonical port keys every dynamic peer; the
	// template's own Port is what the group LISTENS on (peerListenPort,
	// reactor_peers.go).
	ps.Port = DefaultBGPPort
	ps.PeerAS = 0                     // RFC 4271 Section 4.2: learned from the peer's OPEN
	ps.Connection = ConnectionPassive // `ip dynamic` requires `connect false`
	ps.IsDynamic = true

	// Every prefix map is CLONED, never aliased. Dynamic peers are built one per
	// accepted connection from one template, so a shared map would let a later
	// write on one peer change enforcement for its siblings.
	ps.PrefixMaximum = maps.Clone(tmpl.PrefixMaximum)
	ps.PrefixWarning = maps.Clone(tmpl.PrefixWarning)
	ps.PrefixTeardown = maps.Clone(tmpl.PrefixTeardown)
	ps.PrefixIdleTimeout = maps.Clone(tmpl.PrefixIdleTimeout)
	ps.PrefixReconnect = maps.Clone(tmpl.PrefixReconnect)
	ps.PrefixCount = maps.Clone(tmpl.PrefixCount)
	ps.PrefixUpdated = maps.Clone(tmpl.PrefixUpdated)

	// The raw template tree still owns `connection > ttl`, because a group built
	// by a caller that never went through the config parser (a test, the API)
	// carries the block and not the parsed values.
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
	clearPrefixStale(settings.Address.String())
	if peer.health != nil {
		peer.health.stop()
	}
	delete(r.peers, key)

	if r.fwdWeights != nil {
		r.fwdWeights.RemovePeer(peer.peerAddrLabel())
	}
	if r.fwdPool != nil {
		r.fwdPool.unregisterOutgoingPool(fwdKey{peerAddr: key})
		r.fwdPool.removeSourceStats(settings.Address)
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

	// Republish without this peer's edges, for the reason doRemovePeer does.
	if r.deliveryPublished {
		r.publishDeliveryGraphLocked()
	}
}

// dynamicTemplateChanged reports whether a dynamic peer at addr would be built with
// different settings from next than it was built with from old.
//
// It compares what buildDynamicPeerSettings PRODUCES for that one address rather
// than the two group configs, because that is what a running dynamic peer was built
// from: a wider range or a raised max-peers changes the group and changes nothing
// the peer holds, and tearing a session down for either would be the reload bounce
// this comparison exists to avoid.
//
// The running peer's own settings are NOT the comparison. resolveDynamicPeerSettings
// rewrites PeerAS from the OPEN and resolves $remote_as / $remote_ip in the filter
// chains at establishment, so a running dynamic peer differs from every template,
// including the one it was built from.
//
// Fail closed: an absent old group or a template that no longer builds counts as
// changed, so the peer restarts and the next connection is accepted under settings
// that were actually derived from the new configuration.
func (r *Reactor) dynamicTemplateChanged(old, next *DynamicGroupConfig, addr netip.Addr) bool {
	if old == nil || next == nil {
		return true
	}
	oldSettings, oldErr := r.buildDynamicPeerSettings(old, addr)
	if oldErr != nil {
		return true
	}
	nextSettings, nextErr := r.buildDynamicPeerSettings(next, addr)
	if nextErr != nil {
		return true
	}
	return !peerSettingsEqual(oldSettings, nextSettings)
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

	// A stop that has begun must not gain a peer. Reactor.stop closes the
	// listeners under this same lock (reactor.go), so the only connection that
	// still reaches here is one a Listener had already accepted at that
	// instant, and its handler goroutine is holding it. Building a peer for it
	// would put a session in OpenSent that no shutdownNotify will ever cover --
	// its snapshot was taken before the peer existed -- and the cancel would
	// then close that socket with nothing on the wire to say why, which is the
	// RFC 4271 Section 8.2.2 ManualStop miss the stop exists to prevent.
	// The caller closes the connection (reactor_connection.go).
	if r.stopping {
		return nil
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
//
// Called during config load and reload. It is the ONE place that reconciles the
// dynamic peer population against configuration, because a dynamic peer's key is
// the AddrPort it connected from and no configuration ever names it: the peer diff
// in reconcilePeersJournaled (reactor_api.go) skips every dynamic peer and relies
// on this function instead.
//
// On reload it tears down a dynamic peer whose group was removed, whose address is
// no longer in any of that group's ranges, or whose group template no longer builds
// the settings the running session was accepted under. Every other dynamic peer is
// left running, session included.
func (r *Reactor) SetDynamicGroups(groups []*DynamicGroupConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	old := r.dynamicGroups

	// Collect peers to remove BEFORE replacing the groups slice.
	// removeDynamicPeer decrements ActivePeers on the current dynamicGroups;
	// we must do this while the old groups are still installed.
	if len(old) > 0 {
		newByName := make(map[string]*DynamicGroupConfig, len(groups))
		for _, g := range groups {
			newByName[g.GroupName] = g
		}
		oldByName := make(map[string]*DynamicGroupConfig, len(old))
		for _, g := range old {
			oldByName[g.GroupName] = g
		}

		var toRemove []*Peer
		for _, peer := range r.peers {
			settings := peer.Settings()
			if !settings.IsDynamic {
				continue
			}
			next, kept := newByName[settings.GroupName]
			if !kept {
				toRemove = append(toRemove, peer)
				continue
			}
			if !next.containsAddr(settings.Address) {
				toRemove = append(toRemove, peer)
				continue
			}
			if r.dynamicTemplateChanged(oldByName[settings.GroupName], next, settings.Address) {
				reactorLogger().Info("dynamic peer restart required",
					"peer", settings.Address, "group", settings.GroupName, "changed", "group template")
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

	// Open a listener for a group that has none. On the startup path this does
	// nothing: the reactor is not running yet, and startMultiListeners
	// (reactor.go) opens every listener once it is. On a reload it is what makes
	// a group added to a running daemon reachable. Nothing else opens a socket
	// for a group.
	//
	// A listener no claimant needs is left alone here. Closing an accepting
	// socket is the peer-removal path's decision (reactor_peers.go), and a group
	// removal that shared the socket with a peer must not take it down.
	if !r.running || r.stopping {
		return
	}
	for _, dg := range groups {
		s := dg.Settings
		if s == nil || !s.LocalAddress.IsValid() || !s.Connection.IsPassive() {
			continue
		}
		if err := r.startListenerForAddressPort(s.LocalAddress, r.peerListenPort(s)); err != nil {
			reactorLogger().Error("dynamic group listener failed",
				"group", dg.GroupName, "address", s.LocalAddress, "error", err)
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

	// Address and LocalAS are set at construction and never mutated, which is the contract
	// on Peer.Settings (peer.go), so they are read without the lock.
	remoteIP := p.settings.Address.String()
	localAS := p.settings.LocalAS

	// The filter pair is NOT in that population. ImportFilters and ExportFilters are two of
	// the three hot-swappable fields, and applyHotSwappableSettings (peer_settings_apply.go)
	// writes both from the config-reload goroutine, under p.mu, while this session runs. So
	// both are read under p.mu here. The establishment goroutine is not their only writer,
	// and a slice header is two words: an unlocked read can take the new pointer with the
	// old length.
	//
	// The dyn* template capture below stays outside the lock. Those two fields are touched
	// by this function alone, a peer has one session at a time, so resolveDynamicPeerSettings
	// is never concurrent with itself. resolveFilterVars allocates and runs outside the lock
	// for the same reason.
	p.mu.RLock()
	importFilters := p.settings.ImportFilters
	exportFilters := p.settings.ExportFilters
	p.mu.RUnlock()

	// Resolve from the original unresolved templates (set on first call, preserved across reconnections).
	if p.dynImportFilters == nil && len(importFilters) > 0 {
		p.dynImportFilters = importFilters
	}
	if p.dynExportFilters == nil && len(exportFilters) > 0 {
		p.dynExportFilters = exportFilters
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
