// Design: docs/architecture/core-design.md -- AS-path length evaluation
// Related: config.go -- AS-path length config parsing
// Related: filter_aspath_length.go -- SDK entry point and handleFilterUpdate
// Related: internal/component/bgp/filtertext -- the reader of the filter text format

package filter_aspath_length

import "strings"

func evaluateASPathLength(pathLen int, def *asPathLengthDef) bool {
	if def.max >= 0 && pathLen > def.max {
		return false
	}
	if def.min >= 0 && pathLen < def.min {
		return false
	}
	return true
}

// countASPathHops counts the ASNs of the AS path as filtertext.ASPath returns
// it: decimal ASNs separated by one space, with no brackets left on a multi-ASN
// path. An update that carries no AS path reads back empty and counts no hops.
func countASPathHops(asPath string) int {
	if asPath == "" {
		return 0
	}
	return 1 + strings.Count(asPath, " ")
}
