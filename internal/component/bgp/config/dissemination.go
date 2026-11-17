// Design: docs/architecture/api/architecture.md -- declared transit grants.
// Related: peers.go -- the common validate, commit and startup peer walk.
package bgpconfig

import (
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
)

// validateDissemination resolves registrations rather than naming particular
// implementations. Explicit attachments to compiled declarations are checked
// before any peer starts; external declarations receive the same runtime gate.
func validateDissemination(tree *config.Tree, peers []*reactor.PeerSettings) error {
	configured, err := config.ExtractPluginsFromTree(tree)
	if err != nil {
		return err
	}
	var processes []pluginserver.DisseminationProcess
	seen := make(map[string]bool)
	add := func(name string, identities []string) {
		if seen[name] {
			return
		}
		seen[name] = true
		for _, identity := range identities {
			row := registry.Lookup(identity)
			if row == nil {
				continue
			}
			if len(row.Dissemination) == 0 {
				continue
			}
			processes = append(processes, pluginserver.DisseminationProcess{Name: name, Claims: row.Claims, Declarations: row.Dissemination})
			return
		}
	}
	for _, process := range configured {
		add(process.Name, plugin.RegistryNames(process))
	}
	for _, peer := range peers {
		for _, binding := range peer.ProcessBindings {
			add(binding.PluginName, []string{binding.PluginName})
		}
	}
	delivery := reactor.DisseminationPeersFromSettings(peers)
	graph := pluginserver.NewDisseminationGraph(bgpevents.Namespace, delivery)
	if err := graph.ValidateDisseminationBindings(processes); err != nil {
		return err
	}
	for i, peer := range peers {
		owner, err := graph.ResolveDisseminationOwner(delivery[i].Addr, reactor.SelectedDisseminationRole, processes)
		if err != nil {
			return err
		}
		peer.DisseminationOwner = owner
	}
	return nil
}
