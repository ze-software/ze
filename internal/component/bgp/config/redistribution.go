// Design: docs/architecture/config/syntax.md — filter chain config parsing
// Overview: peers.go — peer configuration extraction

package bgpconfig

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// extractFilterPolicies reads both directions from one original configuration layer.
func extractFilterPolicies(tree *config.Tree, path string) (policy.Chain, policy.Chain, error) {
	importChain, err := policy.ExtractChain(tree, "import", path)
	if err != nil {
		return policy.Chain{}, policy.Chain{}, err
	}
	exportChain, err := policy.ExtractChain(tree, "export", path)
	return importChain, exportChain, err
}

// validateFilterPolicies visits the bounded BGP/group/peer hierarchy before
// resolving defaults, including layers that have no peer or dynamic template.
func validateFilterPolicies(tree *config.Tree) error {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}
	if err := validatePolicyLayer(bgp, "bgp"); err != nil {
		return err
	}
	for _, group := range bgp.GetListOrdered("group") {
		path := "bgp/group/" + group.Key
		if err := validatePolicyLayer(group.Value, path); err != nil {
			return err
		}
		for _, peer := range group.Value.GetListOrdered("peer") {
			if err := validatePolicyLayer(peer.Value, path+"/peer/"+peer.Key); err != nil {
				return err
			}
		}
	}
	for _, peer := range bgp.GetListOrdered("peer") {
		if err := validatePolicyLayer(peer.Value, "bgp/peer/"+peer.Key); err != nil {
			return err
		}
	}
	return nil
}

func validatePolicyLayer(tree *config.Tree, path string) error {
	for _, direction := range []string{"import", "export"} {
		if _, err := policy.ParseChainFinal(tree, direction, path); err != nil {
			return err
		}
	}
	return nil
}

// canonicalizeFilterRefs rewrites each chain ref to the form runtime dispatch
// consumes and binds a configured in-process instance to its registered
// stage, answering the refs and their parallel bindings (nil when none is
// bound) for filterapi.Policy.WithBoundFilters.
//
// A configured in-process instance receives its registered stage ownership
// and the stage-qualified name BindInProcess gives it; a matching prefix alone
// never grants it. Every other ref is rewritten by policy.CanonicalName, which
// accepts the plugin, filter-type and plain forms, and keeps RPC dispatch.
// Deactivation state (policy.Ref.Inactive) is carried structurally and
// preserved across the rewrite; the name itself is always clean.
func canonicalizeFilterRefs(chain []policy.Ref, reg *policy.Registry) ([]policy.Ref, []bool) {
	if len(chain) == 0 {
		return chain, nil
	}
	out := make([]policy.Ref, len(chain))
	var bound []bool
	typesMap := registry.FilterTypesMap()
	for i, ref := range chain {
		out[i] = ref
		if reg != nil {
			if entry, ok := reg.Lookup(filterInstanceName(ref.Name)); ok {
				if named, inProcess := filterapi.BindInProcess(ref, entry.Type); inProcess {
					out[i] = named
					if bound == nil {
						bound = make([]bool, len(chain))
					}
					bound[i] = true
					continue
				}
			}
		}
		out[i].Name = policy.CanonicalName(ref.Name, reg, typesMap)
	}
	return out, bound
}
