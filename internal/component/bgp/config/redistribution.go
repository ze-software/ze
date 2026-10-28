// Design: docs/architecture/config/syntax.md — filter chain config parsing
// Overview: peers.go — peer configuration extraction

package bgpconfig

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// extractFilterPolicy retains member deactivation separately from the clean name.
func extractFilterPolicy(tree *config.Tree, direction, path string) (filterapi.Policy, error) {
	final, err := parsePolicyDisposition(tree, direction, path)
	if err != nil {
		return filterapi.Policy{}, err
	}
	if final == 0 {
		return filterapi.Policy{}, nil
	}
	policy := tree.GetContainer("filter").GetContainer(direction)
	return filterapi.NewPolicy(memberStatesToFilterRefs(policy.GetMultiValuesState("filters")), final)
}

// parsePolicyDisposition validates an original layer before inheritance can hide
// an omitted final. Validation alone never allocates or copies filter references.
func parsePolicyDisposition(tree *config.Tree, direction, path string) (filterapi.FinalDisposition, error) {
	fc := tree.GetContainer("filter")
	if fc == nil {
		return 0, nil
	}
	policy := fc.GetContainer(direction)
	if policy == nil {
		if len(fc.GetMultiValuesState(direction)) != 0 {
			return 0, fmt.Errorf("%s/filter/%s: list-only policy is no longer supported; write %s { filters [ NAME ]; final accept; }", path, direction, direction)
		}
		if _, exists := fc.Get(direction); exists {
			return 0, fmt.Errorf("%s/filter/%s: list-only policy is no longer supported; write %s { filters [ NAME ]; final accept; }", path, direction, direction)
		}
		return 0, nil
	}
	final, _ := policy.Get("final")
	switch final {
	case "accept":
		return filterapi.FinalAccept, nil
	case "reject":
		return filterapi.FinalReject, nil
	default:
		return 0, fmt.Errorf("%s/filter/%s/final: missing or invalid final %q; every configured policy requires a final decision; write %s { filters [ NAME ]; final accept; } or final reject", path, direction, final, direction)
	}
}

// extractFilterPolicies reads both directions from one original configuration layer.
func extractFilterPolicies(tree *config.Tree, path string) (filterapi.Policy, filterapi.Policy, error) {
	importPolicy, err := extractFilterPolicy(tree, "import", path)
	if err != nil {
		return filterapi.Policy{}, filterapi.Policy{}, err
	}
	exportPolicy, err := extractFilterPolicy(tree, "export", path)
	return importPolicy, exportPolicy, err
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
		if _, err := parsePolicyDisposition(tree, direction, path); err != nil {
			return err
		}
	}
	return nil
}

// memberStatesToFilterRefs maps the config Tree's structural member view onto
// the reactor's FilterRef chain type.
func memberStatesToFilterRefs(states []config.MemberState) []filterapi.FilterRef {
	if len(states) == 0 {
		return nil
	}
	refs := make([]filterapi.FilterRef, len(states))
	for i, s := range states {
		refs[i] = filterapi.FilterRef{Name: s.Value, Inactive: s.Inactive}
	}
	return refs
}

// canonicalizeFilterRefs rewrites each chain ref to its canonical
// `<plugin-process-name>:<filter-name>` form consumed by runtime dispatch,
// accepting three user-facing forms:
//
//  1. `<plugin-process-name>:<filter-name>`  (current explicit form, kept
//     unchanged when the first token is already a registered plugin name)
//  2. `<filter-type>:<filter-name>`           (short form using the YANG list
//     type name, e.g. `prefix-list:CUSTOMERS`; resolved to the plugin that
//     registered the filter type via FilterTypes in registry.Registration)
//  3. `<filter-name>`                          (plain form, no prefix; looked
//     up in the filter registry to find its type, then resolved via the type
//     map to the plugin)
//
// Deactivation state (FilterRef.Inactive) is carried structurally and preserved
// across the rewrite; the name itself is always clean (no `inactive:` prefix).
// A configured in-process instance also receives its registered stage ownership;
// a matching prefix alone never grants it. Other refs retain RPC dispatch.
//
// Refs that cannot be resolved (plain name not in registry; unknown prefix)
// are left untouched so existing validation paths can still report a clean
// error with the user-facing token instead of a synthetic one.
func canonicalizeFilterRefs(chain []filterapi.FilterRef, reg *FilterRegistry) []filterapi.FilterRef {
	if len(chain) == 0 {
		return chain
	}
	out := make([]filterapi.FilterRef, len(chain))
	typesMap := registry.FilterTypesMap()
	for i, ref := range chain {
		out[i] = ref
		if reg != nil {
			if entry, ok := reg.Lookup(filterInstanceName(ref.Name)); ok {
				out[i] = filterapi.BindInProcess(ref, entry.Type)
			}
		}
		if !out[i].InProcess() {
			out[i].Name = canonicalizeOne(ref.Name, reg, typesMap)
		}
	}
	return out
}

// canonicalizeOne resolves a single chain ref name. See canonicalizeFilterRefs.
func canonicalizeOne(name string, reg *FilterRegistry, typesMap map[string]string) string {
	// Typed form: prefix:name
	if before, after, found := strings.Cut(name, ":"); found {
		// If the prefix is a known filter type, rewrite to the plugin form.
		if plugin, ok := typesMap[before]; ok {
			var tb textbuf.Buffer
			return tb.Str(plugin).Byte(':').Str(after).String()
		}
		// Otherwise assume it is already a plugin process name (e.g.,
		// `bgp-filter-prefix:CUSTOMERS`) and pass through.
		return name
	}

	// Plain name: look up in the filter registry to find its YANG list type,
	// then resolve the type to the owning plugin.
	if reg != nil {
		if entry, ok := reg.Lookup(name); ok {
			if plugin, ok := typesMap[entry.Type]; ok {
				var tb textbuf.Buffer
				return tb.Str(plugin).Byte(':').Str(name).String()
			}
		}
	}
	return name
}

// concatPolicies accumulates references while the most-specific configured
// policy supplies the final. An absent child never erases its inherited final.
func concatPolicies(policies ...filterapi.Policy) filterapi.Policy {
	n := 0
	var effective filterapi.Policy
	for _, policy := range policies {
		n += len(policy.Filters())
		if policy.Configured() {
			effective = policy
		}
	}
	if n == 0 {
		return effective.WithFilters(nil)
	}
	refs := make([]filterapi.FilterRef, 0, n)
	for _, policy := range policies {
		refs = append(refs, policy.Filters()...)
	}
	return effective.WithFilters(refs)
}
