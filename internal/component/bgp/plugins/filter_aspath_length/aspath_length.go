// Design: docs/architecture/core-design.md -- AS-path length evaluation
// Related: config.go -- AS-path length config parsing
// Related: filter_aspath_length.go -- SDK entry point and handleFilterUpdate
// Related: internal/component/bgp/filtertext -- the reader of the filter text format

package filter_aspath_length

import (
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

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

// pathHops counts the ASNs of the reconstructed AS path the BGP view answers:
// every ASN of every segment, as countASPathHops counts the text the filter
// reads, where each segment's ASNs are flattened into one list. A nil path,
// an UPDATE with no AS path, counts no hops.
func pathHops(path *attribute.ASPath) int {
	if path == nil {
		return 0
	}
	hops := 0
	for _, segment := range path.Segments {
		hops += len(segment.ASNs)
	}
	return hops
}
