// Design: docs/guide/flowspec-protected-router.md -- mandatory authorization input
package bgpconfig

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"slices"
)

func requireRIBDelivery(plugins []plugin.PluginConfig, peers []*reactor.PeerSettings, feature string, internalOnly bool) error {
	// Authorization and native replay use process-local selecting-RIB lookups.
	// Event delivery alone cannot cross that boundary.
	if internalOnly {
		for _, declared := range plugins {
			if declared.Internal {
				continue
			}
			names := plugin.RegistryNames(declared)
			if slices.Contains(names, bgpredist.LocRIBPlugin) {
				return fmt.Errorf("%s requires an internal bgp-rib process; %q is external", feature, declared.Name)
			}
			if !slices.Contains(names, "bgp-rs") && !slices.Contains(names, "bgp-rr") && !slices.Contains(names, "bgp-adj-rib-in") {
				continue
			}
			for _, peer := range peers {
				if !hasFlowSpecFamily(peer) {
					continue
				}
				for i := range peer.ProcessBindings {
					binding := &peer.ProcessBindings[i]
					// RelayStoredRoute requires SendUpdate. Receive-only and
					// raw-only bindings cannot own this replay rail. An
					// Adj-RIB-In can regain ownership when a forwarder's
					// per-peer claim is retracted, so it needs the provider too.
					if binding.PluginName == declared.Name && binding.ReceivesPeerState() && binding.MaySend(bgpevents.SendUpdate) {
						return fmt.Errorf("peer %s: FlowSpec replay requires process %q to run internally; an external process cannot query the selecting RIB", peer.Name, declared.Name)
					}
				}
			}
		}
	}
	process := processNameFor(plugins, bgpredist.LocRIBPlugin)
	for _, peer := range peers {
		complete := false
		for _, binding := range peer.ProcessBindings {
			if binding.PluginName != process {
				continue
			}
			direction := binding.Receive[bgpevents.EventUpdate]
			complete = binding.ReceiveAll || (binding.ReceivesPeerState() &&
				(direction == events.DirReceived || direction == events.DirBoth))
			break
		}
		if !complete {
			return fmt.Errorf("peer %s: %s requires process %q to receive update-received and state from every BGP peer; remove the restrictive attach binding or grant those events", peer.Name, feature, process)
		}
	}
	return nil
}

func hasFlowSpecFamily(peer *reactor.PeerSettings) bool {
	for _, name := range [...]string{"ipv4/flow", "ipv6/flow", "ipv4/flow-vpn", "ipv6/flow-vpn"} {
		if _, configured := peer.PrefixMaximum[name]; configured {
			return true
		}
	}
	return false
}
