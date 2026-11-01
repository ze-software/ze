// Design: docs/architecture/core-design.md -- AS-path length evaluation
// Related: config.go -- AS-path length config parsing
// Related: condition.go -- the as-path-length condition of a match instance

package filter_aspath_length

import "github.com/ze-software/ze/internal/core/bgp/attribute"

func evaluateASPathLength(pathLen int, def *asPathLengthDef) bool {
	if def.max >= 0 && pathLen > def.max {
		return false
	}
	if def.min >= 0 && pathLen < def.min {
		return false
	}
	return true
}

// pathHops counts the ASNs of the reconstructed AS path the BGP view answers:
// every ASN of every segment, each segment's ASNs flattened into one list, so
// an AS_SET counts each of its ASNs. A nil path, an UPDATE with no AS path,
// counts no hops.
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
