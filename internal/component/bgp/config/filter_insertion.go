// Design: docs/architecture/core-design.md -- Policy Filter Chain, auto-inserted members
// Related: peers.go -- prependDefaultFilters, the operator-config prepend this generalises

package bgpconfig

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
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
// the inserted one compare in one form, and before the chains compile. It
// answers, per dynamic group template, the members it inserted under a
// session condition, for compileLibraryChains to judge per member peer.
func insertDeclaredFilters(bgpTree map[string]any, peers []*reactor.PeerSettings,
	groups []*reactor.DynamicGroupConfig, reg *policy.Registry,
) (map[*reactor.PeerSettings]sessionMembers, error) {
	insertions, err := declaredInsertions()
	if err != nil {
		return nil, err
	}
	if len(insertions) == 0 {
		return nil, nil
	}
	peerMap, _ := bgpTree["peer"].(map[string]any)
	for _, ps := range peers {
		resolved, _ := peerMap[ps.Name].(map[string]any)
		if _, err := insertForPeer(ps, resolved, insertions, reg); err != nil {
			return nil, err
		}
	}
	// A dynamic group's members are built from its template, so the template
	// is the resolved config its condition is judged against, as peerRoles
	// judges a role.
	conditional := make(map[*reactor.PeerSettings]sessionMembers, len(groups))
	for _, dg := range groups {
		members, err := insertForPeer(dg.Settings, dg.Template, insertions, reg)
		if err != nil {
			return nil, err
		}
		conditional[dg.Settings] = members
	}
	return conditional, nil
}

// sessionMembers are the members a dynamic template carries under a session
// condition, per list.
type sessionMembers struct {
	imports []sessionMember
	exports []sessionMember
}

// declaredInsertions collects every compiled-in registration's insertions,
// each judged by the same rules as at Stage 1, and those a third-party
// plugin declared at Stage 1 (registry.RuntimeFilterInsertions, already
// judged there), sorted by priority then reference so the order never
// depends on registration order. The daemon loads its config before any
// plugin reaches Stage 1, so a third party's insertions are absent from that
// first load and land through the tree reload the BGP plugin runs before its
// peers start (spec-filter-wire-6-stage-filters A-12).
//
// A compiled-in plugin's insertions apply only where that plugin runs in this
// daemon, configured or auto-loaded (registry.LoadedPlugins): the plugin that
// owns the procedure is its scope, and the condition then picks the peers, so
// a session-condition member needs no config of its plugin on the peer. With
// no plugin server (an offline `ze config validate`, a unit test) nothing
// says which plugins load, and every compiled-in plugin's insertions are
// placed: the widest set a daemon could run, so a required-member refusal it
// could meet is reported there too.
func declaredInsertions() ([]rpc.FilterInsertion, error) {
	var insertions []rpc.FilterInsertion
	loaded, known := registry.LoadedPlugins()
	for _, r := range registry.All() {
		if len(r.FilterInsertions) == 0 {
			continue
		}
		if known && !slices.Contains(loaded, r.Name) {
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
	for _, runtime := range registry.RuntimeFilterInsertions() {
		insertions = append(insertions, runtime.Insertions...)
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
//
// A session condition is judged on the peer's facts (PeerSettings.SessionFacts).
// A dynamic group's template states no remote AS, so there the member is
// inserted in its place and answered as a session member, for each peer of
// the group to keep or drop once its OPEN gives its AS
// (templatedChain.CompileResolved).
func insertForPeer(ps *reactor.PeerSettings, resolved map[string]any,
	insertions []rpc.FilterInsertion, reg *policy.Registry,
) (sessionMembers, error) {
	var members sessionMembers
	for i := len(insertions) - 1; i >= 0; i-- {
		ins := &insertions[i]
		holds, deferred, err := insertionHolds(ps, resolved, ins)
		if err != nil {
			return members, err
		}
		if !holds {
			continue
		}
		refs, bound := canonicalizeFilterRefs([]policy.Ref{{Name: ins.Filter}}, reg)
		ref := refs[0]
		inProcess := len(bound) > 0 && bound[0]
		member := sessionMember{filter: ref.Name, condition: ins.Session}
		if ins.Direction == rpc.FilterImport {
			if filterChainContains(ps.ImportPolicy.Filters(), ref.Name) {
				if ins.Required {
					return members, requiredInsertionRefusal(ps, "import", ins)
				}
				continue
			}
			ps.ImportPolicy = ps.ImportPolicy.Prepend(ref, inProcess)
			if deferred {
				members.imports = append(members.imports, member)
			}
			continue
		}
		if filterChainContains(ps.ExportPolicy.Filters(), ref.Name) {
			if ins.Required {
				return members, requiredInsertionRefusal(ps, "export", ins)
			}
			continue
		}
		ps.ExportPolicy = ps.ExportPolicy.Prepend(ref, inProcess)
		if deferred {
			members.exports = append(members.exports, member)
		}
	}
	return members, nil
}

// insertionHolds reports whether the peer meets the insertion's condition,
// and, for a session condition on a dynamic template, that the answer is
// deferred to each member peer (holds and deferred both true).
func insertionHolds(ps *reactor.PeerSettings, resolved map[string]any, ins *rpc.FilterInsertion) (holds, deferred bool, err error) {
	if len(ins.Session) == 0 {
		return insertionConditionHolds(resolved, ins.Config, ins.Values), false, nil
	}
	if ps.IsDynamic {
		return true, true, nil
	}
	holds, err = policyview.SessionConditionHolds(ins.Session, ps.SessionFacts(ps.PeerAS))
	if err != nil {
		return false, false, fmt.Errorf("peer %s: filter insertion %s: %w", ps.Name, ins.Filter, err)
	}
	return holds, false, nil
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
