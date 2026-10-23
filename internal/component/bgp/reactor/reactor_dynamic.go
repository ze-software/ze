// Design: docs/architecture/core-design.md — dynamic peer groups for IXP route servers
// RFC: rfc/short/rfc4271.md — Section 4.2, the peer's AS arrives in its OPEN
// Related: reactor_connection.go — TCP accept integration point
// Related: reactor_peers.go — peer add/remove/lookup

package reactor

import (
	"errors"
	"maps"
	"net/netip"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
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

	// The plugin startup barrier, not running: the listener that accepted this
	// connection opens before plugin startup ends, and a member started then
	// can reach Established before the filter plugins its import chain names
	// are connected. Held here, the peer is started by the StartPeers copy, and
	// its inbound socket waits in the peer's inbound slot (acceptOrReject). A
	// refused barrier never lifts, so the held member is named in the log.
	if r.peersStarted {
		peer.StartWithContext(r.ctx)
	} else {
		r.logPeerHeldByRefusalLocked(peer)
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
	// no configuration. The canonical port keys every dynamic peer. The
	// template's LocalPort is inherited untouched: it is what the group LISTENS
	// on (peerListenPort, reactor_peers.go), so the member reports the same
	// socket the group accepted it on.
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
// Must be called with r.mu held (Lock). It returns the stop that sends the
// peer's live session, if any, Cease with subcode, and the caller MUST run it
// after releasing r.mu (peerStop).
func (r *Reactor) removeDynamicPeer(peer *Peer, subcode uint8) peerStop {
	settings := peer.Settings()
	key := settings.PeerKey()
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
	notifyFiltersPeerRemoved(settings.Address, settings.Name, settings.GroupName)

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
	return peerStop{peer: peer, subcode: subcode}
}

// dynamicGroupsLoad is what one configuration load says about the dynamic
// groups. A load that read them carries the set, an empty one included, and the
// journaled dynamic step reconciles the population against it. A load that did
// not read them (the tree route, which has no ReloadFunc) is
// dynamicGroupsNotRead, and the step then leaves the group set and every dynamic
// peer as they are.
//
// The zero value is dynamicGroupsNotRead on purpose, and that is a guard: it is
// the one answer that changes nothing, so a caller that forgets to say what it
// read cannot tear down a single dynamic peer. read, not a nil groups slice,
// carries the difference, because an empty set the ReloadFunc returned is how
// the config says "every group is gone".
type dynamicGroupsLoad struct {
	groups []*DynamicGroupConfig
	read   bool
}

// dynamicGroupsNotRead is the answer of a load that read no dynamic group.
var dynamicGroupsNotRead = dynamicGroupsLoad{}

// dynamicGroupsReadAs is the answer of a load that read groups, empty included.
func dynamicGroupsReadAs(groups []*DynamicGroupConfig) dynamicGroupsLoad {
	return dynamicGroupsLoad{groups: groups, read: true}
}

// dynamicSwap is one dynamic peer the step keeps on its running session: next
// is the next template built for the peer's address, with its filter chains
// still unresolved, and apply the copier peerSettingsSwapPlan chose.
type dynamicSwap struct {
	peer  *Peer
	next  *PeerSettings
	apply settingsCopier
}

// reconcileDynamicGroupsJournaled is the reload's dynamic step, the first one
// after the globals in reconcilePeersJournaled (reactor_api.go). A dynamic
// peer's key is the AddrPort it connected from and no configuration names it,
// so the peer diff skips every dynamic peer and this step reconciles them
// instead.
//
// Under r.mu it only plans (planDynamicGroupsLocked) and installs the new group
// set. The stops and the swaps run after r.mu is released: a stop writes a
// NOTIFICATION, and a swap is applyHotSwappableSettings, which takes r.mu itself
// and delivers to plugins, and sync.RWMutex is not reentrant. A swap goes
// through the configured peer's copier and ordered delivery, so a dynamic peer
// gains a receive-only binding, a BFD opt-in or a filter chain the way a
// configured peer does (owner, OQ-1 and OQ-2, option A).
//
// The undo restores the previous group set, recounts, and undoes each swap
// from the snapshot taken before it, the dyn* template pair included
// (hotSwappableSnapshot). A dynamic peer the step removed is not rebuilt:
// nothing names it, and its next connection is accepted under the restored
// group.
func (a *reactorAPIAdapter) reconcileDynamicGroupsJournaled(groups []*DynamicGroupConfig, label string, j configJournal) error {
	r := a.r
	var previous []*DynamicGroupConfig
	var undone []dynamicSwap
	restore := func() error {
		r.mu.Lock()
		r.installDynamicGroupsLocked(previous)
		r.mu.Unlock()
		var errs []error
		for _, swap := range slices.Backward(undone) {
			if err := swap.peer.restoreHotSwappableSettings(swap.next, swap.apply); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
	return j.Record(
		func() error {
			r.mu.Lock()
			previous = r.dynamicGroups
			stops, swaps := r.planDynamicGroupsLocked(groups, label)
			r.installDynamicGroupsLocked(groups)
			r.mu.Unlock()

			for _, stop := range stops {
				stop.run()
			}
			for _, swap := range swaps {
				before := swap.peer.hotSwappableSnapshot(swap.apply)
				undone = append(undone, dynamicSwap{peer: swap.peer, next: before, apply: swap.apply})
				if err := swap.peer.applyHotSwappableSettings(swap.next, swap.apply); err != nil {
					// Record does not keep the inverse of a failed apply.
					return errors.Join(err, restore())
				}
			}
			return nil
		},
		restore,
	)
}

// planDynamicGroupsLocked decides, for every dynamic peer, what the next group
// set does to it: remove it (its group is gone, or no range covers it any more:
// Cease subcode 3), restart it (peerSettingsSwapPlan names a field its session
// cannot take: Cease subcode 6), swap it in place, or leave it alone. It removes
// the peers it must from r.peers and returns their stops and the swaps, for the
// caller to run with r.mu released.
//
// The plan compares what buildDynamicPeerSettings PRODUCES for the peer's
// address from the old and the next template, never the running peer's own
// settings: resolveDynamicPeerSettings rewrote PeerAS and the filter chains at
// establishment, so the running peer differs from every template, including the
// one it was built from. A wider range or a raised max-peers changes the group
// and nothing the peer holds, so neither reaches the plan.
//
// The caller MUST hold r.mu for writing, and the old groups MUST still be
// installed: removeDynamicPeer decrements the counter of the group it finds.
func (r *Reactor) planDynamicGroupsLocked(groups []*DynamicGroupConfig, label string) ([]peerStop, []dynamicSwap) {
	nextByName := make(map[string]*DynamicGroupConfig, len(groups))
	for _, g := range groups {
		nextByName[g.GroupName] = g
	}
	oldByName := make(map[string]*DynamicGroupConfig, len(r.dynamicGroups))
	for _, g := range r.dynamicGroups {
		oldByName[g.GroupName] = g
	}

	var removed, restarted []*Peer
	var swaps []dynamicSwap
	for _, peer := range r.peers {
		settings := peer.Settings()
		if !settings.IsDynamic {
			continue
		}
		next, kept := nextByName[settings.GroupName]
		if !kept {
			removed = append(removed, peer)
			continue
		}
		if !next.containsAddr(settings.Address) {
			removed = append(removed, peer)
			continue
		}
		oldBuilt, nextBuilt, reason := r.dynamicTemplatePair(oldByName[settings.GroupName], next, settings.Address)
		if reason == "" && peerSettingsEqual(oldBuilt, nextBuilt) {
			continue
		}
		var apply settingsCopier
		if reason == "" {
			apply, reason = peerSettingsSwapPlan(oldBuilt, nextBuilt, peer.currentSession())
		}
		if reason != "" {
			reactorLogger().Info("dynamic peer restart required", "phase", label,
				"peer", settings.Address, "group", settings.GroupName, "changed", reason)
			restarted = append(restarted, peer)
			continue
		}
		swaps = append(swaps, dynamicSwap{peer: peer, next: nextBuilt, apply: apply})
	}

	// RFC 4486 Section 4: a peer the config no longer accepts is told "Peer
	// De-configured", a restarted one "Other Configuration Change".
	stops := make([]peerStop, 0, len(removed)+len(restarted))
	for _, peer := range removed {
		stops = append(stops, r.removeDynamicPeer(peer, message.NotifyCeasePeerDeconfigured))
	}
	for _, peer := range restarted {
		stops = append(stops, r.removeDynamicPeer(peer, message.NotifyCeaseOtherConfigChange))
	}
	return stops, swaps
}

// dynamicTemplateUnbuildable is the restart reason for a dynamic peer whose old
// group is gone or whose template no longer builds.
const dynamicTemplateUnbuildable = "group template"

// dynamicTemplatePair builds the settings a dynamic peer at addr gets from the
// old and from the next template. It fails closed: an absent old group or a
// template that no longer builds is a restart reason, so the next connection is
// accepted under settings actually derived from the new configuration.
func (r *Reactor) dynamicTemplatePair(old, next *DynamicGroupConfig, addr netip.Addr) (*PeerSettings, *PeerSettings, string) {
	if old == nil {
		return nil, nil, dynamicTemplateUnbuildable
	}
	oldBuilt, err := r.buildDynamicPeerSettings(old, addr)
	if err != nil {
		return nil, nil, dynamicTemplateUnbuildable
	}
	nextBuilt, err := r.buildDynamicPeerSettings(next, addr)
	if err != nil {
		return nil, nil, dynamicTemplateUnbuildable
	}
	return oldBuilt, nextBuilt, ""
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

// SetDynamicGroups installs the reactor's dynamic group configs at load.
//
// It reconciles nothing: the boot path calls it once, on a reactor with no
// dynamic peer yet. A reload reaches the dynamic population through the
// journaled dynamic step instead (reconcileDynamicGroupsJournaled), which
// shares installDynamicGroupsLocked with this function.
func (r *Reactor) SetDynamicGroups(groups []*DynamicGroupConfig) {
	r.mu.Lock()
	r.installDynamicGroupsLocked(groups)
	r.mu.Unlock()
}

// installDynamicGroupsLocked makes groups the reactor's dynamic group set,
// recounts each group's ActivePeers from r.peers, and opens a listener for a
// group that has none. The caller MUST hold r.mu for writing.
//
// The recount starts every group at zero, so it is right for a fresh group set
// and for a previous one a journal undo puts back, whose counters the step's
// removals already moved.
func (r *Reactor) installDynamicGroupsLocked(groups []*DynamicGroupConfig) {
	r.dynamicGroups = groups
	for _, g := range groups {
		g.ActivePeers.Store(0)
	}
	for _, peer := range r.peers {
		settings := peer.Settings()
		if !settings.IsDynamic {
			continue
		}
		for _, g := range groups {
			if g.GroupName == settings.GroupName {
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
// config variables ($remote_as, $remote_ip) in the filter chains. Called in the
// Established callback before sendInitialRoutes. Idempotent: it resolves from
// the unresolved template pair (dynImportPolicy, dynExportPolicy) that NewPeer
// captured and a reload's dynamic swap replaces.
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

	// The template pair has two writers: this function's reads, and a reload's
	// dynamic swap, which writes it from the config-reload goroutine
	// (applyHotSwappableSettings). Both touch it under p.mu only. resolveFilterVars
	// allocates, so it runs with the lock released, and the publish then checks
	// that the pair it resolved is still the installed one. A swap that landed in
	// between bumped dynGeneration, and the chains resolved from the replaced pair
	// are dropped and resolved again from the new one: a resolution never
	// publishes chains built from a template a swap already replaced.
	//
	// The loop ends on the first pass no swap interleaves with. Reloads are
	// serialized, and each retry needs a whole reload's dynamic step to write the
	// pair inside one resolveFilterVars call, so it is bounded by the reloads an
	// operator issues during one establishment.
	for {
		read := p.readDynamicTemplate()
		newImport := read.importTemplate.WithFilters(resolveFilterVars(read.importTemplate.Filters(), localAS, remoteAS, remoteIP))
		newExport := read.exportTemplate.WithFilters(resolveFilterVars(read.exportTemplate.Filters(), localAS, remoteAS, remoteIP))
		if p.publishDynamicResolution(read, remoteAS, newImport, newExport) {
			break
		}
	}

	// refreshForwardFacts runs AFTER releasing p.mu because it re-acquires
	// p.mu.RLock (peer_forward_facts.go) and RWMutex is not reentrant.
	p.refreshForwardFacts()
}

// dynamicTemplateRead is one read of a dynamic peer's unresolved template
// pair, with the generation it was read at (Peer.readDynamicTemplate).
type dynamicTemplateRead struct {
	generation     uint64
	importTemplate filterapi.Policy
	exportTemplate filterapi.Policy
}

// readDynamicTemplate reads the template pair and its generation under p.mu.
// The caller resolves it with the lock released and hands the result to
// publishDynamicResolution with this read.
func (p *Peer) readDynamicTemplate() dynamicTemplateRead {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return dynamicTemplateRead{
		generation:     p.dynGeneration,
		importTemplate: p.dynImportPolicy,
		exportTemplate: p.dynExportPolicy,
	}
}

// publishDynamicResolution publishes the AS learned from the OPEN and the
// chains resolved from read, and reports whether it did. It publishes nothing
// and returns false when a swap replaced the template pair since read
// (writeDynamicTemplateLocked bumps dynGeneration): those chains were built
// from a template the operator no longer has, and the caller resolves again.
//
// The three fields are written under p.mu so cross-goroutine readers (the
// routerid_unique.go identifier claim, PeerInfo builders, API/plugin Settings
// snapshots, filter getters) observe consistent values via the
// PeerAS()/ImportPolicy()/ExportPolicy() accessors, never a torn policy value
// or an unsynchronized PeerAS.
func (p *Peer) publishDynamicResolution(read dynamicTemplateRead, remoteAS uint32, newImport, newExport filterapi.Policy) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dynGeneration != read.generation {
		return false
	}
	p.settings.PeerAS = remoteAS
	p.settings.ImportPolicy = newImport
	p.settings.ExportPolicy = newExport
	return true
}

// writeDynamicTemplateLocked is the dynamic half of a swap's write section
// (applyHotSwappableSettings): it installs next's unresolved filter chains as
// the peer's template pair, bumps dynGeneration so a resolution racing it starts
// again (resolveDynamicPeerSettings), and, when the peer has learned its AS,
// resolves the chains the copier just wrote against that AS.
//
// The AS is read here, under the write lock, and never at plan time: a peer
// that established between the plan and this write would otherwise keep
// unresolved chains. A peer that has not learned its AS keeps them unresolved,
// the state createDynamicPeer leaves, and its establishment resolves them from
// the new pair. This runs on the reload goroutine, not a hot path, so the
// allocation under p.mu is accepted.
//
// The caller MUST hold p.mu for writing, and MUST have run the copier onto
// p.settings first.
func (p *Peer) writeDynamicTemplateLocked(next *PeerSettings) {
	p.dynImportPolicy = next.ImportPolicy
	p.dynExportPolicy = next.ExportPolicy
	p.dynGeneration++
	remoteAS := p.settings.PeerAS
	if remoteAS == 0 {
		return
	}
	localAS := p.settings.LocalAS
	remoteIP := p.settings.Address.String()
	p.settings.ImportPolicy = next.ImportPolicy.WithFilters(resolveFilterVars(next.ImportPolicy.Filters(), localAS, remoteAS, remoteIP))
	p.settings.ExportPolicy = next.ExportPolicy.WithFilters(resolveFilterVars(next.ExportPolicy.Filters(), localAS, remoteAS, remoteIP))
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
		current, exists := r.findPeerByAddr(addr)
		if !exists || current != peer {
			r.mu.Unlock()
			return
		}
		if current.State() == PeerStateEstablished {
			r.mu.Unlock()
			return
		}
		stop := r.removeDynamicPeer(current, message.NotifyCeasePeerDeconfigured)
		r.mu.Unlock()
		stop.run()
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
