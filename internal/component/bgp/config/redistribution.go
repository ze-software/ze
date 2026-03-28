// Design: docs/architecture/config/syntax.md — redistribution filter config parsing
// Overview: peers.go — peer configuration extraction

package bgpconfig

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// extractRedistributionFilters extracts import and export filter lists from a config tree.
// Returns validated slices of "<plugin>:<filter>" strings.
func extractRedistributionFilters(tree *config.Tree) (importFilters, exportFilters []string, err error) {
	redist := tree.GetContainer("redistribution")
	if redist == nil {
		return nil, nil, nil
	}

	importFilters, err = validateFilterRefs(redist.GetMultiValues("import"))
	if err != nil {
		return nil, nil, fmt.Errorf("redistribution import: %w", err)
	}

	exportFilters, err = validateFilterRefs(redist.GetMultiValues("export"))
	if err != nil {
		return nil, nil, fmt.Errorf("redistribution export: %w", err)
	}

	return importFilters, exportFilters, nil
}

// validateFilterRefs validates that each filter reference has the <plugin>:<filter> format.
func validateFilterRefs(refs []string) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}

	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		plugin, filter, ok := strings.Cut(ref, ":")
		if !ok {
			return nil, fmt.Errorf("invalid filter reference %q, expected <plugin>:<filter>", ref)
		}
		if plugin == "" {
			return nil, fmt.Errorf("empty plugin name in filter reference %q", ref)
		}
		if filter == "" {
			return nil, fmt.Errorf("empty filter name in filter reference %q", ref)
		}
		result = append(result, ref)
	}

	return result, nil
}

// DefaultImportFilters are default import filters prepended to every peer's chain.
// Can be overridden by user filters that declare matching overrides.
// Populated at init time by protocol filter packages.
var DefaultImportFilters []string

// DefaultExportFilters are default export filters prepended to every peer's chain.
var DefaultExportFilters []string

// applyOverrides removes default filters that are overridden by user filters.
// overrideMap maps "<plugin>:<filter>" to a list of default filter names it replaces.
// Returns the filtered default list with overridden entries removed.
func applyOverrides(defaults, userFilters []string, overrideMap map[string][]string) []string {
	if len(defaults) == 0 || len(overrideMap) == 0 {
		return defaults
	}

	// Collect all overridden default filter names.
	overridden := make(map[string]bool)
	for _, ref := range userFilters {
		if targets, ok := overrideMap[ref]; ok {
			for _, t := range targets {
				overridden[t] = true
			}
		}
	}

	if len(overridden) == 0 {
		return defaults
	}

	result := make([]string, 0, len(defaults))
	for _, d := range defaults {
		if !overridden[d] {
			result = append(result, d)
		}
	}

	return result
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
