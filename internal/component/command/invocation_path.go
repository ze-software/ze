// Design: docs/architecture/api/commands.md -- exact decoded command path prefixes.
// Related: invocation_scan.go -- the shared operational word grammar.

package command

import (
	"slices"
	"strings"
)

// invocationMatchesPathPrefix compares registered path segments with complete
// decoded invocation words. Matching is case-sensitive; a metadata reader that
// owns case folding MUST apply it before calling. An empty path never matches.
func invocationMatchesPathPrefix(words []string, path string) bool {
	segments := strings.Fields(path)
	if len(segments) == 0 {
		return false
	}
	if len(words) < len(segments) {
		return false
	}
	return slices.Equal(words[:len(segments)], segments)
}
