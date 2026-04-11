// Design: docs/architecture/config/syntax.md — filter chain config parsing
// Overview: peers.go — peer configuration extraction

package bgpconfig

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// extractFilterChain extracts import and export filter lists from a config tree.
// Returns raw name slices from the "filter" container. Validation against the
// filter registry is the caller's responsibility.
func extractFilterChain(tree *config.Tree) (importFilters, exportFilters []string) {
	fc := tree.GetContainer("filter")
	if fc == nil {
		return nil, nil
	}

	return fc.GetMultiValues("import"), fc.GetMultiValues("export")
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
// The `inactive:` prefix is preserved around the rewrite: an inactive form
// like `inactive:prefix-list:CUSTOMERS` or `inactive:CUSTOMERS` stays
// `inactive:<plugin>:<filter>` after canonicalization.
//
// Refs that cannot be resolved (plain name not in registry; unknown prefix)
// are left untouched so existing validation paths can still report a clean
// error with the user-facing token instead of a synthetic one.
func canonicalizeFilterRefs(chain []string, reg *FilterRegistry) []string {
	if len(chain) == 0 {
		return chain
	}
	out := make([]string, len(chain))
	typesMap := registry.FilterTypesMap()
	for i, ref := range chain {
		out[i] = canonicalizeOne(ref, reg, typesMap)
	}
	return out
}

// canonicalizeOne resolves a single chain ref. See canonicalizeFilterRefs.
func canonicalizeOne(ref string, reg *FilterRegistry, typesMap map[string]string) string {
	inactive := false
	clean := ref
	if strings.HasPrefix(clean, "inactive:") {
		inactive = true
		clean = strings.TrimPrefix(clean, "inactive:")
	}

	wrap := func(s string) string {
		if inactive {
			return "inactive:" + s
		}
		return s
	}

	// Typed form: prefix:name
	if before, after, found := strings.Cut(clean, ":"); found {
		// If the prefix is a known filter type, rewrite to the plugin form.
		if plugin, ok := typesMap[before]; ok {
			return wrap(plugin + ":" + after)
		}
		// Otherwise assume it is already a plugin process name (e.g.,
		// `bgp-filter-prefix:CUSTOMERS`) and pass through.
		return wrap(clean)
	}

	// Plain name: look up in the filter registry to find its YANG list type,
	// then resolve the type to the owning plugin.
	if reg != nil {
		if entry, ok := reg.Lookup(clean); ok {
			if plugin, ok := typesMap[entry.Type]; ok {
				return wrap(plugin + ":" + clean)
			}
		}
	}
	return wrap(clean)
}

// concatFilters concatenates multiple filter slices into a single ordered chain.
// Nil slices are skipped. Returns nil if all inputs are empty.
func concatFilters(chains ...[]string) []string {
	n := 0
	for _, c := range chains {
		n += len(c)
	}
	if n == 0 {
		return nil
	}

	result := make([]string, 0, n)
	for _, c := range chains {
		result = append(result, c...)
	}

	return result
}
