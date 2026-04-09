// Design: docs/architecture/config/syntax.md — filter chain config parsing
// Overview: peers.go — peer configuration extraction

package bgpconfig

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config"
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
