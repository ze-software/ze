// Design: docs/architecture/core-design.md -- which changed file drifts which index
// Overview: discoveryindex.go -- the generator these rules describe
//
// sources.go answers whether a committed path can make a generated discovery
// index outdated. The changed-file router uses this answer to select the
// freshness gate. The commit gate uses it to require an updated committed
// index.

package discoveryindex

import (
	"slices"
	"strings"
)

// generator is the source file that produces OutputRel. A commit touching it
// can change every byte of the index, so it feeds the index it writes.
const generator = "internal/le/discoveryindex/discoveryindex.go"

// packageMarker is the header a Go file carries when the index derives a line
// from it.
const packageMarker = "// Package"

// outputs names every generated index these rules cover.
//
// ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md are absent by design because they
// are no longer tracked. This file answers whether a commit must refresh a
// COMMITTED index. Git cannot supply an untracked file in a materialized commit
// view. The working tree still generates both files on demand.
var outputs = [...]string{OutputRel}

// feeds answers the indexes that committing path can drift, sorted, or nothing
// when path feeds none.
//
// headerText is used only for a non-test `.go` file. It searches for the
// `// Package` marker from which the index derives text. The caller supplies
// content from the working tree, HEAD, or both. A change can add or remove the
// marker, and only the caller knows which trees are available.
func feeds(path, headerText string) []string {
	// A committed index feeds only itself: committing it is how its own
	// freshness is satisfied, and it never obliges any OTHER index to ride
	// along.
	if slices.Contains(outputs[:], path) {
		return []string{path}
	}

	// A generator feeds exactly the output it writes.
	if path == generator {
		return []string{OutputRel}
	}

	// A register.go Description feeds the map, whether or not the file carries
	// a package header.
	if strings.HasSuffix(path, "register.go") {
		return []string{OutputRel}
	}

	if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") &&
		strings.Contains(headerText, packageMarker) {
		return []string{OutputRel}
	}
	return nil
}

// IsSource reports whether committing path can change a generated discovery
// index. It is defined in terms of feeds so the "is it a source" and "which
// index" answers can never disagree.
func IsSource(path, headerText string) bool {
	return len(feeds(path, headerText)) > 0
}
