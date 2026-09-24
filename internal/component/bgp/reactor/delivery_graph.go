// Design: docs/architecture/api/architecture.md -- peer-to-process delivery edges
// Related: peer_settings_apply.go -- a change to a peer's attach block travels through
//   RemovePeer and AddPeer; only a derived feed-only binding is delivered in place
//   (applyHotSwappableSettings, peer_derived_bindings.go)
// Related: reactor_peers.go, reactor_dynamic.go, reactor_api.go -- the publish
//   points: StartWithContext once before the peers start, then AddPeer,
//   doRemovePeer, createDynamicPeer, removeDynamicPeer and the end of every
//   config apply (reconcilePeersJournaled)

package reactor

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
)

// deliveryPeersFromSettings turns resolved peer settings into the delivery
// graph's input.
//
// RESOLVED settings, never the config tree: a peer built from a group already
// carries its group's `attach process` blocks, merged by ResolveBGPTree, and a
// member that restates one has already replaced the group's list. Reading the
// document instead is what makes config/graph.go's addProcessBindings miss
// every inherited binding; that function feeds `ze config graph` and must not
// feed delivery.
func deliveryPeersFromSettings(peers []*PeerSettings) []pluginserver.DeliveryPeer {
	out := make([]pluginserver.DeliveryPeer, 0, len(peers))
	for _, s := range peers {
		bindings := make([]plugin.PeerProcessBinding, 0, len(s.ProcessBindings))
		for _, b := range s.ProcessBindings {
			bindings = append(bindings, plugin.PeerProcessBinding{
				PluginName: b.PluginName,
				Encoding:   b.Encoding,
				Format:     b.Format,
				ReceiveAll: b.ReceiveAll,
				Receive:    b.Receive,
				SendAll:    b.SendAll,
				Send:       b.Send,
			})
		}
		out = append(out, pluginserver.DeliveryPeer{
			Addr:     s.Address.String(),
			Name:     s.Name,
			Bindings: bindings,
		})
	}
	return out
}

// publishDeliveryGraphLocked rebuilds the peer-to-process index from the
// reactor's own peers and swaps it into the plugin server.
//
// The caller MUST hold r.mu. Address and Name are written once, when the peer is
// built, and no later path writes them. ProcessBindings has two later writers,
// applyHotSwappableSettings and setDynamicGroupsLocked, and both deliver only a
// derived feed-only change while holding r.mu, so reading it here under r.mu
// without p.mu is safe. The build only reads the reactor, and
// the plugin server only builds an index and stores a pointer, so nothing calls
// back into the reactor under the lock.
func (r *Reactor) publishDeliveryGraphLocked() {
	if r.api == nil {
		return
	}
	settings := make([]*PeerSettings, 0, len(r.peers))
	for _, peer := range r.peers {
		settings = append(settings, peer.settings)
	}
	r.api.UpdateDeliveryGraph(bgpevents.Namespace, deliveryPeersFromSettings(settings))
	// The index is live from here, so every later peer change republishes.
	r.deliveryPublished = true
}
