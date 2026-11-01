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
	"github.com/ze-software/ze/internal/component/policy"
)

// attributeBasesPath is the container whose leaves carry the absent-attribute
// bases of policy arithmetic; bgp-filter-modify reads the same container.
const attributeBasesPath = "bgp/defaults/attribute"

// compileLibraryChains compiles, for every peer and dynamic template, the
// import and export chains of the members that evaluate in process over the
// BGP route view (policychain.HasEvaluator) and records each on its policy.
func compileLibraryChains(settings []*reactor.PeerSettings, reg *policy.Registry, bases policychain.Bases) error {
	for _, ps := range settings {
		imported, err := compileLibraryChain(ps.ImportPolicy, reg, bases, "bgp import", "peer "+ps.Name+" import filters")
		if err != nil {
			return err
		}
		exported, err := compileLibraryChain(ps.ExportPolicy, reg, bases, "bgp export", "peer "+ps.Name+" export filters")
		if err != nil {
			return err
		}
		ps.ImportPolicy, ps.ExportPolicy = imported, exported
	}
	return nil
}

// compileLibraryChain answers p with its library chain, p itself when the
// chain names no library member. A plugin filter in the same chain becomes an
// external member of the library chain, in chain order: it is sent the
// fields it declared through the chain's view, so it reads what an earlier
// member set, and its sets fold beside theirs (policychain external.go). The
// reference splits at its colon into plugin and filter, as the text chain
// splits it (PolicyFilterChain).
func compileLibraryChain(p filterapi.Policy, reg *policy.Registry, bases policychain.Bases, attachment, context string) (filterapi.Policy, error) {
	var members []policychain.Member
	library := false
	for i, ref := range p.Filters() {
		if ref.Inactive {
			continue
		}
		if p.InProcess(i) {
			continue // a compiled-in stage filter runs in the ingress pass
		}
		entry, ok := reg.Lookup(filterInstanceName(ref.Name))
		if ok && policychain.HasEvaluator(entry.Type) {
			members = append(members, policychain.Member{Name: entry.Name})
			library = true
			continue
		}
		pluginName, filterName, _ := strings.Cut(ref.Name, ":")
		members = append(members, policychain.Member{Name: filterName, Plugin: pluginName})
	}
	if !library {
		return p, nil
	}
	chain, err := policychain.Compile(attachment, reg, members, p.Final(), bases)
	if err != nil {
		return p, fmt.Errorf("%s: %w", context, err)
	}
	return p.WithLibrary(chain), nil
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
