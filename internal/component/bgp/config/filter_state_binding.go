// Design: docs/architecture/api/architecture.md -- the derived half of a delivery edge
// Related: filter_insertion.go -- insertDeclaredFilters, which places the members this reads
// Related: redistribute_binding.go -- processNameFor, the process a registry name runs under
// Related: internal/component/bgp/reactor/config.go -- GrantImplicitReceive, the grant and its precedence

package bgpconfig

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
)

// filterOwnerReceive is the grant derived for the plugin whose member a
// peer's chain carries: the peer's state, and nothing else. A member keeps
// state per peer (a learned role, say), and the `state` event with reason
// `removed` is how it learns to free it, on either transport
// (spec-filter-wire-6-stage-filters, "Peer removal notice").
const filterOwnerReceive = bgpevents.EventState

// grantFilterOwnersState grants, on every peer and dynamic group template,
// the peer's `state` to each plugin one of whose members the peer's import
// or export list carries, inserted or named by the operator, marked
// implicit. It MUST run after the insertions are placed and the references
// are canonical. An operator's `attach process` block for that plugin that
// does not grant `state` gains it: the plugin receives the peer's removal
// regardless, because the per-peer state its member keeps is the plugin's
// obligation, not the operator's (GrantImplicitReceive). Delivery is still
// the overlap of both halves, so a plugin that does not subscribe to
// `state` receives nothing more.
func grantFilterOwnersState(tree *config.Tree, settings []*reactor.PeerSettings) error {
	plugins, err := config.ExtractPluginsFromTree(tree)
	if err != nil {
		return err
	}
	types := registry.FilterTypesMap()
	for _, ps := range settings {
		for _, ref := range slices.Concat(ps.ImportPolicy.Filters(), ps.ExportPolicy.Filters()) {
			process, owned := filterOwnerProcess(ref, plugins, types)
			if !owned {
				continue
			}
			if err := reactor.GrantImplicitReceive(ps, process, filterOwnerReceive); err != nil {
				return fmt.Errorf("peer %s: %w", ps.Name, err)
			}
		}
	}
	return nil
}

// filterOwnerProcess answers the process that serves an active canonical
// reference: the owner of a `<type>:<instance>` filter type, under the name
// the plugin server runs it, or the plugin of `<plugin>:<filter>` when a
// compiled registration or the config's plugin block knows it. A reference
// no process serves (a bare policy name, an in-process bound member) and an
// inactive member, which runs nowhere, answer false.
func filterOwnerProcess(ref policy.Ref, plugins []plugin.PluginConfig, types map[string]string) (string, bool) {
	if ref.Inactive {
		return "", false
	}
	prefix, _, qualified := strings.Cut(ref.Name, ":")
	if !qualified {
		return "", false
	}
	if owner, isType := types[prefix]; isType {
		return processNameFor(plugins, owner), true
	}
	if registry.Lookup(prefix) != nil {
		return processNameFor(plugins, prefix), true
	}
	declared := slices.ContainsFunc(plugins, func(p plugin.PluginConfig) bool { return p.Name == prefix })
	return prefix, declared
}
