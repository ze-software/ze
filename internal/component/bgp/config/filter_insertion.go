// Design: docs/architecture/core-design.md -- Policy Filter Chain, auto-inserted members
// Related: peers.go -- prependDefaultFilters, the operator-config prepend this generalises

package bgpconfig

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// insertDeclaredFilters inserts into each peer's filter list the members
// plugins declared for the peers whose config needs them (rpc.FilterInsertion,
// spec-filter-wire-6-stage-filters D-5). It reads only the declarations and
// the resolved config: no plugin, feature or path is named here (D-5a), so a
// plugin that declares an insertion is inserted with nobody editing this file.
//
// It MUST run after the chains are canonical, so an operator's reference and
// the inserted one compare in one form, and before the chains compile.
func insertDeclaredFilters(bgpTree map[string]any, peers []*reactor.PeerSettings,
	groups []*reactor.DynamicGroupConfig, reg *policy.Registry,
) error {
	insertions, err := declaredInsertions()
	if err != nil {
		return err
	}
	if len(insertions) == 0 {
		return nil
	}
	peerMap, _ := bgpTree["peer"].(map[string]any)
	for _, ps := range peers {
		resolved, _ := peerMap[ps.Name].(map[string]any)
		if err := insertForPeer(ps, resolved, insertions, reg); err != nil {
			return err
		}
	}
	// A dynamic group's members are built from its template, so the template
	// is the resolved config its condition is judged against, as peerRoles
	// judges a role.
	for _, dg := range groups {
		if err := insertForPeer(dg.Settings, dg.Template, insertions, reg); err != nil {
			return err
		}
	}
	return nil
}

// declaredInsertions collects every compiled-in registration's insertions,
// each judged by the same rules as at Stage 1, sorted by priority then
// reference so the order never depends on registration order.
func declaredInsertions() ([]rpc.FilterInsertion, error) {
	var insertions []rpc.FilterInsertion
	for _, r := range registry.All() {
		if len(r.FilterInsertions) == 0 {
			continue
		}
		if err := registry.CheckFilterInsertions(r.Name, r.Filters, r.FilterTypes, r.FilterInsertions); err != nil {
			return nil, err
		}
		if err := config.CheckFilterInsertionPaths(r.Name, r.YANG, r.FilterInsertions); err != nil {
			return nil, err
		}
		insertions = append(insertions, r.FilterInsertions...)
	}
	slices.SortStableFunc(insertions, func(a, b rpc.FilterInsertion) int {
		if c := cmp.Compare(a.Priority, b.Priority); c != 0 {
			return c
		}
		return cmp.Compare(a.Filter, b.Filter)
	})
	return insertions, nil
}

// insertForPeer prepends, in reverse order, each insertion whose condition
// the peer's resolved config meets, so the list reads in ascending priority
// ahead of the operator's members. An operator reference (plain or
// `inactive:`) to a non-required member places or switches it off, the rule
// filterChainContains applies to loop-detection; to a required one it is
// refused with the declared reason.
func insertForPeer(ps *reactor.PeerSettings, resolved map[string]any,
	insertions []rpc.FilterInsertion, reg *policy.Registry,
) error {
	for i := len(insertions) - 1; i >= 0; i-- {
		ins := &insertions[i]
		if !insertionConditionHolds(resolved, ins.Config, ins.Values) {
			continue
		}
		refs, bound := canonicalizeFilterRefs([]policy.Ref{{Name: ins.Filter}}, reg)
		ref := refs[0]
		inProcess := len(bound) > 0 && bound[0]
		if ins.Direction == rpc.FilterImport {
			if filterChainContains(ps.ImportPolicy.Filters(), ref.Name) {
				if ins.Required {
					return requiredInsertionRefusal(ps, "import", ins)
				}
				continue
			}
			ps.ImportPolicy = ps.ImportPolicy.Prepend(ref, inProcess)
			continue
		}
		if filterChainContains(ps.ExportPolicy.Filters(), ref.Name) {
			if ins.Required {
				return requiredInsertionRefusal(ps, "export", ins)
			}
			continue
		}
		ps.ExportPolicy = ps.ExportPolicy.Prepend(ref, inProcess)
	}
	return nil
}

// insertionConditionHolds reports whether the resolved config holds the path,
// and, when values are given, whether its leaf equals one of them. Values
// arrive as strings (ai/rules/config.md), so a boolean leaf reads "true".
func insertionConditionHolds(resolved map[string]any, path, values []string) bool {
	node, present := resolvedPath(resolved, path)
	return present && insertionLeafMatches(node, values)
}

func insertionLeafMatches(node any, values []string) bool {
	if len(values) == 0 {
		return true
	}
	leaf, isLeaf := node.(string)
	return isLeaf && slices.Contains(values, leaf)
}

// resolvedPath walks the words of path down a resolved config map. The bound
// is the path's length, which the declaration judge requires to be non-empty.
func resolvedPath(resolved map[string]any, path []string) (any, bool) {
	var node any = resolved
	for _, word := range path {
		m, isMap := node.(map[string]any)
		if !isMap {
			return nil, false
		}
		next, found := m[word]
		if !found {
			return nil, false
		}
		node = next
	}
	return node, node != nil
}

// requiredInsertionRefusal names the peer, the list and the reference, and
// gives the plugin's reason, because a required member is placed by the
// plugin that owns the feature and an operator edit cannot reach it.
func requiredInsertionRefusal(ps *reactor.PeerSettings, direction string, ins *rpc.FilterInsertion) error {
	return fmt.Errorf("peer %s: the %s filter list names %s, which its plugin inserts itself where the config "+
		"needs it, so it cannot be named, moved or deactivated: %s\n"+
		"  remove %s from the %s filter list; it runs wherever the config that needs it is set",
		ps.Name, direction, ins.Filter, ins.Reason, ins.Filter, direction)
}
