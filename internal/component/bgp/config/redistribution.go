// Design: docs/architecture/config/syntax.md — filter chain config parsing
// Overview: peers.go — peer configuration extraction

package bgpconfig

import (
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// extractFilterChain extracts import and export filter chains from a config
// tree. Each ref carries the clean filter name plus its per-member deactivation
// state (read out-of-band via GetMultiValuesState); no "inactive:" prefix is
// ever glued into a name. Validation against the filter registry is the
// caller's responsibility.
func extractFilterChain(tree *config.Tree) (importFilters, exportFilters []filterapi.FilterRef) {
	fc := tree.GetContainer("filter")
	if fc == nil {
		return nil, nil
	}

	return memberStatesToFilterRefs(fc.GetMultiValuesState("import")),
		memberStatesToFilterRefs(fc.GetMultiValuesState("export"))
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
		out[i] = filterapi.FilterRef{Name: canonicalizeOne(ref.Name, reg, typesMap), Inactive: ref.Inactive}
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

// concatFilters concatenates multiple filter chains into a single ordered chain.
// Nil slices are skipped. Returns nil if all inputs are empty.
func concatFilters(chains ...[]filterapi.FilterRef) []filterapi.FilterRef {
	n := 0
	for _, c := range chains {
		n += len(c)
	}
	if n == 0 {
		return nil
	}

	result := make([]filterapi.FilterRef, 0, n)
	for _, c := range chains {
		result = append(result, c...)
	}

	return result
}
