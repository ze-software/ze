// Design: docs/architecture/api/architecture.md -- the derived half of a delivery edge
// Related: peers.go -- peersAndDynamicGroups, the one walk that builds every peer
// Related: internal/component/bgp/reactor/config.go -- EnsureProcessBinding, where precedence is decided

package bgpconfig

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
)

// locRIBReceive is the receive grant the built-in Loc-RIB needs, written in the
// vocabulary an operator writes in an `attach process` block.
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

// wireRedistributeDelivery grants every peer the process bindings the config's
// `redistribute` rules depend on, and grants nothing where no rule needs one.
//
// Two rules imply a binding, and each implies its own:
//
//	import bgp        the Loc-RIB must be FED this peer's UPDATEs
//	destination bgp   the orchestrator must be PERMITTED to send to this peer
//
// Each has one correct value, so the config derives it rather than asking the
// operator to write plumbing no page documents (Key Design Decisions,
// spec-fixit-redistribution-chain-drops-silently). A config whose rules imply
// neither is left exactly as written.
func wireRedistributeDelivery(tree *config.Tree, settings []*reactor.PeerSettings) error {
	rules, err := config.ExtractRedistributeRules(tree)
	if err != nil {
		return err
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
	if locRIB {
		if err := grantEveryPeer(tree, settings, bgpredist.LocRIBPlugin, locRIBReceive, ""); err != nil {
			return err
		}
	}
	if orchestrator {
		if err := grantEveryPeer(tree, settings, bgpredist.OrchestratorPlugin, orchestratorReceive, orchestratorSend); err != nil {
			return err
		}
	}
	return nil
}

// grantEveryPeer adds one derived binding to every peer the config builds. The
// process is named as the plugin server will run it, which is the operator's
// alias where a `plugin` block declares one.
func grantEveryPeer(tree *config.Tree, settings []*reactor.PeerSettings, registryName, receive, send string) error {
	process := processNameFor(tree, registryName)
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
// The operator chooses that name with a `plugin` block, written
// `internal <alias> { use <registryName> }`. Where no block declares one, the
// server auto-loads the plugin under its registry name from a config root it
// claims (Server.getConfigPathPlugins,
// internal/component/plugin/server/startup_autoload.go).
//
// plugin.RegistryNames resolves the alias. The plugin server asks the same
// function whether a registry row is already configured, so both sides answer
// from one definition of what `use` means.
func processNameFor(tree *config.Tree, registryName string) string {
	pluginContainer := tree.GetContainer("plugin")
	if pluginContainer == nil {
		return registryName
	}
	for _, kind := range []string{"internal", "external"} {
		for name, declared := range pluginContainer.GetList(kind) {
			use, _ := declared.Get("use")
			if use == "" {
				continue
			}
			for _, candidate := range plugin.RegistryNames(plugin.PluginConfig{Name: name, Run: use}) {
				if candidate == registryName {
					return name
				}
			}
		}
	}
	return registryName
}
