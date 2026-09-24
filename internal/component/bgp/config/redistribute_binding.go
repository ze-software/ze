// Design: docs/architecture/api/architecture.md -- the derived half of a delivery edge
// Related: peers.go -- peersAndDynamicGroups, the one walk that builds every peer
// Related: internal/component/bgp/reactor/config.go -- EnsureProcessBinding, where precedence is decided

package bgpconfig

import (
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
)

// locRIBReceive is the receive grant derived for a BGP redistribution source,
// written in the vocabulary of an `attach process` block.
//
// It states exactly what the plugin declares it handles. bgp-rib subscribes to
// `update direction sent`, `update direction received`, `state` and `refresh`
// (SetStartupSubscriptions, internal/component/bgp/plugins/rib/rib.go).
//
// Naming the same set on both halves keeps the delivery reconcile silent. The
// derived binding therefore adds no operator-facing warning
// (Server.reconcileDelivery, internal/component/plugin/server/delivery_reconcile.go).
const locRIBReceive = "update state refresh"

// The grant the redistribute orchestrator needs on a peer that a
// `destination bgp` rule feeds.
//
// `send [ update ]` is the permission to put a route on that peer's wire
// (Peer.maySend, internal/component/bgp/reactor/send_permission.go). Without
// it, the consumer's UpdateRoute is refused for every peer and the rule moves
// nothing.
//
// `receive [ state ]` is what the plugin declares (SetStartupSubscriptions,
// internal/component/bgp/plugins/redistribute_egress/register.go). It carries
// the peer's down-to-up edge, which is what fires the peer-up replay.
const (
	orchestratorReceive = "state"
	orchestratorSend    = "update"
)

// wireRedistributeDelivery derives Loc-RIB delivery for redistribution,
// FlowSpec authorization and AIGP selection. Explicit incomplete feature
// bindings are refused. Only destination-BGP rules grant forwarding authority.
func wireRedistributeDelivery(tree *config.Tree, settings []*reactor.PeerSettings) error {
	rules, err := config.ExtractRedistributeRules(tree)
	if err != nil {
		return err
	}

	flowSpec := false
	aigp := false
	for _, peer := range settings {
		aigp = aigp || peer.AIGPEnabled()
		flowSpec = flowSpec || hasFlowSpecFamily(peer)
	}
	locRIB, orchestrator := false, false
	for i := range rules {
		if bgpredist.SourceIsBGP(rules[i].Source) {
			locRIB = true
		}
		if bgpredist.DestinationIsBGP(rules[i].Destination) {
			orchestrator = true
		}
	}
	if !locRIB && !flowSpec && !aigp && !orchestrator {
		return nil
	}
	plugins, err := config.ExtractPluginsFromTree(tree)
	if err != nil {
		return err
	}
	if locRIB || flowSpec || aigp {
		// Selection and authorization consume inbound candidates, not sent-route
		// storage or refresh replay. Redistribution keeps its existing grant.
		receive := "update-received state"
		if locRIB {
			receive = locRIBReceive
		}
		if err := grantEveryPeer(plugins, settings, bgpredist.LocRIBPlugin, receive, ""); err != nil {
			return err
		}
	}
	if flowSpec {
		if err := requireRIBDelivery(plugins, settings, "FlowSpec authorization", true); err != nil {
			return err
		}
	}
	if aigp {
		if err := requireRIBDelivery(plugins, settings, "AIGP selection", false); err != nil {
			return err
		}
	}
	if orchestrator {
		if err := grantEveryPeer(plugins, settings, bgpredist.OrchestratorPlugin, orchestratorReceive, orchestratorSend); err != nil {
			return err
		}
	}
	return nil
}

// grantEveryPeer adds one derived binding to every peer the config builds. The
// process is named as the plugin server will run it, which is the operator's
// alias where a `plugin` block declares one.
func grantEveryPeer(plugins []plugin.PluginConfig, settings []*reactor.PeerSettings, registryName, receive, send string) error {
	process := processNameFor(plugins, registryName)
	for _, ps := range settings {
		if err := reactor.EnsureProcessBinding(ps, process, receive, send); err != nil {
			return fmt.Errorf("peer %s: %w", ps.Name, err)
		}
	}
	return nil
}

// processNameFor returns the name the plugin server will run registryName
// under. A peer's binding has to name it for the delivery graph to resolve the
// process (newDeliveryGraph, internal/component/plugin/server/delivery_graph.go).
//
// The operator can name it through `use` or an external `run` command. Reading
// the loader's effective PluginConfig also covers inline process declarations.
// Without a declaration, the server auto-loads the registry name from a config
// root it claims (Server.getConfigPathPlugins, startup_autoload.go).
//
// plugin.RegistryNames resolves the alias. The plugin server asks the same
// function whether a registry row is already configured, so both sides answer
// from one definition of the configured process's registry identity.
func processNameFor(plugins []plugin.PluginConfig, registryName string) string {
	for _, declared := range plugins {
		if slices.Contains(plugin.RegistryNames(declared), registryName) {
			return declared.Name
		}
	}
	return registryName
}
