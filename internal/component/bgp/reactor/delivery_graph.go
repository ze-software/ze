// Design: docs/architecture/api/architecture.md -- peer-to-process delivery edges
// Related: peer_settings_apply.go -- a change to a peer's bus receive block travels
//   through RemovePeer and AddPeer; only a feed-only bus send change is delivered in
//   place (applyHotSwappableSettings, peer_derived_bindings.go)
// Related: reactor_peers.go, reactor_dynamic.go, reactor_api.go -- the publish
//   points: StartWithContext once before the peers start, then AddPeer,
//   doRemovePeer, createDynamicPeer, removeDynamicPeer and the end of every
//   config apply (reconcilePeersJournaled)

package reactor

import (
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const bgpSecondaryIPPrefix = "/bgp/ip/"

// DeliveryGraph returns the reactor's current compiled delivery index, or nil
// when no graph has been published on a standalone reactor. Safe for concurrent
// use.
func (r *Reactor) DeliveryGraph() *pluginserver.DeliveryGraph {
	if r == nil {
		return nil
	}
	if r.api != nil {
		return r.api.DeliveryGraph()
	}
	return r.deliveryGraph.Load()
}

// SetDeliveryGraph installs a compiled DeliveryGraph directly on a standalone
// reactor (used by unit tests running without a pluginserver.Server). Safe for
// concurrent use.
func (r *Reactor) SetDeliveryGraph(g *pluginserver.DeliveryGraph) {
	if r == nil {
		return
	}
	r.deliveryGraph.Store(g)
}

// deliveryPeerPathForSettings returns the canonical short bus path for s.
func deliveryPeerPathForSettings(s *PeerSettings) string {
	if s == nil {
		return ""
	}
	if s.BusPath != "" {
		return s.BusPath
	}
	var tb textbuf.Buffer
	if s.GroupName != "" {
		if s.Name != "" {
			return tb.Str("/bgp/group/").Str(s.GroupName).Str("/peer/").Str(s.Name).String()
		}
		if s.Address.IsValid() {
			return tb.Str("/bgp/group/").Str(s.GroupName).Str("/peer/").Addr(s.Address).String()
		}
		return tb.Str("/bgp/group/").Str(s.GroupName).String()
	}
	if s.Name != "" {
		return tb.Str("/bgp/peer/").Str(s.Name).String()
	}
	if s.Address.IsValid() {
		return tb.Str(bgpSecondaryIPPrefix).Addr(s.Address).String()
	}
	return ""
}

// deliveryPeerParentPathForSettings returns the parent group path for a peer
// belonging to a BGP group, or "" for top-level peers and group templates.
func deliveryPeerParentPathForSettings(s *PeerSettings) string {
	if s == nil {
		return ""
	}
	if s.BusGroupPath != "" {
		return s.BusGroupPath
	}
	if s.GroupName == "" {
		return ""
	}
	gPath := "/bgp/group/" + s.GroupName
	if deliveryPeerPathForSettings(s) == gPath {
		return ""
	}
	return gPath
}

// deliveryPeerSessionFacts projects the session-scope facts evaluated against
// ClaimSessionScope for s with remote AS peerAS.
func deliveryPeerSessionFacts(s *PeerSettings, peerAS uint32) map[string]string {
	if s == nil {
		return nil
	}
	rsClient := valFalse
	if s.RSClient {
		rsClient = valTrue
	}
	sessType := "ebgp"
	if s.isIBGPWith(peerAS) {
		sessType = "ibgp"
	}
	rrClient := valFalse
	if s.RouteReflectorClient {
		rrClient = valTrue
	}
	return map[string]string{
		"rs-client":              rsClient,
		"session-type":           sessType,
		"route-reflector-client": rrClient,
	}
}

// deliveryPeerFromSettings converts one resolved PeerSettings into its
// DeliveryPeer projection.
func deliveryPeerFromSettings(s *PeerSettings) pluginserver.DeliveryPeer {
	if s == nil {
		return pluginserver.DeliveryPeer{}
	}
	return deliveryPeerFromSettingsWithAS(s, s.PeerAS)
}

// deliveryPeerFromSettingsWithAS converts one resolved PeerSettings into its
// DeliveryPeer projection using an explicit remoteAS snapshot (safe to call
// without p.mu while resolveDynamicPeerSettings updates p.settings.PeerAS).
func deliveryPeerFromSettingsWithAS(s *PeerSettings, remoteAS uint32) pluginserver.DeliveryPeer {
	if s == nil {
		return pluginserver.DeliveryPeer{}
	}
	pPath := deliveryPeerPathForSettings(s)
	parentPath := deliveryPeerParentPathForSettings(s)
	addr := ""
	var secondary []string
	if s.Address.IsValid() {
		addr = s.Address.String()
		ipPath := bgpSecondaryIPPrefix + addr
		if pPath != ipPath {
			secondary = []string{ipPath}
		}
	}
	source := s.BusSource
	if source == "" && !s.Bus.IsEmpty() && pPath != "" {
		source = pPath + " > bus"
	}
	return pluginserver.DeliveryPeer{
		Addr:            addr,
		Name:            s.Name,
		Path:            pPath,
		ParentPath:      parentPath,
		SecondaryPaths:  secondary,
		Bus:             s.Bus.Clone(),
		BusOrigin:       s.BusOrigin,
		BusSource:       source,
		OwnBus:          s.OwnBus.Clone(),
		ParentBus:       s.GroupBus.Clone(),
		ParentBusSource: s.GroupBusSource,
		Dynamic:         s.IsDynamic,
		SessionFacts:    deliveryPeerSessionFacts(s, remoteAS),
	}
}

// deliveryPeersFromSettings turns resolved peer settings into the delivery
// graph's input.
//
// RESOLVED settings, never the config tree: a peer built from a group already
// carries its group's merged `bus` block from ResolveBGPTree.
func deliveryPeersFromSettings(peers []*PeerSettings) []pluginserver.DeliveryPeer {
	out := make([]pluginserver.DeliveryPeer, 0, len(peers))
	for _, s := range peers {
		out = append(out, deliveryPeerFromSettings(s))
	}
	dedupSecondaryPaths(out)
	return out
}

// dedupSecondaryPaths strips any secondary lookup path shared by more than one
// peer in the same snapshot (for example two peers sharing an IP on different
// ports) so CompileBus never rejects a secondary-path collision.
func dedupSecondaryPaths(peers []pluginserver.DeliveryPeer) {
	if len(peers) < 2 {
		return
	}
	counts := make(map[string]int, len(peers))
	for i := range peers {
		for _, sp := range peers[i].SecondaryPaths {
			counts[sp]++
		}
	}
	for i := range peers {
		if len(peers[i].SecondaryPaths) == 0 {
			continue
		}
		kept := peers[i].SecondaryPaths[:0]
		for _, sp := range peers[i].SecondaryPaths {
			if counts[sp] == 1 {
				kept = append(kept, sp)
			}
		}
		if len(kept) == 0 {
			peers[i].SecondaryPaths = nil
		} else {
			peers[i].SecondaryPaths = kept
		}
	}
}

// DisseminationPeersFromSettings preserves exact configured peer identity for
// role resolution: peers sharing an IP but using different ports stay distinct.
func DisseminationPeersFromSettings(peers []*PeerSettings) []pluginserver.DeliveryPeer {
	out := deliveryPeersFromSettings(peers)
	seenPaths := make(map[string]bool, len(out))
	for i, peer := range peers {
		out[i].Addr = peer.PeerKey().String()
		if !peer.Address.IsValid() {
			// A template has no connection yet. Preserve each original node
			// so two groups cannot complete each other's required grants.
			out[i].Addr = "dynamic-template:" + strconv.Itoa(i)
			if out[i].Path == "" {
				out[i].Path = "/bgp/dynamic-template/" + strconv.Itoa(i)
			}
		} else if out[i].Path == "" {
			out[i].Path = bgpSecondaryIPPrefix + peer.PeerKey().String()
		}
		if seenPaths[out[i].Path] {
			// Disambiguate unnamed peers sharing an IP on different ports so
			// BuildDeliveryGraph indexes both entries independently.
			var tb textbuf.Buffer
			out[i].Path = tb.Str(out[i].Path).Str("/port/").Uint16(peer.PeerKey().Port()).String()
		}
		seenPaths[out[i].Path] = true
	}
	return out
}

// publishDeliveryGraphLocked rebuilds the peer-to-process index from the
// reactor's own peers and swaps it into the plugin server (or local cache on a
// standalone reactor).
//
// The caller MUST hold r.mu. Address and Name are written once, when the peer is
// built, and no later path writes them. Bus has one later writer,
// applyHotSwappableSettings (through Peer.writeBus and its own write), and it
// delivers only a feed-only change while holding r.mu, so reading it here under
// r.mu without p.mu is safe.
func (r *Reactor) publishDeliveryGraphLocked() {
	peers := make([]pluginserver.DeliveryPeer, 0, len(r.peers))
	for _, peer := range r.peers {
		peers = append(peers, deliveryPeerFromSettingsWithAS(peer.settings, peer.resolvedPeerAS.Load()))
	}
	dedupSecondaryPaths(peers)
	if r.api != nil {
		r.api.UpdateDeliveryGraph(bgpevents.Namespace, peers)
		// The index is live from here, so every later peer change republishes.
		r.deliveryPublished = true
		return
	}
	g, err := pluginserver.BuildDeliveryGraph(bgpevents.Namespace, peers, r.busConfig, r.testBusEndpoints)
	if err != nil {
		reactorLogger().Warn("standalone delivery graph compilation failed; retaining previous index", "err", err, "code", pluginserver.CodeDoctorPluginDeliveryDisagreement)
		return
	}
	r.deliveryGraph.Store(g)
	r.deliveryPublished = true
}

// resolveRegistrationName maps a configured endpoint/process name to its
// compiled plugin registration name via the process manager when available.
// When the process is known to the manager, its compiled RegistrationName is
// authoritative: a foreign external process carries "" and never falls back to
// its endpoint name (AC-13, R-4). Only when no live process exists in the
// manager (standalone reactor unit tests) does procName act as the fallback.
func resolveRegistrationName(r *Reactor, procName string) string {
	if r != nil && r.api != nil {
		if mgr := r.api.ProcessManager(); mgr != nil {
			if proc := mgr.GetProcess(procName); proc != nil {
				if regName := proc.RegistrationName(); regName != "" {
					return regName
				}
				if !proc.Config().Internal {
					return ""
				}
				for _, rn := range plugin.RegistryNames(proc.Config()) {
					if registry.Lookup(rn) != nil {
						return rn
					}
				}
				return ""
			}
		}
	}
	return procName
}

// peerSettingsMatchesClaimScope reports whether peer settings (with guarded
// remote AS peerAS) satisfy every session-fact predicate in scope.
func peerSettingsMatchesClaimScope(s *PeerSettings, peerAS uint32, scope []rpc.FilterSessionFact) bool {
	if len(scope) == 0 {
		return true
	}
	if s == nil {
		return false
	}
	for _, fact := range scope {
		val, known := peerSettingsSessionFactValue(s, peerAS, fact.Fact)
		if !known {
			return false
		}
		if len(fact.Values) > 0 && !slices.Contains(fact.Values, val) {
			return false
		}
	}
	return true
}

func peerSettingsSessionFactValue(s *PeerSettings, peerAS uint32, fact string) (string, bool) {
	switch fact {
	case "rs-client":
		if s.RSClient {
			return valTrue, true
		}
		return valFalse, true
	case "session-type":
		if s.isIBGPWith(peerAS) {
			return "ibgp", true
		}
		return "ebgp", true
	case "route-reflector-client":
		if s.RouteReflectorClient {
			return valTrue, true
		}
		return valFalse, true
	case "ibgp":
		if s.isIBGPWith(peerAS) {
			return valTrue, true
		}
		return valFalse, true
	default:
		return "", false
	}
}
