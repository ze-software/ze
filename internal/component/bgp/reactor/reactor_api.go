// Design: docs/architecture/core-design.md — reactor API adapter for plugin integration
// Overview: reactor.go — Reactor struct, lifecycle, and connection management
// Related: reactor_wire.go — zero-allocation wire UPDATE builders
// Detail: reactor_api_batch.go — NLRI batch operations and wire attribute building
// Detail: reactor_api_forward.go — UPDATE forwarding, grouped sending, cache ops
package reactor

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"net/netip"
	"reflect"
	"slices"
	"time"

	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
		Address:         s.Address,
		LocalAddress:    s.LocalAddress,
		AddressStr:      peer.addrString,
		LocalAddressStr: peer.localAddrString,
		Name:            s.Name,
		GroupName:       s.GroupName,
		LocalAS:         s.LocalAS,
		PeerAS:          s.PeerAS,
		RouterID:        s.RouterID,
		Connect:         s.Connection.Connect,
		Accept:          s.Connection.Accept,
		State:           peer.State().PluginState(),
	}
	o.dispatcher.OnPeerStateChange(&peerInfo, rpc.SessionStateUp, "")
}

func (o *apiStateObserver) OnPeerClosed(peer *Peer, reason string) {
	if o.dispatcher == nil {
		return
	}
	s := peer.Settings()
	peerInfo := plugin.PeerInfo{
		Address:         s.Address,
		LocalAddress:    s.LocalAddress,
		AddressStr:      peer.addrString,
		LocalAddressStr: peer.localAddrString,
		Name:            s.Name,
		GroupName:       s.GroupName,
		LocalAS:         s.LocalAS,
		// PeerAS via the guarded accessor: unlike OnPeerEstablished (whose callback is always
		// drained on the read goroutine that also writes PeerAS), a from-Established teardown
		// can be drained on the hold-timer goroutine, racing a dynamic peer's reconnect
		// resolveDynamicPeerSettings write (reactor_dynamic.go). Other fields are immutable
		// post-construction (only PeerAS/ImportFilters/ExportFilters mutate) so stay direct.
		PeerAS:   peer.PeerAS(),
		RouterID: s.RouterID,
		Connect:  s.Connection.Connect,
		Accept:   s.Connection.Accept,
		State:    peer.State().PluginState(),
	}
	o.dispatcher.OnPeerStateChange(&peerInfo, rpc.SessionStateDown, reason)
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
		// PeerAS, the filter chains and the prefix dates via the guarded accessors: this
		// API snapshot runs on a command goroutine that can race a dynamic peer's
		// establishment write and a reload's hot swap. All other PeerSettings fields are
		// immutable after construction.
		peerAS := p.PeerAS()
		importFilters := p.ImportFilters()
		exportFilters := p.ExportFilters()
		prefixUpdated := p.OldestPrefixUpdated()
		stats := p.Stats()
		peerType := "external"
		if s.LocalAS == peerAS {
			peerType = "internal"
		}
		localPort, remotePort := p.tCPPorts()
		info := plugin.PeerInfo{
			Address:              s.Address,
			LocalAddress:         s.LocalAddress,
			AddressStr:           p.addrString,
			LocalAddressStr:      p.localAddrString,
			Name:                 s.Name,
			GroupName:            s.GroupName,
			LocalAS:              s.LocalAS,
			PeerAS:               peerAS,
			RouterID:             s.RouterID,
			ReceiveHoldTime:      s.ReceiveHoldTime,
			SendHoldTime:         s.SendHoldTime,
			KeepaliveTime:        s.KeepaliveTime,
			ConnectRetry:         s.ConnectRetry,
			Connect:              s.Connection.Connect,
			Accept:               s.Connection.Accept,
			State:                p.State().PluginState(),
			UpdatesReceived:      stats.UpdatesReceived,
			UpdatesSent:          stats.UpdatesSent,
			KeepalivesReceived:   stats.KeepalivesReceived,
			KeepalivesSent:       stats.KeepalivesSent,
			EORReceived:          stats.EORReceived,
			EORSent:              stats.EORSent,
			PrefixUpdated:        prefixUpdated,
			RouteReflectorClient: s.RouteReflectorClient,
			ClusterID:            s.ClusterID,
			NextHopMode:          s.NextHopMode,
			NextHopAddress:       s.NextHopAddress,
			ImportFilters:        filterapi.FilterRefStrings(importFilters),
			ExportFilters:        filterapi.FilterRefStrings(exportFilters),

			OpensReceived:         stats.OpensReceived,
			OpensSent:             stats.OpensSent,
			NotificationsReceived: stats.NotificationsReceived,
			NotificationsSent:     stats.NotificationsSent,
			RefreshReceived:       stats.RefreshReceived,
			RefreshSent:           stats.RefreshSent,

			ConnectionsEstablished: stats.ConnectionsEstablished,
			ConnectionsDropped:     stats.ConnectionsDropped,
			ConnectRetryCounter:    stats.ConnectRetryCounter,
			LastNotifCode:          stats.LastNotifCode,
			LastNotifSubcode:       stats.LastNotifSubcode,
			LastNotifRecv:          stats.LastNotifRecv,
			LastNotifTime:          stats.LastNotifTime,
			LastReadTime:           stats.LastReadTime,
			LastWriteTime:          stats.LastWriteTime,
			LastStateChange:        p.LastStateChange(),

			PeerType:                peerType,
			LocalPort:               localPort,
			RemotePort:              remotePort,
			MD5Enabled:              s.MD5Key != "",
			BFDEnabled:              s.BFD != nil,
			GTSMOutTTL:              s.OutTTL,
			GTSMMinTTL:              s.MinTTL,
			NegotiatedHoldTime:      time.Duration(p.NegotiatedHoldTime()) * time.Second,
			NegotiatedKeepaliveTime: time.Duration(p.NegotiatedKeepaliveTime()) * time.Second,
		}
		if estAt := p.EstablishedAt(); !estAt.IsZero() {
			info.Uptime = a.r.clock.Now().Sub(estAt)
		}
		if p.health != nil {
			info.FlapCount = p.health.FlapCount()
		}
		if neg := p.negotiated.Load(); neg != nil {
			info.NegotiatedFamilies = neg.Families()
			info.NegotiationComplete = true
			info.NegotiatedASN4 = neg.ASN4
			info.NegotiatedExtMsg = neg.ExtendedMessage
			info.NegotiatedRouteRefresh = neg.RouteRefresh
			info.NegotiatedEnhancedRR = neg.EnhancedRouteRefresh
			for _, f := range neg.Families() {
				if p.addPathFor(f) {
					if info.NegotiatedAddPath == nil {
						info.NegotiatedAddPath = make(map[string]string)
					}
					info.NegotiatedAddPath[f.String()] = addPathSendDirection
				}
			}
			if neg.GracefulRestart != nil {
				info.GracefulRestart = true
				info.GRRestartTime = neg.GracefulRestart.RestartTime
			}
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
func (a *reactorAPIAdapter) SoftClearPeer(sel *selector.Selector) ([]string, error) {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	peers := a.getMatchingPeersSel(sel)
	if len(peers) == 0 {
		return nil, ErrPeerNotFound
	}

	familySet := make(map[string]bool)
	var lastErr error

	for _, peer := range peers {
		if peer.State() != PeerStateEstablished {
			continue
		}

		neg := peer.negotiated.Load()
		if neg == nil {
			continue
		}

		if !neg.RouteRefresh {
			continue
		}

		for _, f := range neg.Families() {
			rr := &message.RouteRefresh{
				AFI:     f.AFI,
				SAFI:    f.SAFI,
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
		// Through the accessor: a reload swap can replace the slice on the shared
		// PeerSettings (peer_settings_negotiation.go), and this runs on the plugin
		// protocol's goroutine.
		for _, cap := range p.configuredCapabilities() {
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
		if r.rmetrics != nil {
			r.rmetrics.configReloadErrors.With("parse").Inc()
		}
		return fmt.Errorf("reload config: %w", err)
	}

	if err := a.reconcilePeers(newPeers, "reload"); err != nil {
		if r.rmetrics != nil {
			r.rmetrics.configReloadErrors.With("apply").Inc()
		}
		return fmt.Errorf("reconcile peers: %w", err)
	}
	if r.rmetrics != nil {
		r.rmetrics.configReloads.Inc()
	}
	return nil
}

// VerifyConfig validates peer settings without modifying reactor state.
// Peer parsing goes through loadPeersFullOrTree.
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
// Peer parsing goes through loadPeersFullOrTree.
// Called by the reload coordinator during the apply phase.
func (a *reactorAPIAdapter) ApplyConfigDiff(bgpTree map[string]any) error {
	newPeers, err := a.loadPeersFullOrTree(bgpTree)
	if err != nil {
		return fmt.Errorf("apply config diff: %w", err)
	}

	return a.reconcilePeers(newPeers, "apply config diff")
}

// loadPeersFullOrTree loads peers from a BGP config tree.
//
// A file-configured daemon has both a config path and a reload function
// (CreateReactor, ../config/loader.go, sets the two together), so it re-reads
// and re-parses the file. That route runs the full pipeline: PruneInactive,
// ResolveBGPTree, CheckRequiredFields and applyPeerSchemaDefaults before the
// parse, and patchRoutes after it (peersAndDynamicGroups, ../config/peers.go).
// It is what populates every field, including YANG defaults, group inheritance
// and static routes, and a PeerSettings missing them reads as a change to
// peerSettingsEqual. It also refreshes the dynamic peer groups, which the tree
// alone cannot do.
//
// Every other reactor parses the tree it was handed, with none of that around
// it. The only stage the two routes share is PeersFromTree (config.go), and
// sharing it is what stops them disagreeing about how a peer's config is READ.
// It does not make them equivalent, because they differ in what reaches the
// parser and in what runs on the result.
func (a *reactorAPIAdapter) loadPeersFullOrTree(bgpTree map[string]any) ([]*PeerSettings, error) {
	r := a.r

	configPath := r.config.ConfigPath
	r.mu.RLock()
	reloadFn := r.reloadFunc
	r.mu.RUnlock()

	if reloadFn != nil && configPath != "" {
		return reloadFn(configPath)
	}

	return PeersFromTree(bgpTree)
}

// configJournal records transactional apply/undo operations.
// Matches registry.ConfigJournal and pkg/plugin/sdk.Journal.
type configJournal interface {
	Record(apply, undo func() error) error
	Rollback() []error
	Discard()
}

// reconcilePeers diffs newPeers against the reactor's current peers and
// stops removed/changed peers, adds new/changed peers.
// Uses an internal journal for automatic rollback on failure.
// The label parameter is used for log messages (e.g., "reload", "apply config diff").
func (a *reactorAPIAdapter) reconcilePeers(newPeers []*PeerSettings, label string) error {
	j := &internalJournal{}
	if err := a.reconcilePeersJournaled(newPeers, label, j); err != nil {
		if rollbackErrs := j.Rollback(); len(rollbackErrs) > 0 {
			reactorLogger().Error(label+": rollback errors", "count", len(rollbackErrs))
		}
		return err
	}
	j.Discard()
	return nil
}

// reconcilePeersJournaled diffs newPeers against current peers, wrapping each
// remove and add operation in journal.Record for rollback support.
// Removes happen before adds (existing order preserved).
// On failure, the caller is responsible for calling journal.Rollback().
func (a *reactorAPIAdapter) reconcilePeersJournaled(newPeers []*PeerSettings, label string, j configJournal) error {
	r := a.r

	// Build map of new peer settings for quick lookup.
	newPeerSettings := make(map[netip.AddrPort]*PeerSettings)
	for _, p := range newPeers {
		newPeerSettings[p.PeerKey()] = p
	}

	// Get current peer addresses and settings snapshot. The session goes with the
	// settings because the swap-or-restart decision is taken against what the
	// RUNNING session negotiated, not against config alone
	// (peer_settings_negotiation.go). A peer with no session yields nil, which the
	// decision reads as "nothing to preserve" and restarts.
	//
	// SettingsSnapshot, not Settings: everything below reads the struct as a whole
	// (peerSettingsEqual here, and the struct copy plus the Capabilities read inside
	// peerSettingsSwapPlan), and the running peer's struct has two writers on two
	// goroutines -- applyHotSwappableSettings on this one, resolveDynamicPeerSettings
	// on the establishment one (peer.go, Settings). Reading the live pointer here
	// races the second, which would decide swap-or-restart from a torn slice header.
	// The snapshot is taken under p.mu and is immutable from here on, so the decision
	// stays a pure function of two PeerSettings and needs no lock of its own.
	r.mu.RLock()
	currentPeers := make(map[netip.AddrPort]*PeerSettings)
	currentSessions := make(map[netip.AddrPort]*Session)
	for key, peer := range r.peers {
		currentPeers[key] = peer.SettingsSnapshot()
		currentSessions[key] = peer.currentSession()
	}
	r.mu.RUnlock()

	// Categorize peers: to remove, to add, to swap in place, unchanged.
	var toRemove []netip.AddrPort
	var toAdd []*PeerSettings
	var toSwap []peerSettingsSwap

	for key := range currentPeers {
		newSettings, exists := newPeerSettings[key]

		// A dynamic peer is not in newPeerSettings and never can be: its key is the
		// AddrPort it CONNECTED FROM, while the configuration names a template and a
		// prefix range (createDynamicPeer, reactor_dynamic.go). Reading that absence
		// as "no longer configured" tore down every established dynamic session on
		// every reload, including a reload that changed nothing about it.
		//
		// The dynamic population is reconciled by SetDynamicGroups instead
		// (reactor_dynamic.go), which runs EARLIER in the same reload, from the
		// ReloadFunc itself (createReloadFunc, bgp/config/loader_create.go). It
		// removes the peers whose group is gone, whose address left every range, or
		// whose group template changed. What reaches this loop is the population it
		// decided to keep, so the answer here is to keep it.
		if currentPeers[key].IsDynamic {
			if !exists {
				continue
			}
			// The new configuration names this address as a peer of its own, so the
			// operator's entry replaces the template-built one. Restart rather than
			// swap: the running settings were resolved at establishment (PeerAS from
			// the OPEN, $remote_as and $remote_ip in the filter chains,
			// resolveDynamicPeerSettings), so they are not the template a config
			// entry can be diffed against, and applyHotSwappableSettings must not run
			// on a dynamic peer -- resolveDynamicPeerSettings writes two of the same
			// fields from the establishment goroutine
			// (plan/deferrals/fixit-dynamic-peer-settings-unlocked-read.md).
			toRemove = append(toRemove, key)
			toAdd = append(toAdd, newSettings)
			reactorLogger().Info("dynamic peer replaced by configured peer", "phase", label, "peer", key)
			continue
		}

		if !exists {
			toRemove = append(toRemove, key)
			continue
		}
		if peerSettingsEqual(currentPeers[key], newSettings) {
			continue
		}
		// A change the running session can take applies WITHOUT a restart: the
		// FSM, the TCP connection and the negotiated capabilities all survive
		// (peer_settings_apply.go). Anything else still costs a bounce, and the
		// reason names the fields that forced it, so an operator watching a
		// session flap on reload can see why.
		apply, reason := peerSettingsSwapPlan(currentPeers[key], newSettings, currentSessions[key])
		if reason != "" {
			toRemove = append(toRemove, key)
			toAdd = append(toAdd, newSettings)
			reactorLogger().Info("peer restart required", "phase", label, "peer", key, "changed", reason)
			continue
		}
		toSwap = append(toSwap, peerSettingsSwap{key: key, next: newSettings, apply: apply})
		reactorLogger().Debug("peer settings swapped in place", "phase", label, "peer", key)
	}

	// Swaps first: they change no map membership, so they neither see nor disturb
	// the add and remove loops below.
	if err := a.swapPeerSettingsJournaled(toSwap, j); err != nil {
		return err
	}

	for key, settings := range newPeerSettings {
		if _, exists := currentPeers[key]; !exists {
			toAdd = append(toAdd, settings)
		}
	}

	// Remove peers with journal recording (undo = re-add with old settings).
	for _, key := range toRemove {
		peerKey := key
		oldSettings := currentPeers[peerKey]
		err := j.Record(
			func() error {
				r.mu.Lock()
				defer r.mu.Unlock()
				if peer, ok := r.peers[peerKey]; ok {
					peer.Stop()
					// Stop only cancels the peer's context (Peer.Stop, peer.go);
					// the peer's own goroutine gives up its AS-wide BGP Identifier
					// claim later, from cleanup (peer_run.go). A reload that MOVES
					// an identifier between peers -- any router-id rotation or swap,
					// and any re-address that points a new peer at the router an
					// outgoing one served -- would then reach the add loop below and
					// dial the new holder while the outgoing peer still holds the
					// claim. routerIDClaims.claim refuses it (routerid_unique.go),
					// so ze answers a legitimate peer with OPEN Message Error / Bad
					// BGP Identifier and that session never establishes; whether it
					// happens is pure scheduling luck. Releasing synchronously here
					// removes the claim before the add loop runs, which closes the
					// ordinary window. The registry is a leaf lock, and the peer's
					// own later release is then a no-op that cannot touch the new
					// holder's entry (routerIDClaims.release checks holder.peer == p).
					//
					// It NARROWS the race rather than closing it, and the difference
					// matters: Stop only cancels a context and does not wait, so an
					// outgoing peer whose session goroutine is already inside
					// validateOpen can re-claim after this line. Nothing tombstones
					// it. Closing that fully needs the peer marked as removed so a
					// late claim is refused -- not done here, and it is the residual
					// this comment must not paper over.
					peer.releaseRouterIDClaim()
					// Clear any prefix-stale warning for this peer from the
					// report bus. Threshold warnings are cleared by the session
					// teardown defer in peer_run.go via Session.ClearReportedWarnings.
					// This mirrors the explicit RemovePeer path; without it,
					// stale entries leak when peers are removed via config reload.
					ClearPrefixStale(peerKey.Addr().String())
					if peer.health != nil {
						peer.health.stop()
					}
					delete(r.peers, peerKey)
					reactorLogger().Debug(label+": removed peer", "peer", peerKey)
				}
				return nil
			},
			func() error {
				return r.AddPeer(oldSettings)
			},
		)
		if err != nil {
			return fmt.Errorf("remove peer %s: %w", peerKey, err)
		}
	}

	// Add peers with journal recording (undo = stop and remove).
	for _, settings := range toAdd {
		addSettings := settings
		addKey := addSettings.PeerKey()
		err := j.Record(
			func() error {
				if addErr := r.AddPeer(addSettings); addErr != nil {
					return fmt.Errorf("add peer %s: %w", addSettings.Address, addErr)
				}
				reactorLogger().Debug(label+": added peer", "peer", addSettings.Address)
				return nil
			},
			func() error {
				r.mu.Lock()
				defer r.mu.Unlock()
				if peer, ok := r.peers[addKey]; ok {
					peer.Stop()
					delete(r.peers, addKey)
				}
				return nil
			},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// peerDiffCount computes the number of peer changes (adds + removes) between
// current peers and a new BGP config tree. Used for budget estimation.
func (a *reactorAPIAdapter) peerDiffCount(bgpTree map[string]any) (int, error) {
	newPeers, err := a.loadPeersFullOrTree(bgpTree)
	if err != nil {
		return 0, err
	}

	newPeerSettings := make(map[netip.AddrPort]*PeerSettings)
	for _, p := range newPeers {
		newPeerSettings[p.PeerKey()] = p
	}

	// SettingsSnapshot for the same reason reconcilePeersJournaled uses it: the two
	// judgements below, peerSettingsEqual and peerSettingsRestartRequired, are
	// whole-struct reads of a struct with writers on two goroutines (peer.go,
	// Settings). This count is only a budget estimate, but an estimate read from a
	// torn slice header still disagrees with the reconcile it is estimating.
	a.r.mu.RLock()
	currentPeers := make(map[netip.AddrPort]*PeerSettings)
	currentSessions := make(map[netip.AddrPort]*Session)
	for key, peer := range a.r.peers {
		currentPeers[key] = peer.SettingsSnapshot()
		currentSessions[key] = peer.currentSession()
	}
	a.r.mu.RUnlock()

	count := 0
	for key := range currentPeers {
		newSettings, exists := newPeerSettings[key]
		switch {
		case currentPeers[key].IsDynamic:
			// Mirrors reconcilePeersJournaled: a dynamic peer the configuration does
			// not name is left alone (SetDynamicGroups owns it), and one the
			// configuration DOES name is removed and re-added from the entry.
			if exists {
				count += 2
			}
		case !exists:
			count++ // remove
		case peerSettingsEqual(currentPeers[key], newSettings):
			// No change.
		case peerSettingsRestartRequired(currentPeers[key], newSettings, currentSessions[key]):
			count += 2 // remove + re-add
		default:
			count++ // swap in place: one apply, no session reset
		}
	}
	for key := range newPeerSettings {
		if _, exists := currentPeers[key]; !exists {
			count++ // add
		}
	}
	return count, nil
}

// internalJournal is a minimal journal for the non-transaction reconcilePeers path.
type internalJournal struct {
	entries []func() error
}

func (j *internalJournal) Record(apply, undo func() error) error {
	if err := apply(); err != nil {
		return err
	}
	j.entries = append(j.entries, undo)
	return nil
}

func (j *internalJournal) Rollback() []error {
	var errs []error
	for i := len(j.entries) - 1; i >= 0; i-- {
		if err := j.entries[i](); err != nil {
			errs = append(errs, err)
		}
	}
	j.entries = nil
	return errs
}

func (j *internalJournal) Discard() {
	j.entries = nil
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

// peerSettingsEqual compares two PeerSettings for reload diffing.
// Returns true if the settings are functionally equivalent.
//
// FAIL-CLOSED BY CONSTRUCTION (ai/rules/evidence.md): every field of
// PeerSettings participates via reflect.DeepEqual unless it is explicitly
// neutralized below with a stated reason. A field added to PeerSettings is
// therefore compared automatically and cannot be silently ignored on reload.
//
// This replaces a hand-maintained field list that compared ~15 fields and ignored
// ~35, including ImportFilters/ExportFilters (import/export policy), MD5Key (RFC
// 2385 TCP-MD5), RouteReflectorClient/ClusterID (RFC 4456), the prefix-limit maps,
// and the loop-detection fields. Because reconcilePeersJournaled only reconciles a
// peer this predicate reports as changed, and Peer.settings is assigned once in
// NewPeer (peer.go:318) with no setter, every omission meant the operator's edit
// was silently discarded until the daemon restarted. Regression coverage:
// peer_settings_reload_test.go.
//
// Cost: reflection on a ~50-field struct, once per peer per reload. Reload is rare
// (the pre-existing capability comparison already accepted this trade-off).
func peerSettingsEqual(a, b *PeerSettings) bool {
	if a == nil || b == nil {
		return a == b
	}

	// Capabilities compare SEMANTICALLY by wire encoding, not structurally: two
	// capability values may differ in Go representation yet encode identically.
	// Reload is rare, capabilities are small (<20 bytes each, <10 per peer).
	if !capabilitiesEqual(a.Capabilities, b.Capabilities) {
		return false
	}

	// Compare every remaining field structurally. Copy so the exclusions below do
	// not mutate the caller's settings. PeerSettings holds no locks, funcs, or
	// channels (only scalars, slices, maps, and pointers), so both the copy and
	// DeepEqual are well-defined.
	ac, bc := *a, *b

	// Excluded: compared semantically above.
	ac.Capabilities, bc.Capabilities = nil, nil

	// PrefixUpdated is NOT excluded. It holds the per-family ISO dates the prefix
	// maximums were last refreshed from PeeringDB (peersettings.go), and it drives
	// the prefix-stale warning and the ze_bgp_prefix_stale gauge. It was excluded
	// here until 2026-08-07 to stop a dates-only edit bouncing the session, which
	// also stopped the dates ever reaching a running peer: the alarm a PeeringDB
	// refresh was meant to clear stayed raised until the daemon restarted
	// (plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md). The bounce is
	// now answered where it belongs: the field is hot-swappable
	// (hotSwappableSettings, peer_settings_apply.go), so the change is delivered
	// to the running session and the FSM is never touched.
	return reflect.DeepEqual(&ac, &bc)
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
	ap := parsePeerAddrToKey(addr)
	if !ap.IsValid() {
		return fmt.Errorf("invalid peer address %q", addr)
	}
	return a.r.fwdPool.BarrierPeer(ctx, ap)
}

// DrainPeerSync blocks until no peer has pending route work -- for every peer,
// !PendingSync(): sendingInitialRoutes cleared AND its opQueue drained
// (peer_initial_sync.go). A peer that is still establishing but already has
// routes queued IS waited on: those routes drain when it comes up, so a test's
// send() reaches the wire before its next send(). Only a down/idle peer with an
// empty queue is skipped. Unlike ShouldQueue, the condition does NOT gate on
// peer state -- gating on state would let a route queued before establishment
// race ahead of the initial-sync EOR (the nexthop.ci ordering).
//
// This complements FlushForwardPool. Routes sent during a peer's initial-sync
// window are diverted into the opQueue and drained DIRECT to the session (not
// through the forward pool), so a "routes on the wire" guarantee needs BOTH
// barriers. Registered as the bgp-peer-sync quiescer alongside bgp-forward-pool.
//
// No completion signal exists for sendingInitialRoutes (a plain atomic cleared at
// several sites), so this polls the cheap PendingSync() condition rather than a
// fixed sleep: it returns as soon as the condition holds, and ctx bounds it.
func (a *reactorAPIAdapter) DrainPeerSync(ctx context.Context) error {
	return waitForCondition(ctx, time.Millisecond, a.peerSyncDrained)
}

// peerSyncDrained reports whether no peer has pending route work (see peersSynced):
// a still-establishing peer with queued routes is waited on; a down/idle peer with
// an empty queue is not.
func (a *reactorAPIAdapter) peerSyncDrained() bool {
	return peersSynced(a.r.Peers())
}

// peersSynced reports whether no peer has pending route work (!PendingSync for
// every peer): a peer with routes queued while establishing IS waited on (those
// routes drain when it comes up, and a test's send() must reach the wire before
// the next send), while a down/idle peer with an empty queue is skipped.
func peersSynced(peers []*Peer) bool {
	for _, p := range peers {
		if p.PendingSync() {
			return false
		}
	}
	return true
}

// waitForCondition returns nil as soon as cond() is true, or ctx.Err() if the
// deadline hits first. It checks once up front, then polls on a ticker. Used for
// draining barriers that have no completion signal to await.
func waitForCondition(ctx context.Context, tick time.Duration, cond func() bool) error {
	if cond() {
		return nil
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if cond() {
				return nil
			}
		}
	}
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

// getMatchingPeersSel resolves peers matching a typed selector.
// Caller must hold a.r.mu (read or write).
func (a *reactorAPIAdapter) getMatchingPeersSel(sel *selector.Selector) []*Peer {
	if sel.IsExclude() {
		included := a.matchPositive(sel)
		excludeSet := make(map[*Peer]struct{}, len(included))
		for _, p := range included {
			excludeSet[p] = struct{}{}
		}
		peers := make([]*Peer, 0, len(a.r.peers)-len(included))
		for _, peer := range a.r.peers {
			if _, skip := excludeSet[peer]; !skip {
				peers = append(peers, peer)
			}
		}
		return peers
	}
	return a.matchPositive(sel)
}

// matchPositive resolves the positive (non-excluded) peers for a selector.
// Caller must hold a.r.mu (read or write).
func (a *reactorAPIAdapter) matchPositive(sel *selector.Selector) []*Peer {
	switch sel.SelectorKind() {
	case selector.KindAll:
		peers := make([]*Peer, 0, len(a.r.peers))
		for _, peer := range a.r.peers {
			peers = append(peers, peer)
		}
		return peers

	case selector.KindAddr:
		if key := addrToKey(sel.IP()); key.IsValid() {
			if peer, ok := a.r.peers[key]; ok {
				return []*Peer{peer}
			}
		}
		addrStr := sel.IP().String()
		for _, peer := range a.r.peers {
			if peer.settings.Name == addrStr || peer.settings.Address.String() == addrStr {
				return []*Peer{peer}
			}
		}
		return nil

	case selector.KindAddrs:
		var peers []*Peer
		for _, ip := range sel.IPs() {
			if key := addrToKey(ip); key.IsValid() {
				if peer, ok := a.r.peers[key]; ok {
					peers = append(peers, peer)
				}
			}
		}
		return peers

	case selector.KindName:
		name := sel.NameValue()
		for _, peer := range a.r.peers {
			if peer.settings.Name == name {
				return []*Peer{peer}
			}
		}
		return nil

	case selector.KindASN:
		// Name has priority over ASN: a peer named "as65001" matches before ASN 65001.
		// Use a fresh non-excluded ASN selector for the name string (sel.String() would include "!" for excluded selectors).
		asnName := selector.ASN(sel.ASNValue()).String()
		for _, peer := range a.r.peers {
			if peer.settings.Name == asnName {
				return []*Peer{peer}
			}
		}
		asn := sel.ASNValue()
		var peers []*Peer
		for _, peer := range a.r.peers {
			// Guarded PeerAS read: peer selection runs on an API/plugin goroutine that
			// can race a dynamic peer's establishment write (caller holds r.mu).
			if peer.PeerAS() == asn {
				peers = append(peers, peer)
			}
		}
		return peers

	case selector.KindGlob:
		var peers []*Peer
		for addrPort, peer := range a.r.peers {
			if sel.Matches(addrPort.Addr()) {
				peers = append(peers, peer)
			}
		}
		return peers
	}

	return nil
}

// addrToKey converts a netip.Addr to the AddrPort key format used by the reactor.
func addrToKey(addr netip.Addr) netip.AddrPort {
	return netip.AddrPortFrom(addr, DefaultBGPPort)
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

// SetPeerUpBarrier declares how many barrier plugins a peer's peer-up event is
// being delivered to.
func (a *reactorAPIAdapter) SetPeerUpBarrier(peerAddr string, expected int) {
	a.r.SetPeerUpBarrier(peerAddr, expected)
}

// SignalPeerUpBarrier records that one barrier plugin has taken delivery of a
// peer's peer-up event.
func (a *reactorAPIAdapter) SignalPeerUpBarrier(peerAddr string) {
	a.r.SignalPeerUpBarrier(peerAddr)
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
