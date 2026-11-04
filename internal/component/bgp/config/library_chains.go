// Design: docs/architecture/core-design.md -- the in-process BGP chain
// Related: peers.go -- peersAndDynamicGroups, validateChainNames
// Related: ../policychain/chain.go -- Compile

package bgpconfig

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// attributeBasesPath is the container whose leaves carry the absent-attribute
// bases of policy arithmetic; bgp-filter-modify reads the same container.
const attributeBasesPath = "bgp/defaults/attribute"

// compileLibraryChains compiles, for every peer and dynamic template, the
// import and export chains of the members that evaluate in process over the
// BGP route view (policychain.HasEvaluator) and records each on its policy.
//
// conditional holds, per dynamic template, the members plugins inserted
// under a session condition (insertDeclaredFilters): such a template compiles
// per peer once its OPEN arrives (compileConditionalChain).
func compileLibraryChains(settings []*reactor.PeerSettings, conditional map[*reactor.PeerSettings]sessionMembers,
	reg *policy.Registry, bases policychain.Bases,
) error {
	for _, ps := range settings {
		members := conditional[ps]
		imported, err := compileConditionalChain(ps.ImportPolicy, members.imports, reg, bases, "bgp import", "peer "+ps.Name+" import filters")
		if err != nil {
			return err
		}
		exported, err := compileConditionalChain(ps.ExportPolicy, members.exports, reg, bases, attachmentExport, "peer "+ps.Name+" export filters")
		if err != nil {
			return err
		}
		ps.ImportPolicy, ps.ExportPolicy = imported, exported
	}
	return nil
}

// compileConditionalChain answers p compiled now (compileLibraryChain), or,
// when a plugin inserted a member of p under a session condition, p marked
// templated: whether that member stays is known per peer only, so each peer
// compiles its own (templatedChain.CompileResolved).
func compileConditionalChain(p filterapi.Policy, conditional []sessionMember, reg *policy.Registry,
	bases policychain.Bases, attachment, context string,
) (filterapi.Policy, error) {
	if len(conditional) == 0 {
		return compileLibraryChain(p, reg, bases, attachment, context)
	}
	return p.WithTemplated(&templatedChain{reg: reg, bases: bases, attachment: attachment, context: context,
		members: templatedMembers(p), conditional: conditional}), nil
}

// compileLibraryChain answers p with its library chain, p itself when the
// chain holds no member the chain runs (none, or only inactive refs and
// compiled-in stage filters). Every BGP import and export chain runs as the
// library chain (spec filter-wire-0 Data Flow step 2), the only chain the
// reactor runs: a policy naming a plugin filter with no library chain fails
// closed there. A plugin filter becomes an external member of the
// library chain, in chain order: it is sent the fields it declared through
// the chain's view, so it reads what an earlier member set, and its sets fold
// beside theirs (policychain external.go); one that declared no reads
// fails every route it is asked to judge. The reference splits at its first
// colon into plugin and filter.
//
// A dynamic group's chain naming a member through a config variable compiles
// no library here: the instance is known only once a peer resolves the
// variable, so the policy is marked templated and each peer compiles its own
// (templatedChain.CompileResolved, from reactor resolveDynamicPolicy).
func compileLibraryChain(p filterapi.Policy, reg *policy.Registry, bases policychain.Bases, attachment, context string) (filterapi.Policy, error) {
	if templated := templatedMembers(p); templated != nil {
		return p.WithTemplated(&templatedChain{reg: reg, bases: bases, attachment: attachment, context: context, members: templated}), nil
	}
	var members []policychain.Member
	typesMap := registry.FilterTypesMap()
	for i, ref := range p.Filters() {
		if ref.Inactive {
			continue
		}
		if p.InProcess(i) {
			continue // a compiled-in stage filter runs in the ingress pass
		}
		entry, ok := policyChainInstance(reg, ref, typesMap)
		if ok && policychain.HasEvaluator(entry.Type) {
			members = append(members, policychain.Member{Name: entry.Name})
			continue
		}
		pluginName, filterName, _ := strings.Cut(ref.Name, ":")
		members = append(members, policychain.Member{Name: filterName, Plugin: pluginName})
	}
	if len(members) == 0 {
		return p, nil
	}
	chain, err := compileMembers(p, attachment, reg, members, bases)
	if err != nil {
		return p, fmt.Errorf("%s: %w", context, err)
	}
	return p.WithLibrary(chain), nil
}

// compileMembers compiles members under p's final, or, for a policy the
// operator never configured whose members plugins inserted
// (insertDeclaredFilters), under the session's end disposition, so an
// inserted member never makes the peer's list "configured" (RFC 8212
// Section 3).
func compileMembers(p filterapi.Policy, attachment string, reg *policy.Registry, members []policychain.Member, bases policychain.Bases) (*policychain.Chain, error) {
	if !p.Configured() {
		return policychain.CompileUnconfigured(attachment, reg, members, bases)
	}
	return policychain.Compile(attachment, reg, members, p.Final(), bases)
}

// templatedMembers indexes the refs of p that name their filter through a
// config variable, nil when none does. An inactive ref is indexed too: its
// name is resolved with the rest, and the check after resolution skips nothing
// the load would have checked (validateChainNames checks inactive refs).
func templatedMembers(p filterapi.Policy) []int {
	var templated []int
	for i, ref := range p.Filters() {
		if isTemplatedMember(ref.Name) {
			templated = append(templated, i)
		}
	}
	return templated
}

// parseAttributeBases reads bgp { defaults { attribute { } } }, each leaf's
// YANG default standing for an absent leaf.
func parseAttributeBases(tree *config.Tree, schema *config.Schema) (policychain.Bases, error) {
	var container *config.Tree
	if bgp := tree.GetContainer("bgp"); bgp != nil {
		if defaults := bgp.GetContainer("defaults"); defaults != nil {
			container = defaults.GetContainer("attribute")
		}
	}
	lp, err := attributeBase(container, schema, "local-preference")
	if err != nil {
		return policychain.Bases{}, err
	}
	med, err := attributeBase(container, schema, "med")
	if err != nil {
		return policychain.Bases{}, err
	}
	return policychain.Bases{LocalPreference: lp, MED: med}, nil
}

func attributeBase(container *config.Tree, schema *config.Schema, leaf string) (uint32, error) {
	var raw string
	present := false
	if container != nil {
		raw, present = container.Get(leaf)
	}
	if !present {
		node, err := schema.Lookup(attributeBasesPath + "/" + leaf)
		if err != nil {
			return 0, fmt.Errorf("attribute defaults: %w", err)
		}
		declared, ok := node.(*config.LeafNode)
		if !ok {
			return 0, fmt.Errorf("BUG: attribute defaults: %s/%s is not a leaf", attributeBasesPath, leaf)
		}
		raw = declared.Default
	}
	value, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("bgp defaults attribute %s: %q is not a number from 0 to 4294967295", leaf, raw)
	}
	return uint32(value), nil
}
