// Design: docs/architecture/core-design.md -- AS-path length evaluation
// Related: config.go -- AS-path length config parsing
// Related: filter_aspath_length.go -- SDK entry point and handleFilterUpdate

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

// countASPathHops counts the number of ASNs in the space-separated AS-path
// string from the filter text format. Brackets are stripped (multi-ASN paths
// use "[65001 65002]" format). Empty paths return 0.
func countASPathHops(asPathStr string) int {
	if asPathStr == "" {
		return 0
	}
	asPathStr = strings.TrimSpace(asPathStr)
	if len(asPathStr) >= 2 && asPathStr[0] == '[' && asPathStr[len(asPathStr)-1] == ']' {
		asPathStr = asPathStr[1 : len(asPathStr)-1]
	}
	asPathStr = strings.TrimSpace(asPathStr)
	if asPathStr == "" {
		return 0
	}
	return 1 + strings.Count(asPathStr, " ")
}

func extractASPathField(updateText string) string {
	_, rest, ok := strings.Cut(updateText, "as-path ")
	if !ok {
		return ""
	}
	return extractASPathValue(rest)
}

func extractASPathValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s[0] == '[' {
		for i := 1; i < len(s); i++ {
			if s[i] == ']' {
				return s[:i+1]
			}
		}
		return s
	}
	before, _, found := strings.Cut(s, " ")
	if !found {
		return s
	}
	return before
}
