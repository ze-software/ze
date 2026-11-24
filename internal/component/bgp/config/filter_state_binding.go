// Design: docs/architecture/api/architecture.md -- filter-owner resolution for compiled bus state requirements
// Related: filter_insertion.go -- insertDeclaredFilters, which places the members this reads
// Related: bus_requirements.go -- validates that every filter owner receives bgp:state

package bgpconfig

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// filterOwnerProcess answers the endpoint that serves an active canonical
// reference: the owner of a `<type>:<instance>` filter type, under the name
// the plugin server runs it, or the plugin of `<plugin>:<filter>` when a
// compiled registration or the config's bus endpoint list knows it. A reference
// no process serves (a bare policy name, an in-process bound member, or a
// built-in next-hop:* engine step) and an inactive member, which runs
// nowhere, answer false.
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
