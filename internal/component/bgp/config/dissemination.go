// Design: docs/architecture/api/architecture.md -- declared dissemination ownership over compiled bus filters
// Related: bus_requirements.go -- compiles the bus and runs all BGP feature requirements
// Related: peers.go -- the common validate, commit and startup peer walk
package bgpconfig

import (
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// validateDisseminationOnGraph resolves dissemination declarations across all
// configured and auto-loaded compiled endpoints in busCtx, validates complete
// grants on the compiled DeliveryGraph, and records each peer's resolved
// DisseminationOwner.
func validateDisseminationOnGraph(settings []*reactor.PeerSettings, busCtx *bgpCompiledBusContext) error {
	var processes []pluginserver.DisseminationProcess
	seen := make(map[string]bool)
	seenCompiled := make(map[string]bool)

	add := func(name string, identities []string) {
		if name == "" {
			return
		}
		if seen[name] {
			return
		}
		seen[name] = true
		for _, identity := range identities {
			row := registry.Lookup(identity)
			if row == nil {
				continue
			}
			seenCompiled[row.Name] = true
			if len(row.Dissemination) == 0 {
				continue
			}
			processes = append(processes, pluginserver.DisseminationProcess{
				Name:              name,
				Claims:            row.Claims,
				ClaimSessionScope: slices.Clone(row.ClaimSessionScope),
				Declarations:      row.Dissemination,
			})
			return
		}
	}

	for _, ep := range busCtx.endpoints {
		ids := plugin.RegistryNames(ep)
		if id := ep.CompiledIdentity(); id != "" && !slices.Contains(ids, id) {
			ids = append([]string{id}, ids...)
		}
		add(ep.Name, ids)
	}
	if busCtx.compiled != nil {
		for _, pPath := range busCtx.compiled.Participants() {
			f := busCtx.compiled.Filter(pPath)
			if f == nil {
				continue
			}
			if f.EndpointName == "" {
				continue
			}
			if seen[f.EndpointName] {
				continue
			}
			if row := registry.Lookup(f.EndpointName); row != nil && !seenCompiled[row.Name] {
				add(f.EndpointName, []string{row.Name})
			}
		}
	}

	if err := busCtx.graph.ValidateDisseminationBindings(processes); err != nil {
		return err
	}
	for _, peer := range settings {
		addr := busCtx.peerToAddr[peer]
		owner, err := busCtx.graph.ResolveDisseminationOwner(addr, reactor.SelectedDisseminationRole, processes)
		if err != nil {
			return err
		}
		peer.DisseminationOwner = owner
	}
	return nil
}
